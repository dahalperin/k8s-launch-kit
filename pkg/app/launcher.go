// Copyright 2025 NVIDIA CORPORATION & AFFILIATES
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/nvidia/k8s-launch-kit/pkg/config"
	"github.com/nvidia/k8s-launch-kit/pkg/kubeclient"
	"github.com/nvidia/k8s-launch-kit/pkg/llm"
	applog "github.com/nvidia/k8s-launch-kit/pkg/log"
	"github.com/nvidia/k8s-launch-kit/pkg/networkoperatorplugin"
	"github.com/nvidia/k8s-launch-kit/pkg/options"
	"github.com/nvidia/k8s-launch-kit/pkg/plugin"
	"github.com/nvidia/k8s-launch-kit/pkg/profiles"
	"gopkg.in/yaml.v2"
)

// Launcher represents the main application launcher
type Launcher struct {
	options    options.Options
	logger     logr.Logger
	plugins    map[string]plugin.Plugin
	kubeClient client.Client
}

// New creates a new Launcher instance with the given options
func New(options options.Options) *Launcher {
	l := &Launcher{
		options: options,
		logger:  log.Log,
		plugins: make(map[string]plugin.Plugin),
	}

	return l
}

// Run executes the main application logic with the 3-phase workflow
func (l *Launcher) Run() error {
	if l.options.LogLevel != "" {
		if err := applog.SetLogLevel(l.options.LogLevel); err != nil {
			return fmt.Errorf("failed to set log level: %w", err)
		}
	}

	for _, plugin := range l.options.EnabledPlugins {
		switch plugin {
		case networkoperatorplugin.PluginName:
			l.plugins[plugin] = &networkoperatorplugin.NetworkOperatorPlugin{}
		default:
			err := fmt.Errorf("unknown plugin: %s", plugin)
			l.logger.Error(err, "Skipping plugin")
			return err
		}
	}

	if l.options.Kubeconfig != "" {
		k8sClient, err := kubeclient.New(l.options.Kubeconfig)
		if err != nil {
			return fmt.Errorf("failed to create k8s client: %w", err)
		}
		l.kubeClient = k8sClient
	}

	if err := l.executeWorkflow(); err != nil {
		return err
	}

	return nil
}

