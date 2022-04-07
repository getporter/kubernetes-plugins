package secrets

import (
	"context"
	"fmt"
	"strings"

	k8shelper "get.porter.sh/plugin/kubernetes/pkg/kubernetes/helper"
	portercontext "get.porter.sh/porter/pkg/context"
	portersecrets "get.porter.sh/porter/pkg/secrets/plugins"
	cnabsecrets "github.com/cnabio/cnab-go/secrets"
	cnabhost "github.com/cnabio/cnab-go/secrets/host"
	"github.com/hashicorp/go-hclog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var _ portersecrets.SecretsProtocol = &Store{}

const (
	SecretSourceType = "secret"
	SecretDataKey    = "credential"
)

// Store implements the backing store for secrets as kubernetes secrets.
type Store struct {
	*portercontext.Context
	hostStore cnabsecrets.Store
	Secrets   map[string]map[string]string
	namespace string
	clientSet *kubernetes.Clientset
	logger    hclog.Logger
}

func NewStore(c *portercontext.Context, cfg PluginConfig) *Store {
	namespace := cfg.Namespace
	s := &Store{
		Secrets:   make(map[string]map[string]string),
		hostStore: &cnabhost.SecretStore{},
		namespace: namespace,
		logger:    cfg.Logger,
	}
	return s
}

func (s *Store) connect() error {

	if s.clientSet != nil {
		return nil
	}
	s.logger.Info(fmt.Sprintf("Store.connect: pre-clientset %s : %s", "namespace", s.namespace))
	clientSet, namespace, err := k8shelper.GetClientSet(s.namespace)
	if err != nil {
		return err
	}
	s.namespace = *namespace
	s.logger.Info(fmt.Sprintf("Store.connect: post-clientset %s : %s", "namespace", s.namespace))

	s.clientSet = clientSet

	return nil
}

func (s *Store) Resolve(keyName string, keyValue string) (string, error) {
	if err := s.connect(); err != nil {
		return "", err
	}
	if strings.ToLower(keyName) != SecretSourceType {
		return s.hostStore.Resolve(keyName, keyValue)
	}
	s.logger.Debug(fmt.Sprintf("Store.Resolve: ns:%s, keyName:%s, keyValue:%s", s.namespace, keyName, keyValue))
	key := strings.ToLower(keyValue)

	secret, err := s.clientSet.CoreV1().Secrets(s.namespace).Get(context.Background(), key, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if val, ok := secret.Data[SecretDataKey]; !ok {
		return "", InvalidSecretDataKeyError{msg: fmt.Sprintf("Key \"%s\" not found in secret", SecretDataKey)}
	} else {
		return string(val), nil
	}
}
