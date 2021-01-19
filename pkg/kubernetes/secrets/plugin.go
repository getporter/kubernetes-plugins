package secrets

import (
	"os"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/config"
	"get.porter.sh/porter/pkg/secrets"
	cnabsecrets "github.com/cnabio/cnab-go/secrets"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
)

const PluginInterface = secrets.PluginInterface + ".kubernetes.secret"

var _ cnabsecrets.Store = &Plugin{}

// Plugin is the plugin wrapper for accessing secrets from Kubernetes Secrets.
type Plugin struct {
	logger hclog.Logger
	cnabsecrets.Store
}

func NewPlugin(cfg config.Config) plugin.Plugin {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:       PluginInterface,
		Output:     os.Stderr,
		Level:      hclog.Debug,
		JSONFormat: true,
	})

	return &secrets.Plugin{
		Impl: &Plugin{
			Store: NewStore(cfg, logger),
		},
	}
}
