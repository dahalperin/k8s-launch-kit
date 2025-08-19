package templates

import (
	"strings"
	"testing"

	"github.com/nvidia/k8s-launch-kit/pkg/clusterconfig"
)

func TestIpoibTemplates(t *testing.T) {
	baseConfig := clusterconfig.ClusterConfig{
		NetworkOperator: clusterconfig.NetworkOperatorConfig{
			Repository:       "nvcr.io/nvstaging/mellanox",
			ComponentVersion: "v25.7.0",
			Namespace:        "network-operator",
		},
		NvIpam: clusterconfig.NvIpamConfig{
			PoolName: "ipoib-pool",
			Subnets: []clusterconfig.NvIpamSubnetConfig{
				{Subnet: "192.168.5.0/24", Gateway: "192.168.5.1"},
				{Subnet: "192.168.6.0/24", Gateway: "192.168.6.1"},
				{Subnet: "192.168.7.0/24", Gateway: "192.168.7.1"},
			},
		},
		RdmaShared: clusterconfig.RdmaSharedConfig{
			ResourceName: "rdma-shared-resource",
			HcaMax:       63,
		},
		Ipoib: clusterconfig.IpoibConfig{
			NetworkName: "ipoib-network",
		},
		ClusterConfig: clusterconfig.ClusterConfigStruct{
			NvidiaNICs: clusterconfig.NvidiaNICsConfig{
				PF: clusterconfig.PFConfig{
					NetworkInterfaces: []string{"ibs1f0", "ibs1f1", "ibs2f0"},
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
		expectedRdmaResources      int
	}{
		{
			name:                       "SeparateNetworkPerDevice",
			separateNetworkPerDevice:   true,
			singleNetworkForAllDevices: false,
			expectedNetworks:           3, // One per interface
			expectedPools:              3, // One per interface
			expectedRdmaResources:      3, // One per interface
		},
		{
			name:                       "SingleNetworkForAllDevices",
			separateNetworkPerDevice:   false,
			singleNetworkForAllDevices: true,
			expectedNetworks:           1, // Single shared
			expectedPools:              1, // Single shared
			expectedRdmaResources:      1, // Single with all interfaces
		},
		{
			name:                       "DefaultFirstDeviceOnly",
			separateNetworkPerDevice:   false,
			singleNetworkForAllDevices: false,
			expectedNetworks:           1, // First device only
			expectedPools:              1, // First device only
			expectedRdmaResources:      1, // First device only
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Configure ipoib settings for this test case
			config := baseConfig
			config.Ipoib.SeparateNetworkPerDevice = tc.separateNetworkPerDevice
			config.Ipoib.SingleNetworkForAllDevices = tc.singleNetworkForAllDevices

			// Test NicClusterPolicy template
			nicTemplate := "../../profiles/ipoib-rdma-shared/10-nicclusterpolicy.yaml"
			nicRendered, err := ProcessTemplate(nicTemplate, config)
			if err != nil {
				t.Fatalf("Failed to process NicClusterPolicy template: %v", err)
			}

			// Count RDMA resources in configList
			rdmaResourceCount := strings.Count(nicRendered, "\"resourceName\":")
			if rdmaResourceCount != tc.expectedRdmaResources {
				t.Errorf("Expected %d RDMA resources, got %d", tc.expectedRdmaResources, rdmaResourceCount)
			}

			// Test IP Pool template
			poolTemplate := "../../profiles/ipoib-rdma-shared/20-ippool.yaml"
			poolRendered, err := ProcessTemplate(poolTemplate, config)
			if err != nil {
				t.Fatalf("Failed to process IP pool template: %v", err)
			}

			// Count number of IPPool resources
			poolCount := strings.Count(poolRendered, "kind: IPPool")
			if poolCount != tc.expectedPools {
				t.Errorf("Expected %d IP pools, got %d", tc.expectedPools, poolCount)
			}

			// Test IPoIB Network template
			networkTemplate := "../../profiles/ipoib-rdma-shared/30-ipoibnetwork.yaml"
			networkRendered, err := ProcessTemplate(networkTemplate, config)
			if err != nil {
				t.Fatalf("Failed to process IPoIB network template: %v", err)
			}

			// Count number of IPoIBNetwork resources
			networkCount := strings.Count(networkRendered, "kind: IPoIBNetwork")
			if networkCount != tc.expectedNetworks {
				t.Errorf("Expected %d IPoIB networks, got %d", tc.expectedNetworks, networkCount)
			}

			// Verify no unreplaced templates
			templates := []string{nicRendered, poolRendered, networkRendered}
			for i, tmpl := range templates {
				if strings.Contains(tmpl, "{{.") {
					t.Errorf("Template %d contains unreplaced variables", i)
				}
			}

			t.Logf("Test case %s passed: %d RDMA resources, %d pools, %d networks",
				tc.name, rdmaResourceCount, poolCount, networkCount)
		})
	}
}
