package secrets

import (
	"context"
	"fmt"
	"strings"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/config"
	k8s "get.porter.sh/plugin/kubernetes/pkg/kubernetes/helper"
	portersecrets "get.porter.sh/porter/pkg/secrets"
	cnabsecrets "github.com/cnabio/cnab-go/secrets"
	"github.com/cnabio/cnab-go/secrets/host"
	"github.com/hashicorp/go-hclog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

	clientSet, namespace, err := k8s.GetClientSet(s.config.Namespace, s.logger)

	if err != nil {
		s.logger.Debug(fmt.Sprintf("Failed to get Kubernetes Client Set: %v", err))
		return err
	}

	s.clientSet = clientSet
	s.config.Namespace = *namespace

	return nil
}

func (s *Store) Resolve(keyName string, keyValue string) (string, error) {
	if strings.ToLower(keyName) != SecretSourceType {
		return s.hostStore.Resolve(keyName, keyValue)
	}

	key := strings.ToLower(keyValue)

	s.logger.Debug(fmt.Sprintf("Looking for key:%s", keyValue))
	secret, err := s.clientSet.CoreV1().Secrets(s.config.Namespace).Get(context.Background(), key, metav1.GetOptions{})
	if err != nil {
		s.logger.Debug(fmt.Sprintf("Failed to Read secrets for key:%s %v", keyValue, err))
		return "", err
	}

	return string(secret.Data[SecretDataKey]), nil
}