// executeWorkflow executes the main 3-phase workflow
func (l *Launcher) executeWorkflow() error {
	l.logger.Info("Starting l8k workflow")

	configPath := ""
	if l.options.DiscoverClusterConfig {
		if err := l.discoverClusterConfig(); err != nil {
			return fmt.Errorf("cluster discovery failed: %w", err)
		}

		configPath = l.options.SaveClusterConfig
	} else {
		configPath = l.options.UserConfig
	}

	profilesConfiguredInCmd := true
	for _, plugin := range l.plugins {
		if !plugin.ProfileConfiguredInCmd(l.options) {
			profilesConfiguredInCmd = false
			break
		}
	}

	if !profilesConfiguredInCmd && l.options.Prompt == "" && !l.options.LLMInteractive {
		l.logger.Info("Profiles are not configured for every plugin, skipping deployment files generation")
		return nil
	}

	fullConfig, err := config.LoadFullConfig(configPath, l.logger)
	if err != nil {
		return fmt.Errorf("failed to load full config: %w", err)
	}

	if fullConfig.Profile == nil {
		fullConfig.Profile = &config.Profile{}

		if profilesConfiguredInCmd {
			for _, plugin := range l.plugins {
				if err := plugin.BuildProfileFromOptions(l.options, fullConfig.Profile); err != nil {
					return fmt.Errorf("failed to build profile for plugin %s: %w", plugin.GetName(), err)
				}
			}
		} else if l.options.LLMInteractive {
			l.logger.Info("Starting interactive LLM session")

			prompt, err := l.runInteractiveSession(fullConfig.ClusterConfig)
			if err != nil {
				return fmt.Errorf("interactive session failed: %w", err)
			}

			for _, plugin := range l.plugins {
				if err := plugin.BuildProfileFromLLMResponse(prompt, fullConfig.Profile); err != nil {
					return fmt.Errorf("failed to build profile for plugin %s: %w", plugin.GetName(), err)
				}
			}

			l.logger.Info("Selected options",
				"fabric", fullConfig.Profile.Fabric,
				"deployment", fullConfig.Profile.Deployment,
				"multirail", fullConfig.Profile.Multirail,
				"spectrumX", fullConfig.Profile.SpectrumX,
				"ai", fullConfig.Profile.Ai,
				"reasoning", prompt["reasoning"])
		} else if l.options.Prompt != "" {
			l.logger.Info("Selecting a profile using LLM-assisted prompt")

			prompt, err := llm.SelectPromptWithModel(l.options.Prompt, *fullConfig.ClusterConfig, l.options.LLMApiKey, l.options.LLMApiUrl, l.options.LLMVendor, l.options.LLMModel)
			if err != nil {
				return fmt.Errorf("failed to select prompt: %w", err)
			}
			confidence := prompt["confidence"]
			if confidence == "low" {
				return fmt.Errorf("couldn't select a deployment profile based on the user prompt. Try again with a different prompt or use the cli flags (--fabric, --deployment-type, --multirail) to select the profile manually. Reason: %s", prompt["reasoning"])
			}

			for _, plugin := range l.plugins {
				if err := plugin.BuildProfileFromLLMResponse(prompt, fullConfig.Profile); err != nil {
					return fmt.Errorf("failed to build profile for plugin %s: %w", plugin.GetName(), err)
				}
			}

			l.logger.Info("Selected options",
				"fabric", fullConfig.Profile.Fabric,
				"deployment", fullConfig.Profile.Deployment,
				"multirail", fullConfig.Profile.Multirail,
				"spectrumX", fullConfig.Profile.SpectrumX,
				"ai", fullConfig.Profile.Ai,
				"reasoning", prompt["reasoning"])
		} else {
			return fmt.Errorf("no profile configured in the command line and no prompt provided")
		}
	}

	foundProfiles := []profiles.Profile{}
	for pluginName, plugin := range l.plugins {
		profile, err := profiles.FindApplicableProfile(fullConfig.Profile, fullConfig.ClusterConfig.Capabilities, pluginName)
		if err != nil {
			l.logger.Error(err, "Failed to find applicable profile for the plugin", "plugin", plugin.GetName(), "cluster capabilities", fullConfig.ClusterConfig.Capabilities, "profile requirements", fullConfig.Profile)
			return err
		}
		foundProfiles = append(foundProfiles, *profile)
	}

	for _, profile := range foundProfiles {
		l.logger.Info("Generating deployment files for profile", "profile", profile.Name)

		if err := l.generateDeploymentFiles(&profile, fullConfig); err != nil {
			return fmt.Errorf("deployment files generation failed: %w", err)
		}
	}

	// Phase 3: Cluster Deployment
	if l.options.Deploy {
		for _, profile := range foundProfiles {
			if err := l.deployConfigurationProfile(&profile); err != nil {
				return fmt.Errorf("deployment failed: %w", err)
			}
		}
	}

	l.logger.Info("l8k workflow completed successfully")
	return nil
}

// discoverClusterConfig handles cluster configuration discovery
func (l *Launcher) discoverClusterConfig() error {
	if l.options.UserConfig != "" {
		l.logger.Info("Using provided user config", "path", l.options.UserConfig)
		// TODO: Validate and load user config file
		return nil
	}

	l.logger.Info("Discovering cluster configuration")

	// Load defaults from l8k-config.yaml (temporary default path)
	defaultsPath := "l8k-config.yaml"
	defaults, err := config.LoadFullConfig(defaultsPath, l.logger)
	if err != nil {
		return fmt.Errorf("failed to load default config from %s: %w", defaultsPath, err)
	}

	defaults.ClusterConfig = &config.ClusterConfig{
		Capabilities: &config.ClusterCapabilities{
			Nodes: &config.NodesCapabilities{},
		},
		PFs:          []config.PFConfig{},
		WorkerNodes:  []string{},
		NodeSelector: map[string]string{"feature.node.kubernetes.io/pci-15b3.present": "true"},
	}
	defaults.Profile = nil

	for _, plugin := range l.plugins {
		err := plugin.DiscoverClusterConfig(context.Background(), l.kubeClient, defaults)
		if err != nil {
			return fmt.Errorf("failed to discover cluster config: %w", err)
		}
	}

	discoveredConfig := *defaults

	// Ensure output path provided
	if l.options.SaveClusterConfig == "" {
		return fmt.Errorf("no output path provided for discovered cluster config (use --discover-cluster-config)")
	}

	// Marshal and save merged config to disk
	data, err := yaml.Marshal(discoveredConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal discovered config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(l.options.SaveClusterConfig), 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", filepath.Dir(l.options.SaveClusterConfig), err)
	}
	if err := os.WriteFile(l.options.SaveClusterConfig, data, 0644); err != nil {
		return fmt.Errorf("failed to write discovered config to %s: %w", l.options.SaveClusterConfig, err)
	}

	l.logger.Info("Discovered cluster config saved", "path", l.options.SaveClusterConfig)
	return nil
}

