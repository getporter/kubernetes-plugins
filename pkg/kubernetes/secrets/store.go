package secrets

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	k8shelper "get.porter.sh/plugin/kubernetes/pkg/kubernetes/helper"
	"get.porter.sh/porter/pkg/portercontext"
	portersecrets "get.porter.sh/porter/pkg/secrets/plugins"
	"get.porter.sh/porter/pkg/tracing"
	cnabsecrets "github.com/cnabio/cnab-go/secrets"
	cnabhost "github.com/cnabio/cnab-go/secrets/host"
	"github.com/hashicorp/go-hclog"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var _ portersecrets.SecretsProtocol = &Store{}

const (
	SecretSourceType = "secret"
	SecretDataKey    = "value"
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

func (s *Store) Resolve(ctx context.Context, keyName string, keyValue string) (string, error) {
	ctx, log := tracing.StartSpan(ctx)
	defer log.EndSpan()

	if err := s.connect(); err != nil {
		return "", err
	}
	if strings.ToLower(keyName) != SecretSourceType {
		return s.hostStore.Resolve(keyName, keyValue)
	}
	s.logger.Debug(fmt.Sprintf("Store.Resolve: ns:%s, keyName:%s, keyValue:%s", s.namespace, keyName, keyValue))
	key := SanitizeKey(keyValue)

	secret, err := s.clientSet.CoreV1().Secrets(s.namespace).Get(ctx, key, metav1.GetOptions{})
	if err != nil {
		return "", log.Error(fmt.Errorf("could not get secret %s: %w ", keyValue, err))
	}
	if val, ok := secret.Data[SecretDataKey]; !ok {
		return "", log.Error(InvalidSecretDataKeyError{msg: fmt.Sprintf(`The secret %s/%s does not have a key named %s. `+
			`The kubernetes.secrets plugin requires that the Kubernetes secret is named after the secret referenced in the `+
			`Porter parameter or credential set, and secret value is stored in a key on the Kubernetes secret named %s`,
			s.namespace, keyValue, SecretDataKey, SecretDataKey)})
	} else {
		return string(val), nil
	}
}

func (s *Store) Create(ctx context.Context, keyName string, keyValue string, value string) error {
	ctx, log := tracing.StartSpan(ctx)
	defer log.EndSpan()

	if err := s.connect(); err != nil {
		return err
	}

	s.logger.Debug(fmt.Sprintf("Store.Create: ns:%s, keyName:%s, keyValue:%s", s.namespace, keyName, keyValue))

	key := strings.ToLower(keyName)
	if key != SecretSourceType {
		return log.Error(fmt.Errorf("unsupported secret type: %s. Only %s is supported", keyName, SecretSourceType))
	}

	byteValue := []byte(value)
	if len(byteValue) > v1.MaxSecretSize {
		return log.Error(fmt.Errorf("secret: %s exceeded the maximum secret size", key))
	}

	Immutable := true
	data := map[string][]byte{
		SecretDataKey: byteValue,
	}
	secret := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: SanitizeKey(keyValue)}, Immutable: &Immutable, Data: data}
	_, err := s.clientSet.CoreV1().Secrets(s.namespace).Create(ctx, secret, metav1.CreateOptions{})
	return log.Error(err)
}

// SanitizeKey converts a string to follow below rules:
// 1. only contains lower case alphanumeric characters, '-' or '.'
// 2. must start and end with an alphanumeric character
func SanitizeKey(v string) string {
	key := strings.ToLower(v)
	// replace non-alphanumeric characters at the beginning and the end of the string
	startEndReg := regexp.MustCompile(`^[^a-z0-9]|[^a-z0-9]$`)
	firstPass := startEndReg.ReplaceAllString(key, "000")
	// replace non-alphanumeric characters except `-` and `.` in the string
	characterReg := regexp.MustCompile(`[^a-z0-9-.]+`)
	return characterReg.ReplaceAllString(firstPass, "-")
}
