package kubernetes

import (
	"strings"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/config"
	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/secrets"
	"get.porter.sh/porter/pkg/plugins"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/pkg/errors"
)

type RunOptions struct {
	Key               string
	selectedPlugin    plugin.Plugin
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

	availableImplementations := getPlugins(cfg)
	selectedPlugin, ok := availableImplementations[o.Key]
	if !ok {
		return errors.Errorf("invalid plugin key specified: %q", o.Key)
	}
	o.selectedPlugin = selectedPlugin()

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

	plugins.Serve(opts.selectedInterface, opts.selectedPlugin)
}

func getPlugins(cfg config.Config) map[string]func() plugin.Plugin {
	return map[string]func() plugin.Plugin{
		secrets.PluginInterface: func() plugin.Plugin { return secrets.NewPlugin(cfg) },
	}
}
