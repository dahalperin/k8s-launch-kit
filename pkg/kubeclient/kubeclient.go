package kubeclient

import (
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	netop "github.com/Mellanox/network-operator/api/v1alpha1"
	nicop "github.com/Mellanox/nic-configuration-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// New builds a controller-runtime client using the provided kubeconfig path
// and registers required schemes.
func New(kubeconfigPath string) (client.Client, error) {
	// Build REST config from kubeconfig path
	restCfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	// Prepare scheme and client
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = netop.AddToScheme(scheme)
	_ = nicop.AddToScheme(scheme)

	return client.New(restCfg, client.Options{Scheme: scheme})
}
