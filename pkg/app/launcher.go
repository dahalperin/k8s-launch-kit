package app

import (
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/nvidia/k8s-launch-kit/pkg/clusterconfig"
	applog "github.com/nvidia/k8s-launch-kit/pkg/log"
	"github.com/nvidia/k8s-launch-kit/pkg/templates"
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

	// Phase 2: Deployment Generation
	clusterConfig, err := clusterconfig.LoadClusterConfig(configPath, l.logger)
	if err != nil {
		return fmt.Errorf("failed to load cluster config: %w", err)
	}

	// Validate config for the selected profile
	if err := clusterconfig.ValidateClusterConfig(clusterConfig, l.options.Profile); err != nil {
		return fmt.Errorf("cluster config validation failed: %w", err)
	}

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
		l.logger.Info("Phase 1: Using provided user config", "path", l.options.UserConfig)
		// TODO: Validate and load user config file
		return nil
	}

	if l.options.DiscoverClusterConfig {
		l.logger.Info("Phase 1: Discovering cluster configuration", "outputPath", l.options.DiscoverClusterConfig)
		// TODO: Deploy thin profile and discover cluster config
		return nil
	}

	// This should not happen due to validation, but handle gracefully
	return fmt.Errorf("no cluster configuration source specified")
}

// generateDeploymentFiles handles deployment file generation
func (l *Launcher) generateDeploymentFiles(clusterConfig *clusterconfig.ClusterConfig) error {
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
		l.logger.Info("Phase 3: Skipped (deploy not requested)")
		return nil
	}

	l.logger.Info("Phase 3: Deploying to cluster", "kubeconfig", l.options.Kubeconfig)

	// TODO: Load kubeconfig
	// TODO: Apply generated deployment files to cluster
	// TODO: Wait for deployment completion
	// TODO: Verify deployment status

	return nil
}
