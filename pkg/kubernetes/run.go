package kubernetes

import (
	"fmt"
	"strings"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/config"
	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/secrets"
	"get.porter.sh/porter/pkg/plugins"
	"get.porter.sh/porter/pkg/portercontext"
	"github.com/hashicorp/go-hclog"
	hplugin "github.com/hashicorp/go-plugin"
	"github.com/pkg/errors"
)

type RunOptions struct {
	Key               string
	selectedPlugin    hplugin.Plugin
	selectedInterface string
}

const (
	PluginProtocolVersion uint = 1
)

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

//TODO: implement our own Serve until porter/pkg/plugins supports protocolVersion
// Serve a single named plugin.
func Serve(interfaceName string, pluginImplementation hplugin.Plugin, protocolVersion uint) {
	ServeMany(map[string]hplugin.Plugin{interfaceName: pluginImplementation}, protocolVersion)
}

// Serve many plugins that the client will select by named interface.
func ServeMany(pluginMap map[string]hplugin.Plugin, protocolVersion uint) {
	plugins.HandshakeConfig.ProtocolVersion = protocolVersion
	hplugin.Serve(&hplugin.ServeConfig{
		HandshakeConfig: plugins.HandshakeConfig,
		Plugins:         pluginMap,
	})
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
	logger.Debug(fmt.Sprintf("Run.Plugin.Config.Namespace: %s", p.Config.Namespace))
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
	Serve(opts.selectedInterface, opts.selectedPlugin, PluginProtocolVersion)
}

func getPlugins(cfg config.Config) map[string]func() hplugin.Plugin {
	cxt := portercontext.New()
	secretPlugin, _ := secrets.NewPlugin(cxt, cfg)

	return map[string]func() hplugin.Plugin{
		secrets.PluginKey: func() hplugin.Plugin {
			return secretPlugin
		},
	}
}
