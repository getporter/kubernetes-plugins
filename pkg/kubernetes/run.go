package kubernetes

import (
	"fmt"
	"strings"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/config"
	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/secrets"
	"get.porter.sh/porter/pkg/plugins"
	"get.porter.sh/porter/pkg/portercontext"
	secretplugins "get.porter.sh/porter/pkg/secrets/plugins"
	"github.com/hashicorp/go-hclog"
	hplugin "github.com/hashicorp/go-plugin"
	"github.com/pkg/errors"
)

type RunOptions struct {
	Key               string
	selectedPlugin    hplugin.Plugin
	selectedInterface string
}

func (o *RunOptions) Validate(args []string, cfg config.Config) error {
	if len(args) == 0 {
		return errors.New("The positional argument KEY was not specified")
	}
	if len(args) > 1 {
		return errors.New("Multiple positional arguments were specified but only one, KEY is expected")
	}

	o.Key = args[0]

	availableImplementations := getPlugins()
	selectedPlugin, ok := availableImplementations[o.Key]
	if !ok {
		return errors.Errorf("invalid plugin key specified: %q", o.Key)
	}
	var err error
	o.selectedPlugin, err = selectedPlugin(portercontext.New(), cfg)
	if err != nil {
		return err
	}

	parts := strings.Split(o.Key, ".")
	o.selectedInterface = parts[0]

	return nil
}

func (p *Plugin) Run(args []string) {
	// This logger only helps log errors with loading the plugin
	logger := hclog.New(&hclog.LoggerOptions{
		Name:       "kubernetes",
		Output:     p.Err,
		Level:      hclog.Debug,
		JSONFormat: true,
	})
	err := p.LoadConfig()
	logger.Debug(fmt.Sprintf("Run.Plugin.Config.Namespace: %s", p.Namespace))
	if err != nil {
		logger.Error(err.Error())
		return
	}
	// We are not following the normal CLI pattern here because
	// if we write to stdout without the hclog, it will cause the plugin framework to blow up
	var opts RunOptions
	err = opts.Validate(args, p.Config)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	plugins.Serve(p.Context, opts.selectedInterface, opts.selectedPlugin, secretplugins.PluginProtocolVersion)
}

type pluginInitializer func(ctx *portercontext.Context, cfg config.Config) (hplugin.Plugin, error)

func getPlugins() map[string]pluginInitializer {

	return map[string]pluginInitializer{
		secrets.PluginKey: secrets.NewPlugin,
	}
}
