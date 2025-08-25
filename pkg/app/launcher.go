package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"context"

	"github.com/nvidia/k8s-launch-kit/pkg/config"
	"github.com/nvidia/k8s-launch-kit/pkg/deploy"
	"github.com/nvidia/k8s-launch-kit/pkg/discovery"
	"github.com/nvidia/k8s-launch-kit/pkg/kubeclient"
	"github.com/nvidia/k8s-launch-kit/pkg/llm"
	applog "github.com/nvidia/k8s-launch-kit/pkg/log"
	"github.com/nvidia/k8s-launch-kit/pkg/profiles"
	"github.com/nvidia/k8s-launch-kit/pkg/templates"
	"gopkg.in/yaml.v2"
)

// Options holds all the configuration parameters for the application
type Options struct {
	// Logging
	LogLevel string

	// Phase 1: Cluster Discovery
	UserConfig            string // Path to user-provided config (skips discovery)
	DiscoverClusterConfig bool   // Whether to discover cluster config
	SaveClusterConfig     string // Path to save discovered config

	// Phase 2: Deployment Generation
	Fabric              string // Fabric type to deploy
	DeploymentType      string // Deployment type to deploy
	Multirail           bool   // Whether to deploy with multirail
	SpectrumX           bool   // Whether to deploy with Spectrum X
	Ai                  bool   // Whether to deploy with AI
	Prompt              string // Path to file with a prompt to use for LLM-assisted profile generation
	SaveDeploymentFiles string // Directory to save generated files

	// Phase 3: Cluster Deployment
	Deploy     bool   // Whether to deploy to cluster
	Kubeconfig string // Path to kubeconfig for deployment
}

// Launcher represents the main application launcher
type Launcher struct {
	options Options
	logger  logr.Logger
}

// New creates a new Launcher instance with the given options
func New(options Options) *Launcher {
	return &Launcher{
		options: options,
		logger:  log.Log,
	}
}

// Run executes the main application logic with the 3-phase workflow
func (l *Launcher) Run() error {
	if l.options.LogLevel != "" {
		if err := applog.SetLogLevel(l.options.LogLevel); err != nil {
			return fmt.Errorf("failed to set log level: %w", err)
		}
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

	if l.options.Fabric == "" && l.options.DeploymentType == "" && l.options.Prompt == "" {
		l.logger.Info("No fabric, deployment type or prompt specified, skipping deployment files generation")
		return nil
	}

	fullConfig, err := config.LoadFullConfig(configPath, l.logger)
	if err != nil {
		return fmt.Errorf("failed to load full config: %w", err)
	}

	if l.options.UserConfig == "" && l.options.Prompt == "" {
		fullConfig.Profile = &config.Profile{
			Fabric:     l.options.Fabric,
			Deployment: l.options.DeploymentType,
			Multirail:  l.options.Multirail,
			SpectrumX:  l.options.SpectrumX,
			Ai:         l.options.Ai,
		}
	} else if l.options.Prompt != "" {
		l.logger.Info("Selecting a profile using LLM-assisted prompt")

		prompt, err := llm.SelectPrompt(l.options.Prompt, *fullConfig.ClusterConfig)
		if err != nil {
			return fmt.Errorf("failed to select prompt: %w", err)
		}
		confidence := prompt["confidence"]
		if confidence == "low" {
			return fmt.Errorf("couldn't select a deployment profile based on the user prompt. Try again with a different prompt or use the cli flags (--fabric, --deployment-type, --multirail) to select the profile manually. Reason: %s", prompt["reasoning"])
		}
		fullConfig.Profile = &config.Profile{
			Fabric:     prompt["fabric"],
			Deployment: prompt["deploymentType"],
			Multirail:  prompt["multirail"] == "true",
			SpectrumX:  prompt["spectrumX"] == "true",
			Ai:         prompt["ai"] == "true",
		}

		l.logger.Info("Selected options", "fabric", fullConfig.Profile.Fabric, "deployment", fullConfig.Profile.Deployment, "multirail", fullConfig.Profile.Multirail, "spectrumX", fullConfig.Profile.SpectrumX, "ai", fullConfig.Profile.Ai)
	}

	profile, err := profiles.FindApplicableProfile(fullConfig.Profile, fullConfig.ClusterConfig.Capabilities)
	if err != nil {
		l.logger.Error(err, "Failed to find applicable profile for the cluster", "cluster capabilities", fullConfig.ClusterConfig.Capabilities, "profile requirements", fullConfig.Profile)
		return err
	}

	l.logger.Info("Generating deployment files for profile", "profile", profile.Name)

	if err := l.generateDeploymentFiles(profile, fullConfig); err != nil {
		return fmt.Errorf("deployment files generation failed: %w", err)
	}

	// Phase 3: Cluster Deployment
	if l.options.Deploy {
		if err := l.deployConfigurationProfile(profile); err != nil {
			return fmt.Errorf("deployment failed: %w", err)
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

	// Build Kubernetes client
	k8sClient, err := kubeclient.New(l.options.Kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	// Discover cluster config using client and defaults for network-operator
	clusterCfg, err := discovery.DiscoverClusterConfig(context.Background(), k8sClient, defaults.NetworkOperator)
	if err != nil {
		return fmt.Errorf("failed to discover cluster config: %w", err)
	}

	// Merge discovered cluster config into defaults
	discoveredConfig := *defaults
	discoveredConfig.ClusterConfig = &clusterCfg

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

	renderedFiles, err := templates.ProcessProfileTemplates(profile, *clusterConfig)
	if err != nil {
		return fmt.Errorf("failed to process profile templates: %w", err)
	}

	if l.options.SaveDeploymentFiles != "" {
		if err := l.saveDeploymentFiles(renderedFiles); err != nil {
			return fmt.Errorf("failed to save deployment files: %w", err)
		}
	}

	return nil
}

// saveDeploymentFiles saves the rendered deployment files to disk
func (l *Launcher) saveDeploymentFiles(renderedFiles map[string]string) error {
	l.logger.Info("Saving deployment files", "directory", l.options.SaveDeploymentFiles)

	// Clean the output directory before saving files
	if err := os.RemoveAll(l.options.SaveDeploymentFiles); err != nil {
		return fmt.Errorf("failed to clean output directory %s: %w", l.options.SaveDeploymentFiles, err)
	}
	if err := os.MkdirAll(l.options.SaveDeploymentFiles, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", l.options.SaveDeploymentFiles, err)
	}

	for filename, content := range renderedFiles {
		outputPath := fmt.Sprintf("%s/%s", l.options.SaveDeploymentFiles, filename)

		if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", outputPath, err)
		}

		l.logger.Info("Saved deployment file", "file", outputPath)
	}

	l.logger.Info("All deployment files saved successfully",
		"directory", l.options.SaveDeploymentFiles,
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

	if err := deploy.Apply(context.Background(), l.options.Kubeconfig, l.options.SaveDeploymentFiles); err != nil {
		return fmt.Errorf("failed to deploy manifests: %w", err)
	}

	l.logger.Info("Deployment profile applied successfully", "profile", profile.Name)
	return nil
}
