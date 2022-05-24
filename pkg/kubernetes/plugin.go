package kubernetes

import (
	"bufio"
	"encoding/json"
	"io/ioutil"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/config"
	"get.porter.sh/porter/pkg/portercontext"
	"github.com/pkg/errors"
)

type Plugin struct {
	*portercontext.Context
	config.Config
}

// New kubernetes plugin client, initialized with useful defaults.
func New() *Plugin {
	return &Plugin{
		Context: portercontext.New(),
	}
}

func (p *Plugin) LoadConfig() error {
	reader := bufio.NewReader(p.In)
	b, err := ioutil.ReadAll(reader)
	if err != nil {
		return errors.Wrap(err, "could not read stdin")
	}

	if len(b) == 0 {
		return nil
	}

	err = json.Unmarshal(b, &p.Config)
	if err != nil {
		return errors.Wrapf(err, "error unmarshaling stdin %q as kubernetes.Config", string(b))
	}

	return nil
}
