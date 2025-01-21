package secrets

import (
	"fmt"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/config"
	"get.porter.sh/porter/pkg/portercontext"
	"get.porter.sh/porter/pkg/secrets"
	"get.porter.sh/porter/pkg/secrets/plugins"
	"get.porter.sh/porter/pkg/secrets/pluginstore"
	"github.com/hashicorp/go-hclog"
	hplugin "github.com/hashicorp/go-plugin"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

const PluginKey = plugins.PluginInterface + ".kubernetes.secrets"

var _ plugins.SecretsProtocol = &Plugin{}

type PluginConfig struct {
	Namespace string `mapstructure:"namespace"`
	Logger    hclog.Logger
}

type Plugin struct {
	secrets.Store
}

func NewPlugin(cxt *portercontext.Context, pluginConfig config.Config) (hplugin.Plugin, error) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:       PluginKey,
		Output:     cxt.Err,
		Level:      hclog.Debug,
		JSONFormat: true,
	})
	cfg := PluginConfig{Logger: logger, Namespace: pluginConfig.Namespace}
	logger.Debug(fmt.Sprintf("NewPlugin.Config.Namespace: %s", cfg.Namespace))
	if err := mapstructure.Decode(pluginConfig, &cfg); err != nil {
		return nil, errors.Wrapf(err, "error decoding %s plugin config from %#v", PluginKey, pluginConfig)
	}
	return pluginstore.NewPlugin(cxt, NewStore(cxt, cfg)), nil
}