// generateDeploymentFiles handles deployment file generation
func (l *Launcher) generateDeploymentFiles(profile *profiles.Profile, clusterConfig *config.LaunchKubernetesConfig) error {
	l.logger.Info("Generating deployment files", "profile", profile.Name)
	l.logger.Info("Generating deployment files", "config", clusterConfig)

	plugin, ok := l.plugins[profile.Plugin]
	if !ok {
		return fmt.Errorf("plugin %s not found", profile.Plugin)
	}

	renderedFiles, err := plugin.GenerateProfileDeploymentFiles(profile, clusterConfig)
	if err != nil {
		return fmt.Errorf("failed to process profile templates: %w", err)
	}

	if l.options.SaveDeploymentFiles != "" {
		if err := l.saveDeploymentFiles(renderedFiles, filepath.Join(l.options.SaveDeploymentFiles, profile.Plugin)); err != nil {
			return fmt.Errorf("failed to save deployment files: %w", err)
		}
	}

	return nil
}

// saveDeploymentFiles saves the rendered deployment files to disk
func (l *Launcher) saveDeploymentFiles(renderedFiles map[string]string, outputDir string) error {
	l.logger.Info("Saving deployment files", "directory", outputDir)

	// Clean the output directory before saving files
	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("failed to clean output directory %s: %w", outputDir, err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	for filename, content := range renderedFiles {
		outputPath := fmt.Sprintf("%s/%s", outputDir, filename)

		if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", outputPath, err)
		}

		l.logger.Info("Saved deployment file", "file", outputPath)
	}

	l.logger.Info("All deployment files saved successfully",
		"directory", outputDir,
		"fileCount", len(renderedFiles))

	return nil
}

// deployConfigurationProfile handles cluster deployment
func (l *Launcher) deployConfigurationProfile(profile *profiles.Profile) error {
	if !l.options.Deploy {
		l.logger.Info("Skipped (deploy not requested)")
		return nil
	}

	l.logger.Info("Deploying profile to cluster", "profile", profile.Name, "kubeconfig", l.options.Kubeconfig)

	if l.options.SaveDeploymentFiles == "" {
		return fmt.Errorf("--deploy requires generated files directory; provide --save-deployment-files")
	}

	plugin, ok := l.plugins[profile.Plugin]
	if !ok {
		return fmt.Errorf("plugin %s not found", profile.Plugin)
	}

	if err := plugin.DeployProfile(context.Background(), profile, l.kubeClient, filepath.Join(l.options.SaveDeploymentFiles, profile.Plugin)); err != nil {
		return fmt.Errorf("failed to deploy profile: %w", err)
	}

	l.logger.Info("Deployment profile applied successfully", "profile", profile.Name)
	return nil
}

// runInteractiveSession runs an interactive chat session with the LLM
func (l *Launcher) runInteractiveSession(clusterConfig *config.ClusterConfig) (map[string]string, error) {
	session, err := llm.NewChatSession(*clusterConfig, l.options.LLMApiKey, l.options.LLMApiUrl, l.options.LLMVendor, l.options.LLMModel)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat session: %w", err)
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n=== Interactive LLM Session ===")
	fmt.Println("Ask questions about network configuration or describe your requirements.")
	fmt.Println("Type 'generate' to generate manifests based on the recommended profile.")
	fmt.Println("Type 'exit' or 'quit' to cancel.")
	fmt.Println("================================\n")

	for {
		fmt.Print("You: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read input: %w", err)
		}

		input = strings.TrimSpace(input)

		// Check for exit commands
		if strings.EqualFold(input, "exit") || strings.EqualFold(input, "quit") {
			return nil, fmt.Errorf("session cancelled by user")
		}

		// Check for generate command
		if strings.EqualFold(input, "generate") {
			fmt.Println("\nExtracting profile from last response...")
			profile, err := session.ExtractProfile()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				fmt.Println("Please ask a question first to get a profile recommendation.")
				continue
			}

			confidence := profile["confidence"]
			if confidence == "low" {
				fmt.Printf("\nWarning: The LLM has low confidence in this recommendation.\n")
				fmt.Printf("Reason: %s\n", profile["reasoning"])
				fmt.Print("Do you want to proceed anyway? (yes/no): ")

				confirm, _ := reader.ReadString('\n')
				confirm = strings.TrimSpace(strings.ToLower(confirm))
				if confirm != "yes" && confirm != "y" {
					fmt.Println("Cancelled. Ask another question or refine your requirements.")
					continue
				}
			}

			fmt.Println("\nProceeding with profile generation...")
			return profile, nil
		}

		// Send message to LLM
		response, err := session.SendMessage(context.Background(), input)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("\nAssistant: %s", response)
		fmt.Println(llm.InteractivePromptSuffix)
		fmt.Println()
	}
}
