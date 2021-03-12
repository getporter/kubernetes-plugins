package helper

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func GetClientSet(namespace string, logger hclog.Logger) (*kubernetes.Clientset, *string, error) {
	if namespace == "" {
		// Try to get the namespace of current pod
		if ns, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err != nil {
			logger.Error("Failed to lookup Kubernetes namespace", "error", err)
			return nil, nil, err
		} else {
			namespace = string(ns)
		}
	}

	var err error
	var config *restclient.Config
	var kubeconfigfile string

	if kubeconfigfile = os.Getenv("KUBECONFIG"); kubeconfigfile == "" {
		kubeconfigfile = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	if _, err = os.Stat(kubeconfigfile); err != nil {
		if os.IsNotExist(err) {
			// If the kubeconfig file does not exist then try in cluster config
			logger.Error("Kubernetes client config file does not exist", "file", kubeconfigfile)
			config, err = clientcmd.BuildConfigFromFlags("", "")
		} else {
			logger.Error("Failed to stat Kubernetes client config file", "file", kubeconfigfile, "error", err)
			return nil, nil, err
		}
	} else {
		logger.Info("Using Kubeconfig", "file", kubeconfigfile)
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigfile)
	}

	if err != nil {
		logger.Error("Failed to get Kubernetes client config", "error", err)
		return nil, nil, err
	}

	clientSet, err := kubernetes.NewForConfig(config)

	if err != nil {
		logger.Error("Failed to get Kubernetes clientset", "error", err)
		return nil, nil, err
	}

	if _, err := clientSet.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{}); err != nil {
		logger.Error("Failed to validate Kubernetes namespace", "error", err)
		return nil, nil, err
	}

	return clientSet, &namespace, nil
}
