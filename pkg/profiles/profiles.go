package profiles

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nvidia/k8s-launch-kit/pkg/config"
	"gopkg.in/yaml.v2"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ProfileRequirements struct {
	Fabric     string `yaml:"fabric"`
	Deployment string `yaml:"deployment"`
	Multirail  *bool  `yaml:"multirail"`
	SpectrumX  *bool  `yaml:"spectrumX"`
	Ai         *bool  `yaml:"ai"`
}

type NodeCapabilities struct {
	Sriov *bool `yaml:"sriov"`
	Rdma  *bool `yaml:"rdma"`
	Ib    *bool `yaml:"ib"`
}

type Profile struct {
	Name                string
	Description         string
	ProfileRequirements ProfileRequirements `yaml:"profileRequirements"`
	NodeCapabilities    NodeCapabilities    `yaml:"nodeCapabilities"`
	DeploymentGuide     string
	Templates           []string
}

const ProfilesDir = "profiles"

func FindApplicableProfile(requirements *config.Profile, capabilities *config.ClusterCapabilities) (*Profile, error) {
	log.Log.Info("Finding applicable profile", "requirements", requirements)
	entries, err := os.ReadDir(ProfilesDir)
	if err != nil {
		return nil, err
	}

	log.Log.V(1).Info("Found profiles", "count", len(entries))

	errorMessages := []string{}

	for _, entry := range entries {
		if entry.IsDir() {
			profileManifest := filepath.Join(ProfilesDir, entry.Name(), "profile.yaml")
			profileData, err := os.ReadFile(profileManifest)
			if err != nil {
				log.Log.Error(err, "failed to read profile manifest", "profileManifest", profileManifest)
				return nil, err
			}
			profile := &Profile{}
			err = yaml.Unmarshal(profileData, profile)
			if err != nil {
				log.Log.Error(err, "failed to unmarshal profile manifest", "profileManifest", profileManifest)
				return nil, err
			}
			valid, reason := profile.Validate(requirements, capabilities)
			if valid {
				log.Log.V(1).Info("Found applicable profile", "profile", profile)
				profile.UpdateManifestsPaths(filepath.Join(ProfilesDir, entry.Name()))
				return profile, nil
			} else {
				errorMessages = append(errorMessages, fmt.Sprintf("profile %s is not applicable: %s", entry.Name(), reason))
			}
		}
	}

	log.Log.Info("No applicable profile found based on the given requirements")
	for _, errorMessage := range errorMessages {
		log.Log.Error(fmt.Errorf(errorMessage), "errorMessage")
	}

	return nil, fmt.Errorf("no applicable profile found")
}

func (p *Profile) Validate(requirements *config.Profile, capabilities *config.ClusterCapabilities) (bool, string) {
	log.Log.V(1).Info("Validating profile", "profile", p)

	if p.ProfileRequirements.Fabric != "" && p.ProfileRequirements.Fabric != requirements.Fabric {
		return false, fmt.Sprintf("selected fabric type does not match profile requirements: %s", p.ProfileRequirements.Fabric)
	}

	if p.ProfileRequirements.Deployment != "" && p.ProfileRequirements.Deployment != requirements.Deployment {
		return false, fmt.Sprintf("selected deployment type does not match profile requirements: %s", p.ProfileRequirements.Deployment)
	}

	if p.ProfileRequirements.Multirail != nil && *p.ProfileRequirements.Multirail != requirements.Multirail {
		return false, fmt.Sprintf("selected multirail setting does not match profile requirements: %t", *p.ProfileRequirements.Multirail)
	}

	if p.ProfileRequirements.SpectrumX != nil {
		if !(*p.ProfileRequirements.SpectrumX) && !requirements.SpectrumX {
			return false, fmt.Sprintf("profile is not applicable to Spectrum-X clusters: %t", *p.ProfileRequirements.SpectrumX)
		}
		if *p.ProfileRequirements.SpectrumX && requirements.SpectrumX {
			return false, fmt.Sprintf("profile can obly be deployed on Spectrum-X clusters: %t", *p.ProfileRequirements.SpectrumX)
		}
	}

	if p.ProfileRequirements.Ai != nil {
		if !(*p.ProfileRequirements.Ai) && requirements.Ai {
			return false, fmt.Sprintf("profile is not applicable to AI clusters: %t", *p.ProfileRequirements.Ai)
		}
		if *p.ProfileRequirements.Ai && !requirements.Ai {
			return false, fmt.Sprintf("profile can only be deployed on AI clusters: %t", *p.ProfileRequirements.Ai)
		}
	}

	if p.NodeCapabilities.Sriov != nil && *p.NodeCapabilities.Sriov != capabilities.Nodes.Sriov {
		return false, fmt.Sprintf("cluster sriov capability does not match profile requirements: %t", *p.NodeCapabilities.Sriov)
	}
	if p.NodeCapabilities.Rdma != nil && *p.NodeCapabilities.Rdma != capabilities.Nodes.Rdma {
		return false, fmt.Sprintf("cluster rdma capability does not match profile requirements: %t", *p.NodeCapabilities.Rdma)
	}
	if p.NodeCapabilities.Ib != nil && *p.NodeCapabilities.Ib != capabilities.Nodes.Ib {
		return false, fmt.Sprintf("cluster ib capability does not match profile requirements: %t", *p.NodeCapabilities.Ib)
	}

	return true, ""
}

// UpdateManifestsPaths appends the directory path to the templates and deployment guide
func (p *Profile) UpdateManifestsPaths(dirPath string) {
	for i := range p.Templates {
		p.Templates[i] = filepath.Join(dirPath, p.Templates[i])
	}

	p.DeploymentGuide = filepath.Join(dirPath, p.DeploymentGuide)
}
