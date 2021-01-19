package secrets

import (

	"strings"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/config"
	portersecrets "get.porter.sh/porter/pkg/secrets"
	cnabsecrets "github.com/cnabio/cnab-go/secrets"
	"github.com/cnabio/cnab-go/secrets/host"
	"github.com/hashicorp/go-hclog"
)

var _ cnabsecrets.Store = &Store{}

const (
	SecretKeyName = "secret"
)

// Store implements the backing store for secrets as kubernetes secrets.
type Store struct {
	logger    hclog.Logger
	config    config.Config
	hostStore cnabsecrets.Store
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
	return nil
}

func (s *Store) Resolve(keyName string, keyValue string) (string, error) {
	if strings.ToLower(keyName) != SecretKeyName {
		return s.hostStore.Resolve(keyName, keyValue)
	}

	return "", nil
}
