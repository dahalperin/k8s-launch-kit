package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/nvidia/k8s-launch-kit/pkg/app"
	applog "github.com/nvidia/k8s-launch-kit/pkg/log"
	"github.com/nvidia/k8s-launch-kit/pkg/profiles"
)

var (
	logLevel              string
	profile               string
	prompt                string
	saveDeploymentFiles   string
	deploy                bool
	kubeconfig            string
	userConfig            string
	discoverClusterConfig bool
	saveClusterConfig     string
	logger                = log.Log.WithName("l8k")
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "l8k",
	Short: "Network Operator deployment and configuration tool",
	Long: `l8k is a CLI tool for deploying and managing NVIDIA Network Operator on Kubernetes.

The tool operates in 3 phases:

1. DISCOVER CLUSTER CONFIG: Deploy a thin profile of the Network Operator to discover 
   the cluster configuration and capabilities. This phase can be skipped if you provide 
   your own configuration with --user-options.

2. GENERATE DEPLOYMENT FILES: Based on the discovered or provided configuration, 
   generate a complete set of YAML deployment files for the selected network profile. 
   Files can be saved to disk using --save-deployment-files.

3. DEPLOY TO CLUSTER: Apply the generated deployment files to your Kubernetes cluster. 
   This phase requires --kubeconfig and can be skipped if --deploy is not specified.

This tool helps you deploy network profiles and configure cluster settings for optimal 
network performance with SR-IOV, RDMA, and other networking technologies.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Create application options from CLI flags
		options := app.Options{
			LogLevel:              logLevel,
			UserConfig:            userConfig,
			DiscoverClusterConfig: discoverClusterConfig,
			Profile:               profile,
			Prompt:                prompt,
			SaveDeploymentFiles:   saveDeploymentFiles,
			Deploy:                deploy,
			Kubeconfig:            kubeconfig,
			SaveClusterConfig:     saveClusterConfig,
		}

		// Validate CLI configuration
		if err := validateConfig(options); err != nil {
			logger.Error(err, "Invalid command line arguments")
			os.Exit(1)
		}

		logger.Info("SaveConfig", "val", options)

		// Create and run the application
		launcher := app.New(options)
		if err := launcher.Run(); err != nil {
			logger.Error(err, "Application execution failed")
			os.Exit(1)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Phase 1: Cluster discovery flags
	rootCmd.Flags().BoolVar(&discoverClusterConfig, "discover-cluster-config", false, "Deploy a thin Network Operator profile to discover cluster capabilities")
	rootCmd.Flags().StringVar(&saveClusterConfig, "save-cluster-config", "/opt/nvidia/k8s-launch-kit/cluster-config.yaml", "Save discovered cluster configuration to the specified path")
	rootCmd.Flags().StringVar(&userConfig, "user-config", "", "Use provided cluster configuration file instead of auto-discovery (skips cluster discovery)")

	// Phase 2: Deployment generation flags
	rootCmd.Flags().StringVar(&profile, "profile", "", "Select the network profile to generate deployment files for ("+profiles.GetProfilesString()+")")
	rootCmd.Flags().StringVar(&prompt, "prompt", "", "Path to file with a prompt to use for LLM-assisted profile generation")
	rootCmd.Flags().StringVar(&saveDeploymentFiles, "save-deployment-files", "/opt/nvidia/k8s-launch-kit/deployment", "Save generated deployment files to the specified directory")

	// Phase 3: Cluster deployment flags
	rootCmd.Flags().BoolVar(&deploy, "deploy", false, "Deploy the generated files to the Kubernetes cluster")
	rootCmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file for cluster deployment (required when using --deploy)")
	// Log level flag
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
}

// validateConfig validates the CLI flag combinations
func validateConfig(options app.Options) error {
	// Rule 1: Either user-config or discover-cluster-config should be provided
	if options.UserConfig == "" && !options.DiscoverClusterConfig {
		return fmt.Errorf("either --user-config or --discover-cluster-config must be provided")
	}

	// Rule 1b: Both user-config and discover-cluster-config cannot be provided together
	if options.UserConfig != "" && options.DiscoverClusterConfig {
		return fmt.Errorf("--user-config and --discover-cluster-config cannot be used together")
	}

	// Rule 2: if profile is selected, either save-deployment-files or deploy options should be provided
	if (options.Profile != "" || options.Prompt != "") && options.SaveDeploymentFiles == "" && !options.Deploy {
		return fmt.Errorf("when --profile or --prompt is specified, either --save-deployment-files or --deploy must be provided")
	}

	// Rule 3: save-deployment-files or deploy can't work without profile
	if options.Profile == "" && options.Prompt == "" && (options.Deploy || options.SaveDeploymentFiles != "") {
		return fmt.Errorf("--deploy requires --profile or --prompt to be specified")
	}

	// Rule 4: if deploy is provided, kubeconfig should be too
	if options.Deploy && options.Kubeconfig == "" {
		return fmt.Errorf("--deploy requires --kubeconfig to be specified")
	}

	if options.Prompt != "" && options.Profile != "" {
		return fmt.Errorf("--prompt and --profile cannot be used together")
	}

	// Validate profile if provided
	if options.Profile != "" {
		if !profiles.IsValidProfile(options.Profile) {
			availableProfiles, _ := profiles.GetAvailableProfiles()
			return fmt.Errorf("invalid profile '%s', available profiles: %v", options.Profile, availableProfiles)
		}
		logger.Info("Using profile", "profile", options.Profile)
	}

	return nil
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Initialize logging
	applog.InitLog()

	// Implementation for config initialization
	// This can be expanded later to read from config files
}
