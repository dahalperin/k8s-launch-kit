# Host Device RDMA Profile Template Variables

This profile uses Go templates based on the l8k-config.yaml structure. The following variables are available for templating:

## Required Template Variables

### Network Operator Configuration
- `{{.NetworkOperator.Repository}}` - Repository for network operator components
- `{{.NetworkOperator.ComponentVersion}}` - Version for network operator components
- `{{.NetworkOperator.Version}}` - Network operator version
- `{{.NetworkOperator.Namespace}}` - Namespace for network operator

### NVIDIA IPAM Configuration  
- `{{.NvIpam.PoolName}}` - Name of the IP pool
- `{{.NvIpam.Subnet}}` - Subnet for IP allocation
- `{{.NvIpam.Gateway}}` - Gateway IP address
- `{{.NvIpam.SubnetOffset}}` - Subnet offset for multiple pools

### Host Device Configuration
- `{{.Hostdev.ResourceName}}` - Resource name for host devices
- `{{.Hostdev.NetworkName}}` - Network name for host device network

## Example Usage

```yaml
repository: {{.NetworkOperator.Repository}}
version: {{.NetworkOperator.ComponentVersion}}
resourceName: {{.Hostdev.ResourceName}}
poolName: {{.NvIpam.PoolName}}
```

## Template Processing

These templates should be processed using Go's `text/template` package with a Config struct that matches the l8k-config.yaml structure before applying to Kubernetes.
