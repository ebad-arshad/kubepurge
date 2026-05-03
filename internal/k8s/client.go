package k8s

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
)

// GetClient initializes the standard Kubernetes clientset
func GetClient() (*kubernetes.Clientset, error) {
	// 1. Locate the kubeconfig file (usually at ~/.kube/config)
	home, _ := os.UserHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")

	// 2. Build the configuration from the file
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	// 3. Create and return the clientset
	return kubernetes.NewForConfig(config)
}