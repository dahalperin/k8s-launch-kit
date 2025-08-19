package templates

import (
	"strings"
	"testing"

	"github.com/nvidia/k8s-launch-kit/pkg/clusterconfig"
)

func TestMacvlanTemplates(t *testing.T) {
	baseConfig := clusterconfig.ClusterConfig{
		NetworkOperator: clusterconfig.NetworkOperatorConfig{
			Repository:       "nvcr.io/nvstaging/mellanox",
			ComponentVersion: "v25.7.0",
			Namespace:        "network-operator",
		},
		NvIpam: clusterconfig.NvIpamConfig{
			PoolName: "macvlan-pool",
			Subnets: []clusterconfig.NvIpamSubnetConfig{
				{Subnet: "192.168.4.0/24", Gateway: "192.168.4.1"},
				{Subnet: "192.168.5.0/24", Gateway: "192.168.5.1"},
				{Subnet: "192.168.6.0/24", Gateway: "192.168.6.1"},
			},
		},
		Sriov: clusterconfig.SriovConfig{
			Mtu: 1500,
		},
		RdmaShared: clusterconfig.RdmaSharedConfig{
			ResourceName: "rdma-shared-resource",
		},
		Macvlan: clusterconfig.MacvlanConfig{
			NetworkName: "macvlan-network",
		},
		ClusterConfig: clusterconfig.ClusterConfigStruct{
			NvidiaNICs: clusterconfig.NvidiaNICsConfig{
				PF: clusterconfig.PFConfig{
					NetworkInterfaces: []string{"ens1f0", "ens1f1", "ens2f0"},
				},
			},
		},
	}

	testCases := []struct {
		name                       string
		separateNetworkPerDevice   bool
		singleNetworkForAllDevices bool
		expectedNetworks           int
		expectedPools              int
	}{
		{
			name:                       "SeparateNetworkPerDevice",
			separateNetworkPerDevice:   true,
			singleNetworkForAllDevices: false,
			expectedNetworks:           3, // One per interface
			expectedPools:              3, // One per interface
		},
		{
			name:                       "SingleNetworkForAllDevices",
			separateNetworkPerDevice:   false,
			singleNetworkForAllDevices: true,
			expectedNetworks:           1, // Single shared
			expectedPools:              1, // Single shared
		},
		{
			name:                       "DefaultFirstDeviceOnly",
			separateNetworkPerDevice:   false,
			singleNetworkForAllDevices: false,
			expectedNetworks:           1, // First device only
			expectedPools:              1, // First device only
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Configure macvlan settings for this test case
			config := baseConfig
			config.Macvlan.SeparateNetworkPerDevice = tc.separateNetworkPerDevice
			config.Macvlan.SingleNetworkForAllDevices = tc.singleNetworkForAllDevices

			// Test IP Pool template
			poolTemplate := "../../profiles/macvlan-rdma-shared/20-ippool.yaml"
			poolRendered, err := ProcessTemplate(poolTemplate, config)
			if err != nil {
				t.Fatalf("Failed to process IP pool template: %v", err)
			}

			// Count number of IPPool resources
			poolCount := strings.Count(poolRendered, "kind: IPPool")
			if poolCount != tc.expectedPools {
				t.Errorf("Expected %d IP pools, got %d", tc.expectedPools, poolCount)
			}

			// Test MacVLAN Network template
			networkTemplate := "../../profiles/macvlan-rdma-shared/30-macvlannetwork.yaml"
			networkRendered, err := ProcessTemplate(networkTemplate, config)
			if err != nil {
				t.Fatalf("Failed to process MacVLAN network template: %v", err)
			}

			// Count number of MacvlanNetwork resources
			networkCount := strings.Count(networkRendered, "kind: MacvlanNetwork")
			if networkCount != tc.expectedNetworks {
				t.Errorf("Expected %d MacVLAN networks, got %d", tc.expectedNetworks, networkCount)
			}

			// Test Pod template
			podTemplate := "../../profiles/macvlan-rdma-shared/40-pod.yaml"
			podRendered, err := ProcessTemplate(podTemplate, config)
			if err != nil {
				t.Fatalf("Failed to process pod template: %v", err)
			}

			// Verify no unreplaced templates
			if strings.Contains(poolRendered, "{{.") {
				t.Errorf("IP pool template contains unreplaced variables")
			}
			if strings.Contains(networkRendered, "{{.") {
				t.Errorf("Network template contains unreplaced variables")
			}
			if strings.Contains(podRendered, "{{.") {
				t.Errorf("Pod template contains unreplaced variables")
			}

			t.Logf("Test case %s passed: %d pools, %d networks", tc.name, poolCount, networkCount)
		})
	}
}
