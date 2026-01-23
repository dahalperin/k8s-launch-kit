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

package networkoperatorplugin

import (
	"context"
	"fmt"
	"time"

	netop "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/nvidia/k8s-launch-kit/pkg/ui"
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
	uiOutput := ui.FromContext(parentCtx)
	progress := uiOutput.StartProgress("Waiting for NIC Cluster Policy to become ready")

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
				progress.Success("NIC Cluster Policy is ready")
				log.Log.Info("NicClusterPolicy is ready")
				return nil
			case netop.StateError:
				progress.Fail(fmt.Sprintf("Policy error: %s", policy.Status.Reason))
				return fmt.Errorf("NicClusterPolicy in error state: %s", policy.Status.Reason)
			default:
				if policy.Status.State != "" {
					progress.Update(fmt.Sprintf("Current state: %s", policy.Status.State))
				}
			}
		}

		select {
		case <-ctx.Done():
			progress.Fail("Timeout waiting for policy")
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
