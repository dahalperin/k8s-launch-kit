package networkoperatorplugin

import (
	"os"

	"github.com/nvidia/k8s-launch-kit/pkg/config"
	"github.com/nvidia/k8s-launch-kit/pkg/options"
	"github.com/nvidia/k8s-launch-kit/pkg/plugin"
	"github.com/nvidia/k8s-launch-kit/pkg/profiles"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	PluginName    = "network-operator"
	PluginVersion = "1.0.0"
)

type NetworkOperatorPlugin struct {
}

func (p *NetworkOperatorPlugin) GetName() string {
	return PluginName
}

func (p *NetworkOperatorPlugin) GetVersion() string {
	return PluginVersion
}

func (p *NetworkOperatorPlugin) ProfileConfiguredInCmd(options options.Options) bool {
	return options.Fabric != "" || options.DeploymentType != ""
}

func (p *NetworkOperatorPlugin) BuildProfileFromOptions(options options.Options, profile *config.Profile) error {
	profile.Fabric = options.Fabric
	profile.Deployment = options.DeploymentType
	profile.Multirail = options.Multirail
	profile.SpectrumX = options.SpectrumX
	profile.Ai = options.Ai

	log.Log.V(1).Info("Built profile for plugin", "plugin", p.GetName(), "profile", profile)
	return nil
}

func (p *NetworkOperatorPlugin) BuildProfileFromLLMResponse(llmResponse map[string]string, profile *config.Profile) error {
	profile.Fabric = llmResponse["fabric"]
	profile.Deployment = llmResponse["deploymentType"]
	profile.Multirail = llmResponse["multirail"] == "true"
	profile.SpectrumX = llmResponse["spectrumX"] == "true"
	profile.Ai = llmResponse["ai"] == "true"

	log.Log.V(1).Info("Built profile for plugin", "plugin", p.GetName(), "profile", profile)
	return nil
}

func (p *NetworkOperatorPlugin) GetSystemPromptAddendum() (string, error) {
	data, err := os.ReadFile("network-operator-system-prompt-addendum")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (p *NetworkOperatorPlugin) SelectProfile(config *config.LaunchKubernetesConfig) (*profiles.Profile, error) {
	return nil, nil
}

var _ plugin.Plugin = &NetworkOperatorPlugin{}
