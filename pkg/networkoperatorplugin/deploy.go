package networkoperatorplugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nvidia/k8s-launch-kit/pkg/profiles"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	yaml "sigs.k8s.io/yaml"
)

// Apply reads Kubernetes manifests from dirPath and applies them to the cluster.
// If a NicClusterPolicy is present, it is applied first and the function waits
// for it to become ready before applying the remaining manifests.
func (p *NetworkOperatorPlugin) DeployProfile(ctx context.Context, profile *profiles.Profile, kubeClient client.Client, manifestsDir string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// List files in directory (non-recursive) and sort
	entries, err := os.ReadDir(manifestsDir)
	if err != nil {
		return err
	}
	filePaths := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		filePaths = append(filePaths, filepath.Join(manifestsDir, e.Name()))
	}
	sort.Strings(filePaths)

	// Collect manifests from all files (support multi-doc YAML using '---')
	var nicDoc []byte
	var otherDocs [][]byte
	for _, p := range filePaths {
		content, rErr := os.ReadFile(p)
		if rErr != nil {
			return rErr
		}
		docs := splitYAMLDocuments(string(content))
		for _, doc := range docs {
			if len(strings.TrimSpace(doc)) == 0 {
				continue
			}
			b := []byte(doc)
			if containsNicClusterPolicyKind(b) {
				if len(nicDoc) != 0 {
					return fmt.Errorf("multiple NicClusterPolicy manifests found; only one is allowed")
				}
				nicDoc = b
			} else {
				otherDocs = append(otherDocs, b)
			}
		}
	}

	// Apply NicClusterPolicy first if present
	if len(nicDoc) != 0 {
		log.Log.Info("Applying NicClusterPolicy for selected profile")
		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal(nicDoc, obj); err != nil {
			return fmt.Errorf("failed to decode NicClusterPolicy: %w", err)
		}
		// Ensure GVK set for server-side apply
		apiv, kind := obj.GetAPIVersion(), obj.GetKind()
		if apiv != "" && kind != "" {
			gv, err := schema.ParseGroupVersion(apiv)
			if err == nil {
				obj.SetGroupVersionKind(gv.WithKind(kind))
			}
		}
		if err := applyUnstructured(ctx, kubeClient, obj); err != nil {
			return err
		}

		log.Log.Info("Waiting for NicClusterPolicy to be ready")
		if err := WaitNicClusterPolicyReady(ctx, kubeClient, obj.GetName()); err != nil {
			return err
		}
	}

	// Apply remaining manifests
	log.Log.Info("Applying remaining profile manifests", "count", len(otherDocs))
	for _, b := range otherDocs {
		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal(b, obj); err != nil {
			return fmt.Errorf("failed to decode manifest: %w", err)
		}
		// Ensure GVK set for server-side apply
		apiv, kind := obj.GetAPIVersion(), obj.GetKind()
		if apiv != "" && kind != "" {
			gv, err := schema.ParseGroupVersion(apiv)
			if err == nil {
				obj.SetGroupVersionKind(gv.WithKind(kind))
			}
		}
		log.Log.Info("Applying object", "kind", obj.GetKind(), "name", obj.GetName(), "version", obj.GetAPIVersion())

		// Apply with retry for Pod kind
		applyErr := applyUnstructured(ctx, kubeClient, obj)
		if applyErr != nil && strings.EqualFold(obj.GetKind(), "Pod") {
			const maxAttempts = 3
			for attempt := 2; attempt <= maxAttempts && applyErr != nil; attempt++ {
				log.Log.Info("Pod apply failed, retrying", "name", obj.GetName(), "attempt", attempt, "delay", "30s", "error", applyErr.Error())
				time.Sleep(30 * time.Second)
				applyErr = applyUnstructured(ctx, kubeClient, obj)
			}
		}
		if applyErr != nil {
			return applyErr
		}
	}

	return nil
}

func containsNicClusterPolicyKind(b []byte) bool {
	// Very small YAML sniffing: unmarshal just Kind
	type metaOnly struct {
		Kind string `yaml:"kind"`
	}
	var mo metaOnly
	if err := yaml.Unmarshal(b, &mo); err != nil {
		return false
	}
	return mo.Kind == "NicClusterPolicy"
}

func applyUnstructured(ctx context.Context, c client.Client, obj *unstructured.Unstructured) error {
	// kubectl-style server-side apply
	return c.Patch(ctx, obj, client.Apply, client.FieldOwner("l8k"), client.ForceOwnership)
}

// splitYAMLDocuments splits a YAML stream by lines that start with '---' (doc separators)
func splitYAMLDocuments(s string) []string {
	var docs []string
	var cur []string
	lines := strings.Split(s, "\n")
	for _, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "---") {
			if len(cur) > 0 {
				docs = append(docs, strings.Join(cur, "\n"))
				cur = nil
			}
			continue
		}
		cur = append(cur, ln)
	}
	if len(cur) > 0 {
		docs = append(docs, strings.Join(cur, "\n"))
	}
	return docs
}
