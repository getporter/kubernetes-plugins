//go:build integration
// +build integration

package tests

import (
	"context"
	"fmt"
	"os"
	"testing"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/secrets"
	tests "get.porter.sh/plugin/kubernetes/tests/integration/local"
	"get.porter.sh/porter/pkg/portercontext"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

var logger hclog.Logger = hclog.New(&hclog.LoggerOptions{
	Name:   secrets.PluginKey,
	Output: os.Stdout,
	Level:  hclog.Error})

func Test_Default_Namespace(t *testing.T) {
	k8sConfig := secrets.PluginConfig{Logger: logger}
	tc := portercontext.NewTestContext(t)
	store := secrets.NewStore(tc.Context, k8sConfig)
	t.Run("Test Default Namespace", func(t *testing.T) {
		_, err := store.Resolve(context.Background(), "secret", "test")
		require.Error(t, err)
		if tests.RunningInKubernetes() {
			require.Contains(t, err.Error(), "secrets \"test\" not found")
		} else {
			require.Contains(t, err.Error(), "secrets \"test\" not found")
		}
	})
}

func Test_NoNamespace(t *testing.T) {
	tc := portercontext.NewTestContext(t)
	k8sConfig := secrets.PluginConfig{Logger: logger}
	store := secrets.NewStore(tc.Context, k8sConfig)
	t.Run("Test No Namespace", func(t *testing.T) {
		_, err := store.Resolve(context.Background(), "secret", "test")
		require.Error(t, err)
		require.Contains(t, err.Error(), "secrets \"test\" not found")
	})
}

func Test_Namespace_Does_Not_Exist(t *testing.T) {
	namespace := tests.GenerateNamespaceName()
	k8sConfig := secrets.PluginConfig{
		Namespace: namespace,
		Logger:    logger,
	}
	tc := portercontext.NewTestContext(t)
	store := secrets.NewStore(tc.Context, k8sConfig)
	t.Run("Test Namespace Does Not Exist", func(t *testing.T) {
		_, err := store.Resolve(context.Background(), "secret", "test")
		require.Error(t, err)
		require.Contains(t, err.Error(), "secrets \"test\" not found")
	})
}

func TestResolve_Secret(t *testing.T) {
	nsName := tests.CreateNamespace(t)
	k8sConfig := secrets.PluginConfig{
		Namespace: nsName,
		Logger:    logger,
	}
	tc := portercontext.NewTestContext(t)
	store := secrets.NewStore(tc.Context, k8sConfig)
	defer tests.DeleteNamespace(t, nsName)
	tests.CreateSecret(t, nsName, secrets.SecretDataKey, "testkey", "testvalue")
	t.Run("resolve secret source: value", func(t *testing.T) {
		resolved, err := store.Resolve(context.Background(), secrets.SecretSourceType, "testkey")
		require.NoError(t, err)
		require.Equal(t, "testvalue", resolved)
	})

}

func Test_UppercaseKey(t *testing.T) {
	nsName := tests.CreateNamespace(t)
	defer tests.DeleteNamespace(t, nsName)
	k8sConfig := secrets.PluginConfig{
		Namespace: nsName,
		Logger:    logger,
	}
	tc := portercontext.NewTestContext(t)
	store := secrets.NewStore(tc.Context, k8sConfig)
	tests.CreateSecret(t, nsName, secrets.SecretDataKey, "testkey", "testvalue")
	t.Run("Test Uppercase Key", func(t *testing.T) {
		resolved, err := store.Resolve(context.Background(), secrets.SecretSourceType, "TESTkey")
		require.NoError(t, err)
		require.Equal(t, "testvalue", resolved)
	})
}

func Test_IncorrectSecretDataKey(t *testing.T) {
	nsName := tests.CreateNamespace(t)
	defer tests.DeleteNamespace(t, nsName)
	k8sConfig := secrets.PluginConfig{
		Namespace: nsName,
		Logger:    logger,
	}
	tc := portercontext.NewTestContext(t)
	store := secrets.NewStore(tc.Context, k8sConfig)
	tests.CreateSecret(t, nsName, "invalid", "testkey", "testvalue")
	t.Run("Test Incorrect Secret Data Key", func(t *testing.T) {
		resolved, err := store.Resolve(context.Background(), secrets.SecretSourceType, "testkey")
		require.Error(t, err)
		require.EqualError(t, err, fmt.Sprintf(`The secret %s/%s does not have a key named %s. `+
			`The kubernetes.secrets plugin requires that the Kubernetes secret is named after the secret referenced in the `+
			`Porter parameter or credential set, and secret value is stored in a key on the Kubernetes secret named %s`,
			nsName, "testkey", secrets.SecretDataKey, secrets.SecretDataKey))
		require.Equal(t, resolved, "")
	})

}

func TestCreate_Secret(t *testing.T) {
	nsName := tests.CreateNamespace(t)
	k8sConfig := secrets.PluginConfig{
		Namespace: nsName,
		Logger:    logger,
	}
	tc := portercontext.NewTestContext(t)
	store := secrets.NewStore(tc.Context, k8sConfig)
	defer tests.DeleteNamespace(t, nsName)

	t.Run("success", func(t *testing.T) {
		err := store.Create(context.Background(), secrets.SecretSourceType, "testkey", "testValue")
		require.NoError(t, err)

		resolved, err := store.Resolve(context.Background(), secrets.SecretSourceType, "testkey")
		require.NoError(t, err)
		require.Equal(t, "testValue", resolved)
	})

	t.Run("exceeded maximum secret value size", func(t *testing.T) {
		invalidValue := make([]byte, v1.MaxSecretSize+1)
		err := store.Create(context.Background(), secrets.SecretSourceType, "testkey-max-secret-size", string(invalidValue))
		require.Error(t, err)

		_, err = store.Resolve(context.Background(), secrets.SecretSourceType, "testkey-max-secret-size")
		require.Error(t, err)
	})
}
