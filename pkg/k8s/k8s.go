package k8s

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var client *kubernetes.Clientset

func CreateClient() (*kubernetes.Clientset, error) {
	// singleton
	if client != nil {
		return client, nil
	}

	config, err := createConfig()
	if err != nil {
		return nil, err
	}

	client, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func createConfig() (*rest.Config, error) {
	configFile := filepath.Join("/etc/cni/net.d/multus.d", "multus.kubeconfig")
	_, err := os.Stat(configFile)
	if err != nil {
		return nil, err
	}
	return clientcmd.BuildConfigFromFlags("", configFile)
}
