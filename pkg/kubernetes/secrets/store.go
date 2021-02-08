package secrets

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/config"
	portersecrets "get.porter.sh/porter/pkg/secrets"
	cnabsecrets "github.com/cnabio/cnab-go/secrets"
	"github.com/cnabio/cnab-go/secrets/host"
	"github.com/hashicorp/go-hclog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var _ cnabsecrets.Store = &Store{}

const (
	SecretSourceType = "secret"
	SecretDataKey    = "credential"
)

// Store implements the backing store for secrets as kubernetes secrets.
type Store struct {
	logger    hclog.Logger
	config    config.Config
	hostStore cnabsecrets.Store
	clientSet *kubernetes.Clientset
}

func NewStore(cfg config.Config, l hclog.Logger) cnabsecrets.Store {
	s := &Store{
		config:    cfg,
		logger:    l,
		hostStore: &host.SecretStore{},
	}

	return portersecrets.NewSecretStore(s)
}

func (s *Store) Connect() error {

	if s.clientSet != nil {
		return nil
	}

	if s.config.Namespace == "" {
		// Try to get the namespace of current pod
		if ns, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err != nil {
			s.logger.Error("Failed to lookup Kubernetes namespace", "error", err)
			return err
		} else {
			s.config.Namespace = string(ns)
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
			s.logger.Error("Kubernetes client config file does not exist", "file", kubeconfigfile)
			config, err = clientcmd.BuildConfigFromFlags("", "")
		} else {
			s.logger.Error("Failed to stat Kubernetes client config file", "file", kubeconfigfile, "error", err)
			return err
		}
	} else {
		s.logger.Info("Using Kubeconfig", "file", kubeconfigfile)
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigfile)
	}

	if err != nil {
		s.logger.Error("Failed to get Kubernetes client config", "error", err)
		return err
	}

	s.clientSet, err = kubernetes.NewForConfig(config)

	if err != nil {
		s.logger.Error("Failed to get Kubernetes clientset", "error", err)
		return err
	}

	if _, err := s.clientSet.CoreV1().Namespaces().Get(s.config.Namespace, metav1.GetOptions{}); err != nil {
		s.logger.Error("Failed to validate Kubernetes namespace", "error", err)
		return err
	}

	return nil
}

func (s *Store) Resolve(keyName string, keyValue string) (string, error) {
	if strings.ToLower(keyName) != SecretSourceType {
		return s.hostStore.Resolve(keyName, keyValue)
	}

	key := strings.ToLower(keyValue)

	secret, err := s.clientSet.CoreV1().Secrets(s.config.Namespace).Get(key, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return string(secret.Data[SecretDataKey]), nil
}
