package app

import (
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"context"

	"github.com/nvidia/k8s-launch-kit/pkg/config"
	"github.com/nvidia/k8s-launch-kit/pkg/deploy"
	"github.com/nvidia/k8s-launch-kit/pkg/discovery"
	"github.com/nvidia/k8s-launch-kit/pkg/kubeclient"
	"github.com/nvidia/k8s-launch-kit/pkg/llm"
	applog "github.com/nvidia/k8s-launch-kit/pkg/log"
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
	Profile             string // Network profile to deploy
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
		logger:  log.Log.WithName("l8k"),
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
		return fmt.Errorf("workflow execution failed: %w", err)
	}

	return nil
}

// executeWorkflow executes the main 3-phase workflow
func (l *Launcher) executeWorkflow() error {
	l.logger.Info("Starting l8k workflow")

	configPath := ""
	// Phase 1: Cluster Discovery
	if l.options.DiscoverClusterConfig {
		if err := l.discoverClusterConfig(); err != nil {
			return fmt.Errorf("cluster discovery failed: %w", err)
		}

		configPath = l.options.SaveClusterConfig
	} else {
		configPath = l.options.UserConfig
	}

	if l.options.Profile == "" && l.options.Prompt == "" {
		l.logger.Info("No profile specified, skipping deployment files generation")
		return nil
	}

	clusterConfig, err := config.LoadClusterConfig(configPath, l.logger)
	if err != nil {
		return fmt.Errorf("failed to load cluster config: %w", err)
	}

	if l.options.Prompt != "" {
		l.logger.Info("Selecting a profile using LLM-assisted prompt")
		prompt, err := llm.SelectPrompt(l.options.Prompt, *clusterConfig)
		if err != nil {
			return fmt.Errorf("failed to select prompt: %w", err)
		}
		if err, found := prompt["error"]; found {
			return fmt.Errorf("failed to select profile: %s. Reasoning: %s", err, prompt["reasoning"])
		}
		l.options.Profile = prompt["selected_usecase"]
		l.logger.Info("Selected profile", "profile", l.options.Profile, "reasoning", prompt["reasoning"], "key_factors", prompt["key_factors"])
	}

	l.logger.Info("Validating selected profile against cluster configuration", "profile", l.options.Profile)

	// Validate config for the selected profile
	if err := config.ValidateClusterConfig(clusterConfig, l.options.Profile); err != nil {
		return fmt.Errorf("cluster config validation failed: %w", err)
	}

	l.logger.Info("Generating deployment files for profile", "profile", l.options.Profile)

	if err := l.generateDeploymentFiles(clusterConfig); err != nil {
		return fmt.Errorf("deployment files generation failed: %w", err)
	}

	// Phase 3: Cluster Deployment
	if l.options.Deploy {
		if err := l.deployConfigurationProfile(); err != nil {
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
	defaults, err := config.LoadClusterConfig(defaultsPath, l.logger)
	if err != nil {
		return fmt.Errorf("failed to load default config from %s: %w", defaultsPath, err)
	}

	// Build Kubernetes client
	k8sClient, err := kubeclient.New(l.options.Kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	// Discover cluster config using client and defaults for network-operator
	clusterCfg, err := discovery.DiscoverClusterConfig(context.Background(), k8sClient, &defaults.NetworkOperator)
	if err != nil {
		return fmt.Errorf("failed to discover cluster config: %w", err)
	}

	// Merge discovered cluster config into defaults
	defaults.ClusterConfig = clusterCfg

	// Ensure output path provided
	if l.options.SaveClusterConfig == "" {
		return fmt.Errorf("no output path provided for discovered cluster config (use --discover-cluster-config)")
	}

	// Marshal and save merged config to disk
	data, err := yaml.Marshal(defaults)
	if err != nil {
		return fmt.Errorf("failed to marshal discovered config: %w", err)
	}
	if err := os.WriteFile(l.options.SaveClusterConfig, data, 0644); err != nil {
		return fmt.Errorf("failed to write discovered config to %s: %w", l.options.SaveClusterConfig, err)
	}

	l.logger.Info("Discovered cluster config saved", "path", l.options.SaveClusterConfig)
	return nil
}

// generateDeploymentFiles handles deployment file generation
func (l *Launcher) generateDeploymentFiles(clusterConfig *config.LaunchKubernetesConfig) error {
	l.logger.Info("Generating deployment files", "profile", l.options.Profile)

	profileDir := fmt.Sprintf("profiles/%s", l.options.Profile)

	l.logger.Info("Processing profile templates", "profileDir", profileDir)

	renderedFiles, err := templates.ProcessProfileTemplates(profileDir, *clusterConfig)
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

	// Create output directory if it doesn't exist
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
func (l *Launcher) deployConfigurationProfile() error {
	if !l.options.Deploy {
		l.logger.Info("Skipped (deploy not requested)")
		return nil
	}

	l.logger.Info("Deploying profile to cluster", "profile", l.options.Profile, "kubeconfig", l.options.Kubeconfig)

	if l.options.SaveDeploymentFiles == "" {
		return fmt.Errorf("--deploy requires generated files directory; provide --save-deployment-files")
	}

	if err := deploy.Apply(context.Background(), l.options.Kubeconfig, l.options.SaveDeploymentFiles); err != nil {
		return fmt.Errorf("failed to deploy manifests: %w", err)
	}

	l.logger.Info("Deployment profile applied successfully", "profile", l.options.Profile)
	return nil
}
