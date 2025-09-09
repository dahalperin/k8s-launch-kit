package plugin

import (
	"context"

	"github.com/nvidia/k8s-launch-kit/pkg/config"
	"github.com/nvidia/k8s-launch-kit/pkg/options"
	"github.com/nvidia/k8s-launch-kit/pkg/profiles"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Plugin defines the interface to implement for tool support for the Launch Kit.
// To integrate a new tool, implement this interface and add the plugin to the plugins map in the main package.
// CLI flags and config type should be defined in the tool, not in the plugin.
// Configuration templates should be stored in the tool's directory. Additional make targets can be used for template provisioning.
type Plugin interface {
	// GetName returns the name of the plugin. The name will be used to enable / disable plugins and match profiles.
	GetName() string
	// GetVersion returns the version of the plugin. The version will be used to check if the plugin is compatible with the current version of the tool.
	GetVersion() string
	// ProfileConfiguredInCmd returns true if the profile is configured for the plugin based on the options.
	ProfileConfiguredInCmd(options options.Options) bool
	// BuildProfileFromOptions builds the profile for the plugin based on the options.
	BuildProfileFromOptions(options options.Options, profile *config.Profile) error
	// BuildProfileFromLLMResponse builds the profile for the plugin based on the LLM response.
	BuildProfileFromLLMResponse(llmResponse map[string]string, profile *config.Profile) error
	// GetSystemPromptAddendum returns the addendum to the system prompt, specific to the plugin. The addendum will be used to add additional context to the system prompt.
	GetSystemPromptAddendum() (string, error)
	// DiscoverClusterConfig discovers the plugin-specific part of the cluster configuration and adds it to the given LaunchKubernetesConfig.
	// Should not reassign defaultConfig.ClusterConfig, only edit it.
	DiscoverClusterConfig(ctx context.Context, kubeClient client.Client, defaultConfig *config.LaunchKubernetesConfig) error
	// GenerateProfileDeploymentFiles generates the deployment files for the profile.
	GenerateProfileDeploymentFiles(profile *profiles.Profile, config *config.LaunchKubernetesConfig) (map[string]string, error)
	// DeployProfile deploys the profile to the cluster.
	DeployProfile(ctx context.Context, profile *profiles.Profile, kubeClient client.Client, manifestsDir string) error
}
