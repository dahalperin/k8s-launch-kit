package discovery

import (
	"context"
	"fmt"
	"sort"
	"time"

	netop "github.com/Mellanox/network-operator/api/v1alpha1"
	nicop "github.com/Mellanox/nic-configuration-operator/api/v1alpha1"
	"github.com/nvidia/k8s-launch-kit/pkg/config"
	"github.com/nvidia/k8s-launch-kit/pkg/netophelper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func DiscoverClusterConfig(ctx context.Context, c client.Client, defaultConfig *config.NetworkOperatorConfig) (config.ClusterConfig, error) {
	// Ensure a NicClusterPolicy exists (error if any already exists, else create one)
	policy := &netop.NicClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nic-cluster-policy",
			Namespace: defaultConfig.Namespace,
		},
		Spec: netop.NicClusterPolicySpec{
			NicConfigurationOperator: &netop.NicConfigurationOperatorSpec{
				Operator: &netop.ImageSpec{
					Repository: defaultConfig.Repository,
					Image:      "nic-configuration-operator",
					Version:    defaultConfig.ComponentVersion,
				},
				ConfigurationDaemon: &netop.ImageSpec{
					Repository: defaultConfig.Repository,
					Image:      "nic-configuration-operator-daemon",
					Version:    defaultConfig.ComponentVersion,
				},
			},
		},
	}

	log.Log.Info("Deploying a thin NicClusterPolicy for cluster config discovery")

	if err := netophelper.EnsureNicClusterPolicy(ctx, c, policy); err != nil {
		return config.ClusterConfig{}, err
	}

	// Always attempt cleanup of the NicClusterPolicy at the end of discovery
	defer func() {
		if err := netophelper.DeleteNicClusterPolicy(ctx, c, "nic-cluster-policy"); err != nil {
			log.Log.Error(err, "failed to delete NicClusterPolicy after discovery")
		} else {
			log.Log.Info("NicClusterPolicy deleted after discovery")
		}
	}()

	// After creation, list pods in the target namespace and ensure all pods
	// from the nic-configuration-daemon DaemonSet are Ready
	if err := checkDaemonSetPodsReady(ctx, c, defaultConfig.Namespace, "nic-configuration-daemon"); err != nil {
		return config.ClusterConfig{}, err
	}

	// Get NicDevice resources and build ClusterConfig.NvidiaNICs from their statuses
	devices := &nicop.NicDeviceList{}
	if err := c.List(ctx, devices, client.InNamespace(defaultConfig.Namespace)); err != nil {
		return config.ClusterConfig{}, err
	}
	if len(devices.Items) == 0 {
		log.Log.Info("No NicDevice resources found yet; waiting for discovery", "namespace", defaultConfig.Namespace)
		if err := waitNicDevicesDiscovered(ctx, c, defaultConfig.Namespace); err != nil {
			return config.ClusterConfig{}, err
		}
		// re-list after wait
		if err := c.List(ctx, devices, client.InNamespace(defaultConfig.Namespace)); err != nil {
			return config.ClusterConfig{}, err
		}
		log.Log.Info("NicDevice resources discovered", "count", len(devices.Items))
	}
	built := buildClusterConfigFromNicDevices(devices.Items)

	return built, nil
}

// checkDaemonSetPodsReady verifies that all pods owned by the given DaemonSet
// in the provided namespace are Ready.
func checkDaemonSetPodsReady(ctx context.Context, c client.Client, namespace, daemonSetName string) error {
	podList := &corev1.PodList{}
	if err := c.List(ctx, podList, client.InNamespace(namespace)); err != nil {
		return err
	}

	var dsPods []corev1.Pod
	for _, pod := range podList.Items {
		for _, owner := range pod.OwnerReferences {
			if owner.Kind == "DaemonSet" && owner.Name == daemonSetName {
				dsPods = append(dsPods, pod)
				break
			}
		}
	}

	if len(dsPods) == 0 {
		return fmt.Errorf("no pods found for DaemonSet %q in namespace %q", daemonSetName, namespace)
	}

	for _, pod := range dsPods {
		if !isPodReady(&pod) {
			return fmt.Errorf("pod %q from DaemonSet %q is not Ready", pod.Name, daemonSetName)
		}
	}

	return nil
}

func isPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// waitNicDevicesDiscovered polls until one or more NicDevice objects exist in the given namespace.
func waitNicDevicesDiscovered(parentCtx context.Context, c client.Client, namespace string) error {
	// Use a bounded timeout if none supplied
	ctx := parentCtx
	if _, hasDeadline := parentCtx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(parentCtx, 5*time.Minute)
		defer cancel()
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		list := &nicop.NicDeviceList{}
		if err := c.List(ctx, list, client.InNamespace(namespace)); err == nil {
			if len(list.Items) > 0 {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for NicDevice resources in namespace %q", namespace)
		case <-ticker.C:
			// continue polling
		}
	}
}

// buildClusterConfigFromNicDevices constructs ClusterConfig.NvidiaNICs based on NicDevice statuses.
func buildClusterConfigFromNicDevices(devices []nicop.NicDevice) config.ClusterConfig {
	rdmaSet := map[string]struct{}{}
	pciSet := map[string]struct{}{}
	netIfSet := map[string]struct{}{}

	for _, d := range devices {
		for _, p := range d.Status.Ports {
			if p.RdmaInterface != "" {
				rdmaSet[p.RdmaInterface] = struct{}{}
			}
			if p.PCI != "" {
				pciSet[p.PCI] = struct{}{}
			}
			if p.NetworkInterface != "" {
				netIfSet[p.NetworkInterface] = struct{}{}
			}
		}
	}

	toSortedSlice := func(m map[string]struct{}) []string {
		out := make([]string, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		// stable order for determinism
		sort.Strings(out)
		return out
	}

	cluster := config.ClusterConfig{}
	cluster.NvidiaNICs.PF.RdmaDevices = toSortedSlice(rdmaSet)
	cluster.NvidiaNICs.PF.PciAddresses = toSortedSlice(pciSet)
	cluster.NvidiaNICs.PF.NetworkInterfaces = toSortedSlice(netIfSet)

	if len(rdmaSet) > 0 {
		cluster.Nodes.Capabilities.Rdma = true
	}

	return cluster
}
