// Copyright 2025 NVIDIA CORPORATION & AFFILIATES
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFullConfig(t *testing.T) {
	logger := logr.Discard()

	t.Run("load valid config with separate MTU values", func(t *testing.T) {
		// Create a temporary config file
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "test-config.yaml")

		configContent := `networkOperator:
  version: v25.10.0
  componentVersion: network-operator-v25.10.0
  repository: nvcr.io/nvidia/mellanox
  namespace: nvidia-network-operator

sriov:
  ethernetMtu: 9000
  infinibandMtu: 4000
  numVfs: 8
  priority: 90
  resourceName: sriov_resource
  networkName: sriov_network

profile:
  fabric: ethernet
  deployment: sriov
  multirail: false
  spectrumX: false
  ai: false
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Load the config
		config, err := LoadFullConfig(configPath, logger)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Verify network operator config
		assert.Equal(t, "v25.10.0", config.NetworkOperator.Version)
		assert.Equal(t, "network-operator-v25.10.0", config.NetworkOperator.ComponentVersion)
		assert.Equal(t, "nvcr.io/nvidia/mellanox", config.NetworkOperator.Repository)
		assert.Equal(t, "nvidia-network-operator", config.NetworkOperator.Namespace)

		// Verify SR-IOV config with separate MTU values
		require.NotNil(t, config.Sriov)
		assert.Equal(t, 9000, config.Sriov.EthernetMtu, "Ethernet MTU should be 9000")
		assert.Equal(t, 4000, config.Sriov.InfinibandMtu, "Infiniband MTU should be 4000")
		assert.Equal(t, 8, config.Sriov.NumVfs)
		assert.Equal(t, 90, config.Sriov.Priority)
		assert.Equal(t, "sriov_resource", config.Sriov.ResourceName)
		assert.Equal(t, "sriov_network", config.Sriov.NetworkName)

		// Verify profile config
		require.NotNil(t, config.Profile)
		assert.Equal(t, "ethernet", config.Profile.Fabric)
		assert.Equal(t, "sriov", config.Profile.Deployment)
		assert.False(t, config.Profile.Multirail)
		assert.False(t, config.Profile.SpectrumX)
		assert.False(t, config.Profile.Ai)
	})

	t.Run("load config file that does not exist", func(t *testing.T) {
		_, err := LoadFullConfig("/nonexistent/path/config.yaml", logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("load config with empty path", func(t *testing.T) {
		_, err := LoadFullConfig("", logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no cluster configuration path provided")
	})

	t.Run("load invalid YAML config", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "invalid-config.yaml")

		invalidContent := `
networkOperator:
  version: v25.10.0
  this is not valid yaml content
    - broken indentation
`
		err := os.WriteFile(configPath, []byte(invalidContent), 0644)
		require.NoError(t, err)

		_, err = LoadFullConfig(configPath, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse cluster config YAML")
	})
}

func TestValidateClusterConfig(t *testing.T) {
	t.Run("validate config with missing network operator repository", func(t *testing.T) {
		config := &LaunchKubernetesConfig{
			NetworkOperator: &NetworkOperatorConfig{
				Version:          "v25.10.0",
				ComponentVersion: "network-operator-v25.10.0",
				Repository:       "", // Missing
				Namespace:        "nvidia-network-operator",
			},
		}

		err := ValidateClusterConfig(config, "sriov-rdma")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "networkOperator.repository is required")
	})

	t.Run("validate config with missing network operator component version", func(t *testing.T) {
		config := &LaunchKubernetesConfig{
			NetworkOperator: &NetworkOperatorConfig{
				Version:          "v25.10.0",
				ComponentVersion: "", // Missing
				Repository:       "nvcr.io/nvidia/mellanox",
				Namespace:        "nvidia-network-operator",
			},
		}

		err := ValidateClusterConfig(config, "sriov-rdma")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "networkOperator.componentVersion is required")
	})

	t.Run("validate config with missing network operator namespace", func(t *testing.T) {
		config := &LaunchKubernetesConfig{
			NetworkOperator: &NetworkOperatorConfig{
				Version:          "v25.10.0",
				ComponentVersion: "network-operator-v25.10.0",
				Repository:       "nvcr.io/nvidia/mellanox",
				Namespace:        "", // Missing
			},
		}

		err := ValidateClusterConfig(config, "sriov-rdma")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "networkOperator.namespace is required")
	})

	t.Run("validate sriov profile with missing resource name", func(t *testing.T) {
		config := &LaunchKubernetesConfig{
			NetworkOperator: &NetworkOperatorConfig{
				Version:          "v25.10.0",
				ComponentVersion: "network-operator-v25.10.0",
				Repository:       "nvcr.io/nvidia/mellanox",
				Namespace:        "nvidia-network-operator",
			},
			Sriov: &SriovConfig{
				EthernetMtu:   9000,
				InfinibandMtu: 4000,
				NumVfs:        8,
				Priority:      90,
				ResourceName:  "", // Missing
				NetworkName:   "sriov_network",
			},
		}

		err := ValidateClusterConfig(config, "sriov-rdma")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "sriov.resourceName is required")
	})

	t.Run("validate sriov profile with missing network name", func(t *testing.T) {
		config := &LaunchKubernetesConfig{
			NetworkOperator: &NetworkOperatorConfig{
				Version:          "v25.10.0",
				ComponentVersion: "network-operator-v25.10.0",
				Repository:       "nvcr.io/nvidia/mellanox",
				Namespace:        "nvidia-network-operator",
			},
			Sriov: &SriovConfig{
				EthernetMtu:   9000,
				InfinibandMtu: 4000,
				NumVfs:        8,
				Priority:      90,
				ResourceName:  "sriov_resource",
				NetworkName:   "", // Missing
			},
		}

		err := ValidateClusterConfig(config, "sriov-rdma")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "sriov.networkName is required")
	})

	t.Run("validate hostdev profile with missing resource name", func(t *testing.T) {
		config := &LaunchKubernetesConfig{
			NetworkOperator: &NetworkOperatorConfig{
				Version:          "v25.10.0",
				ComponentVersion: "network-operator-v25.10.0",
				Repository:       "nvcr.io/nvidia/mellanox",
				Namespace:        "nvidia-network-operator",
			},
			Hostdev: &HostdevConfig{
				ResourceName: "", // Missing
				NetworkName:  "hostdev-network",
			},
		}

		err := ValidateClusterConfig(config, "host-device-rdma")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hostdev.resourceName is required")
	})

	t.Run("validate valid sriov config", func(t *testing.T) {
		config := &LaunchKubernetesConfig{
			NetworkOperator: &NetworkOperatorConfig{
				Version:          "v25.10.0",
				ComponentVersion: "network-operator-v25.10.0",
				Repository:       "nvcr.io/nvidia/mellanox",
				Namespace:        "nvidia-network-operator",
			},
			Sriov: &SriovConfig{
				EthernetMtu:   9000,
				InfinibandMtu: 4000,
				NumVfs:        8,
				Priority:      90,
				ResourceName:  "sriov_resource",
				NetworkName:   "sriov_network",
			},
		}

		err := ValidateClusterConfig(config, "sriov-rdma")
		assert.NoError(t, err)
	})
}

func TestSriovConfig(t *testing.T) {
	t.Run("verify separate MTU fields in struct", func(t *testing.T) {
		config := &SriovConfig{
			EthernetMtu:   9000,
			InfinibandMtu: 4000,
			NumVfs:        8,
			Priority:      90,
			ResourceName:  "sriov_resource",
			NetworkName:   "sriov_network",
		}

		assert.Equal(t, 9000, config.EthernetMtu, "Ethernet MTU should be 9000")
		assert.Equal(t, 4000, config.InfinibandMtu, "Infiniband MTU should be 4000")
		assert.Equal(t, 8, config.NumVfs)
		assert.Equal(t, 90, config.Priority)
		assert.Equal(t, "sriov_resource", config.ResourceName)
		assert.Equal(t, "sriov_network", config.NetworkName)
	})

	t.Run("verify different MTU values for different fabrics", func(t *testing.T) {
		ethernetConfig := &SriovConfig{
			EthernetMtu:   9000,
			InfinibandMtu: 4000,
		}

		infinibandConfig := &SriovConfig{
			EthernetMtu:   9000,
			InfinibandMtu: 4000,
		}

		// Verify that we can access different MTU values for different use cases
		assert.NotEqual(t, ethernetConfig.EthernetMtu, infinibandConfig.InfinibandMtu,
			"Ethernet and Infiniband MTU values should be different")
		assert.Equal(t, 9000, ethernetConfig.EthernetMtu)
		assert.Equal(t, 4000, infinibandConfig.InfinibandMtu)
	})
}
