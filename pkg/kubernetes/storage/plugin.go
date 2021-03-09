package storage

import (
	"os"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/config"
	"get.porter.sh/porter/pkg/storage/crudstore"
	"github.com/cnabio/cnab-go/utils/crud"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
)

const PluginInterface = crudstore.PluginInterface + ".kubernetes.storage"

var _ crud.Store = &Plugin{}

// Plugin is the plugin wrapper for accessing storage from Kubernetes Secrets.
// Secrets are used rather than config maps as there may be sensitive information in the data
type Plugin struct {
	crud.Store
}

func NewPlugin(cfg config.Config) plugin.Plugin {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:       PluginInterface,
		Output:     os.Stderr,
		Level:      hclog.Debug,
		JSONFormat: true,
	})

	return &crudstore.Plugin{
		Impl: &Plugin{
			Store: NewStore(cfg, logger),
		},
	}
}
