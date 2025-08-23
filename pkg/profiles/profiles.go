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
			if profile.Validate(requirements, capabilities) {
				log.Log.V(1).Info("Found applicable profile", "profile", profile)
				profile.UpdateManifestsPaths(filepath.Join(ProfilesDir, entry.Name()))
				return profile, nil
			}
		}
	}
	return nil, fmt.Errorf("no applicable profile found")
}

func (p *Profile) Validate(requirements *config.Profile, capabilities *config.ClusterCapabilities) bool {
	log.Log.V(1).Info("Validating profile", "profile", p)

	if p.ProfileRequirements.Fabric != "" && p.ProfileRequirements.Fabric != requirements.Fabric {
		log.Log.V(1).Info("Cluster fabric does not match profile requirements", "profile", p, "requirements", requirements)
		return false
	}

	if p.ProfileRequirements.Deployment != "" && p.ProfileRequirements.Deployment != requirements.Deployment {
		log.Log.V(1).Info("Cluster deployment does not match profile requirements", "profile", p, "requirements", requirements)
		return false
	}

	if p.ProfileRequirements.Multirail != nil && *p.ProfileRequirements.Multirail != requirements.Multirail {
		log.Log.V(1).Info("Cluster multirail does not match profile requirements", "profile", p, "requirements", requirements)
		return false
	}

	if p.ProfileRequirements.SpectrumX != nil && *p.ProfileRequirements.SpectrumX != requirements.SpectrumX {
		log.Log.V(1).Info("Cluster spectrumX does not match profile requirements", "profile", p, "requirements", requirements)
		return false
	}

	if p.ProfileRequirements.Ai != nil && *p.ProfileRequirements.Ai != requirements.Ai {
		log.Log.V(1).Info("Cluster ai does not match profile requirements", "profile", p, "requirements", requirements)
		return false
	}

	if p.NodeCapabilities.Sriov != nil && *p.NodeCapabilities.Sriov != capabilities.Nodes.Sriov {
		log.Log.V(1).Info("Cluster sriov capability does not match profile requirements", "profile", p, "requirements", requirements)
		return false
	}
	if p.NodeCapabilities.Rdma != nil && *p.NodeCapabilities.Rdma != capabilities.Nodes.Rdma {
		log.Log.V(1).Info("Cluster rdma capability does not match profile requirements", "profile", p, "requirements", requirements)
		return false
	}
	if p.NodeCapabilities.Ib != nil && *p.NodeCapabilities.Ib != capabilities.Nodes.Ib {
		log.Log.V(1).Info("Cluster ib capability does not match profile requirements", "profile", p, "requirements", requirements)
		return false
	}

	return true
}

// UpdateManifestsPaths appends the directory path to the templates and deployment guide
func (p *Profile) UpdateManifestsPaths(dirPath string) {
	for i := range p.Templates {
		p.Templates[i] = filepath.Join(dirPath, p.Templates[i])
	}

	p.DeploymentGuide = filepath.Join(dirPath, p.DeploymentGuide)
}
