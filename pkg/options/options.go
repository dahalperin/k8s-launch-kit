package options

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

	EnabledPlugins []string // Enabled plugins

	// Phase 3: Cluster Deployment
	Deploy     bool   // Whether to deploy to cluster
	Kubeconfig string // Path to kubeconfig for discovery and deployment
}
