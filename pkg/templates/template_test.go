package templates

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/nvidia/k8s-launch-kit/pkg/clusterconfig"
)

func TestHostDeviceRdmaTemplate(t *testing.T) {
	// Define test config matching l8k-config.yaml structure
	config := clusterconfig.ClusterConfig{
		NetworkOperator: clusterconfig.NetworkOperatorConfig{
			Version:          "v25.7.0",
			ComponentVersion: "network-operator-v25.7.0",
			Repository:       "nvcr.io/nvstaging/mellanox",
			Namespace:        "network-operator",
		},
		NvIpam: clusterconfig.NvIpamConfig{
			PoolName: "nv-ipam-pool",
			Subnets: []clusterconfig.NvIpamSubnetConfig{
				{Subnet: "192.168.2.0/24", Gateway: "192.168.2.1"},
				{Subnet: "192.168.3.0/24", Gateway: "192.168.3.1"},
				{Subnet: "192.168.4.0/24", Gateway: "192.168.4.1"},
			},
		},
		Hostdev: clusterconfig.HostdevConfig{
			ResourceName: "hostdev-resource",
			NetworkName:  "hostdev-network",
		},
		ClusterConfig: clusterconfig.ClusterConfigStruct{
			NvidiaNICs: clusterconfig.NvidiaNICsConfig{
				PF: clusterconfig.PFConfig{
					NetworkInterfaces: []string{"ibs1f0", "ibs1f1", "ibs2f0"},
				},
			},
		},
	}

	// Read the template file
	templatePath := filepath.Join("..", "..", "profiles", "host-device-rdma", "10-nicclusterpolicy.yaml")
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("Failed to read template file: %v", err)
	}

	// Parse and execute the template with helper functions
	tmpl, err := template.New("nicclusterpolicy").Funcs(templateFuncs).Parse(string(templateContent))
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, config)
	if err != nil {
		t.Fatalf("Failed to execute template: %v", err)
	}

	// Verify that template variables were replaced
	result := buf.String()

	// Check that no template variables remain unreplaced
	if bytes.Contains([]byte(result), []byte("{{.")) {
		t.Errorf("Template contains unreplaced variables")
	}

	// Check that our test values are present
	if !bytes.Contains([]byte(result), []byte(config.NetworkOperator.Repository)) {
		t.Errorf("Network Operator repository not found in rendered template")
	}

	if !bytes.Contains([]byte(result), []byte(config.Hostdev.ResourceName)) {
		t.Errorf("Hostdev resource name not found in rendered template")
	}

	t.Logf("Template rendered successfully:\n%s", result)
}
