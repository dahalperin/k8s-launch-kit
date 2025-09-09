package networkoperatorplugin

import (
	"context"
	"fmt"
	"time"

	netop "github.com/Mellanox/network-operator/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// EnsureNicClusterPolicy creates the provided NicClusterPolicy if none exist and waits until it's ready.
func EnsureNicClusterPolicy(ctx context.Context, c client.Client, policy *netop.NicClusterPolicy) error {
	// Ensure no NicClusterPolicy exists yet
	list := &netop.NicClusterPolicyList{}
	if err := c.List(ctx, list); err != nil {
		return err
	}
	if len(list.Items) > 0 {
		return fmt.Errorf("NicClusterPolicy already exists (count=%d)", len(list.Items))
	}

	if err := c.Create(ctx, policy); err != nil {
		return err
	}

	return WaitNicClusterPolicyReady(ctx, c, policy.Name)
}

// WaitNicClusterPolicyReady polls NicClusterPolicy until Status.State is ready or error, with a timeout.
func WaitNicClusterPolicyReady(parentCtx context.Context, c client.Client, name string) error {
	// Use a bounded timeout if none supplied
	ctx := parentCtx
	if _, hasDeadline := parentCtx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		// Try to get by name (cluster-scoped)
		policy := &netop.NicClusterPolicy{}
		if err := c.Get(ctx, client.ObjectKey{Name: name}, policy); err == nil {
			switch policy.Status.State {
			case netop.StateReady:
				log.Log.Info("NicClusterPolicy is ready")
				return nil
			case netop.StateError:
				return fmt.Errorf("NicClusterPolicy in error state: %s", policy.Status.Reason)
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for NicClusterPolicy %q to become ready", name)
		case <-ticker.C:
			// continue
		}
	}
}

// DeleteNicClusterPolicy deletes the NicClusterPolicy by name, ignoring NotFound errors.
func DeleteNicClusterPolicy(ctx context.Context, c client.Client, name string) error {
	obj := &netop.NicClusterPolicy{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if err := c.Delete(ctx, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}
