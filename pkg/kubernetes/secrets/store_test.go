package secrets

import (
	"os"
	"testing"

	portercontext "get.porter.sh/porter/pkg/context"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

var logger hclog.Logger = hclog.New(&hclog.LoggerOptions{
	Name:   PluginKey,
	Output: os.Stdout,
	Level:  hclog.Error})

func Test_NoNamespace(t *testing.T) {
	tc := portercontext.TestContext{}
	k8sConfig := PluginConfig{Logger: logger}
	store := NewStore(tc.Context, k8sConfig)
	t.Run("Test No Namespace", func(t *testing.T) {
		_, err := store.Resolve("secret", "test")
		require.Error(t, err)
		require.EqualError(t, err, "secrets \"test\" not found")
	})
}
