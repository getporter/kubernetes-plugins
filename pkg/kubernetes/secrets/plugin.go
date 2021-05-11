package secrets

import (
	"fmt"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/config"
	portercontext "get.porter.sh/porter/pkg/context"
	"get.porter.sh/porter/pkg/secrets"
	"get.porter.sh/porter/pkg/secrets/plugins"
	cnabsecrets "github.com/cnabio/cnab-go/secrets"
	"github.com/hashicorp/go-hclog"
	hplugin "github.com/hashicorp/go-plugin"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

const PluginKey = plugins.PluginInterface + ".kubernetes.secrets"

var _ cnabsecrets.Store = &Plugin{}

type PluginConfig struct {
	KubeConfig string `mapstructure:"kubeconfig"`
	Namespace  string `mapstructure:"namespace"`
	Logger     hclog.Logger
}

type Plugin struct {
	cnabsecrets.Store
}

func NewPlugin(cxt *portercontext.Context, pluginConfig config.Config) (hplugin.Plugin, error) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:       PluginKey,
		Output:     cxt.Err,
		Level:      hclog.Debug,
		JSONFormat: true,
	})
	cfg := PluginConfig{Logger: logger, Namespace: pluginConfig.Namespace}
	logger.Info(fmt.Sprintf("NewPlugin.Config.Namespace: %s", cfg.Namespace))
	if err := mapstructure.Decode(pluginConfig, &cfg); err != nil {
		return nil, errors.Wrapf(err, "error decoding %s plugin config from %#v", PluginKey, pluginConfig)
	}
	return &secrets.Plugin{
		Impl: &Plugin{
			Store: NewStore(cxt, cfg),
		},
	}, nil
}
