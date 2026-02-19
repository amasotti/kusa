package kube

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

// Clients holds the core and metrics Kubernetes clientsets and the resolved context name.
type Clients struct {
	Core        *kubernetes.Clientset
	Metrics     *metricsclient.Clientset
	ContextName string
}

// NewClients builds Kubernetes clients from the given kubeconfig path and optional context override.
func NewClients(kubeconfig, contextOverride string) (*Clients, error) {
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	configOverrides := &clientcmd.ConfigOverrides{}

	// Use specific context if provided, otherwise rely on the kubeconfig's current context
	if contextOverride != "" {
		configOverrides.CurrentContext = contextOverride
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build REST config: %w", err)
	}

	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load raw kubeconfig: %w", err)
	}
	contextName := rawConfig.CurrentContext

	coreClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	metricsClient, err := metricsclient.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics client: %w", err)
	}

	return &Clients{
		Core:        coreClient,
		Metrics:     metricsClient,
		ContextName: contextName,
	}, nil
}
