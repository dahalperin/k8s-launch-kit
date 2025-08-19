package clusterconfig

import (
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v2"
)

// ClusterConfig represents the l8k-config.yaml structure
type ClusterConfig struct {
	NetworkOperator NetworkOperatorConfig `yaml:"networkOperator"`
	NvIpam          NvIpamConfig          `yaml:"nvIpam"`
	Sriov           SriovConfig           `yaml:"sriov"`
	Hostdev         HostdevConfig         `yaml:"hostdev"`
	RdmaShared      RdmaSharedConfig      `yaml:"rdmaShared"`
	Ipoib           IpoibConfig           `yaml:"ipoib"`
	Macvlan         MacvlanConfig         `yaml:"macvlan"`
	ClusterConfig   ClusterConfigStruct   `yaml:"clusterConfig"`
}

type NetworkOperatorConfig struct {
	Version          string `yaml:"version"`
	ComponentVersion string `yaml:"componentVersion"`
	Repository       string `yaml:"repository"`
	Namespace        string `yaml:"namespace"`
}

type NvIpamConfig struct {
	PoolName string               `yaml:"poolName"`
	Subnets  []NvIpamSubnetConfig `yaml:"subnets"`
}

type NvIpamSubnetConfig struct {
	Subnet  string `yaml:"subnet"`
	Gateway string `yaml:"gateway"`
}

type SriovConfig struct {
	Mtu          int    `yaml:"mtu"`
	NumVfs       int    `yaml:"numVfs"`
	Priority     int    `yaml:"priority"`
	ResourceName string `yaml:"resourceName"`
	NetworkName  string `yaml:"networkName"`
}

type HostdevConfig struct {
	ResourceName string `yaml:"resourceName"`
	NetworkName  string `yaml:"networkName"`
}

type RdmaSharedConfig struct {
	ResourceName string `yaml:"resourceName"`
	HcaMax       int    `yaml:"hcaMax"`
}

type IpoibConfig struct {
	NetworkName                string `yaml:"networkName"`
	SeparateNetworkPerDevice   bool   `yaml:"separateNetworkPerDevice"`
	SingleNetworkForAllDevices bool   `yaml:"singleNetworkForAllDevices"`
}

type MacvlanConfig struct {
	NetworkName                string `yaml:"networkName"`
	SeparateNetworkPerDevice   bool   `yaml:"separateNetworkPerDevice"`
	SingleNetworkForAllDevices bool   `yaml:"singleNetworkForAllDevices"`
}

type ClusterConfigStruct struct {
	Nodes      NodesConfig      `yaml:"nodes"`
	NvidiaNICs NvidiaNICsConfig `yaml:"nvidiaNICs"`
}

type NvidiaNICsConfig struct {
	PF PFConfig `yaml:"pf"`
}

type NodesConfig struct {
	Capabilities CapabilitiesConfig `yaml:"capabilities"`
}

type CapabilitiesConfig struct {
	Sriov bool `yaml:"sriov"`
	Rdma  bool `yaml:"rdma"`
	Ib    bool `yaml:"ib"`
}

type PFConfig struct {
	RdmaDevices       []string `yaml:"rdmaDevices"`
	PciAddresses      []string `yaml:"pciAddresses"`
	NetworkInterfaces []string `yaml:"networkInterfaces"`
}

// LoadClusterConfig loads and parses the cluster configuration from the specified path
func LoadClusterConfig(configPath string, logger logr.Logger) (*ClusterConfig, error) {
	if configPath == "" {
		return nil, fmt.Errorf("no cluster configuration path provided")
	}

	logger.Info("Loading cluster configuration", "path", configPath)

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("cluster config file does not exist: %s", configPath)
	}

	// Read the configuration file
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cluster config file %s: %w", configPath, err)
	}

	// Parse the YAML configuration
	var config ClusterConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse cluster config YAML %s: %w", configPath, err)
	}

	logger.Info("Cluster configuration loaded successfully",
		"networkOperatorVersion", config.NetworkOperator.Version,
		"namespace", config.NetworkOperator.Namespace)

	return &config, nil
}

// ValidateClusterConfig validates that essential fields are present in the cluster config
func ValidateClusterConfig(config *ClusterConfig, profile string) error {
	if config.NetworkOperator.Repository == "" {
		return fmt.Errorf("networkOperator.repository is required")
	}

	if config.NetworkOperator.ComponentVersion == "" {
		return fmt.Errorf("networkOperator.componentVersion is required")
	}

	if config.NetworkOperator.Namespace == "" {
		return fmt.Errorf("networkOperator.namespace is required")
	}

	// Validate profile-specific requirements based on the selected profile
	if profile == "host-device-rdma" || profile == "hostdevice" {
		if config.Hostdev.ResourceName == "" {
			return fmt.Errorf("hostdev.resourceName is required for hostdevice profiles")
		}
		if config.Hostdev.NetworkName == "" {
			return fmt.Errorf("hostdev.networkName is required for hostdevice profiles")
		}
	}

	if profile == "sriov-rdma" || profile == "sriov-ib-rdma" {
		if config.Sriov.ResourceName == "" {
			return fmt.Errorf("sriov.resourceName is required for SR-IOV profiles")
		}
		if config.Sriov.NetworkName == "" {
			return fmt.Errorf("sriov.networkName is required for SR-IOV profiles")
		}
	}

	return nil
}
