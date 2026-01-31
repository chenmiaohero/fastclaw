package k8s

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	clientset  *kubernetes.Clientset
	restConfig *rest.Config
)

func InitClient() error {
	kubeconfig := viper.GetString("kubernetes.kubeconfig")

	var config *rest.Config
	var err error

	if kubeconfig != "" {
		// Use specified kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		// Running inside cluster
		config, err = rest.InClusterConfig()
	} else {
		// Try default kubeconfig location
		home, _ := os.UserHomeDir()
		kubeconfig = filepath.Join(home, ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	if err != nil {
		return fmt.Errorf("failed to build k8s config: %w", err)
	}

	restConfig = config

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	return nil
}

func GetRestConfig() *rest.Config {
	return restConfig
}

func GetClient() *kubernetes.Clientset {
	return clientset
}

func GetNamespace() string {
	ns := viper.GetString("kubernetes.namespace")
	if ns == "" {
		ns = "openclaw"
	}
	return ns
}
