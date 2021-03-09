// +build integration

package integration

import (
	"fmt"
	"os"
	"testing"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/config"
	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/secrets"
	"get.porter.sh/plugin/kubernetes/tests"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

var logger hclog.Logger = hclog.New(&hclog.LoggerOptions{
	Name:   secrets.PluginInterface,
	Output: os.Stderr,
	Level:  hclog.Error})

func Test_Default_Namespace(t *testing.T) {
	k8sConfig := config.Config{}
	store := secrets.NewStore(k8sConfig, logger)
	t.Run("Test Default Namespace", func(t *testing.T) {
		_, err := store.Resolve("secret", "test")
		require.Error(t, err)
		if tests.RunningInKubernetes() {
			require.EqualError(t, err, "secrets \"test\" not found")
		} else {
			require.EqualError(t, err, "open /var/run/secrets/kubernetes.io/serviceaccount/namespace: no such file or directory")
		}
	})
}

func Test_Namespace_Does_Not_Exist(t *testing.T) {
	namespace := tests.GenerateNamespaceName()
	k8sConfig := config.Config{
		Namespace: namespace,
	}
	store := secrets.NewStore(k8sConfig, logger)
	t.Run("Test Namepsace Does Not Exist", func(t *testing.T) {
		_, err := store.Resolve("secret", "test")
		require.Error(t, err)
		require.EqualError(t, err, fmt.Sprintf("namespaces \"%s\" not found", namespace))
	})
}

func TestResolve_Secret(t *testing.T) {
	nsName := tests.CreateNamespace(t)
	k8sConfig := config.Config{
		Namespace: nsName,
	}
	store := secrets.NewStore(k8sConfig, logger)
	defer tests.DeleteNamespace(t, nsName)
	tests.CreateSecret(t, nsName, "testkey", "testvalue")
	t.Run("resolve secret source: value", func(t *testing.T) {
		resolved, err := store.Resolve(secrets.SecretSourceType, "testkey")
		require.NoError(t, err)
		require.Equal(t, "testvalue", resolved)
	})

}

func Test_UppercaseKey(t *testing.T) {
	nsName := tests.CreateNamespace(t)
	defer tests.DeleteNamespace(t, nsName)
	k8sConfig := config.Config{
		Namespace: nsName,
	}
	store := secrets.NewStore(k8sConfig, logger)
	tests.CreateSecret(t, nsName, "testkey", "testvalue")
	t.Run("Test Uppercase Key", func(t *testing.T) {
		resolved, err := store.Resolve(secrets.SecretSourceType, "TESTkey")
		require.NoError(t, err)
		require.Equal(t, "testvalue", resolved)
	})
}
