// +build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/config"
	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/secrets"
	"get.porter.sh/plugin/kubernetes/tests"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var logger hclog.Logger = hclog.New(&hclog.LoggerOptions{
	Name:   secrets.PluginInterface,
	Output: os.Stderr,
	Level:  hclog.Error})

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
	nsName := createNamespace(t)
	k8sConfig := config.Config{
		Namespace: nsName,
	}
	store := secrets.NewStore(k8sConfig, logger)
	defer deleteNamespace(t, nsName)
	createSecret(t, nsName, "testkey", "testvalue")
	t.Run("resolve secret source: value", func(t *testing.T) {
		resolved, err := store.Resolve(secrets.SecretSourceType, "testkey")
		require.NoError(t, err)
		require.Equal(t, "testvalue", resolved)
	})

}

func Test_UppercaseKey(t *testing.T) {
	nsName := createNamespace(t)
	defer deleteNamespace(t, nsName)
	k8sConfig := config.Config{
		Namespace: nsName,
	}
	store := secrets.NewStore(k8sConfig, logger)
	createSecret(t, nsName, "testkey", "testvalue")
	t.Run("Test Uppercase Key", func(t *testing.T) {
		resolved, err := store.Resolve(secrets.SecretSourceType, "TESTkey")
		require.NoError(t, err)
		require.Equal(t, "testvalue", resolved)
	})
}

func getKubernetesConfig(t *testing.T) *restclient.Config {
	var err error
	var config *restclient.Config
	var kubeconfigfile string
	if kubeconfigfile = os.Getenv("KUBECONFIG"); kubeconfigfile == "" {
		kubeconfigfile = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	if _, err = os.Stat(kubeconfigfile); err != nil {
		if os.IsNotExist(err) {
			// If the kubeconfig file does not exist then try in cluster config
			config, err = clientcmd.BuildConfigFromFlags("", "")
		}
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigfile)
	}

	require.NoError(t, err)
	require.NotNil(t, config)
	return config
}

func createNamespace(t *testing.T) string {
	nsName := tests.GenerateNamespaceName()
	clientSet, err := kubernetes.NewForConfig(getKubernetesConfig(t))
	require.NoError(t, err)
	ns := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}, TypeMeta: metav1.TypeMeta{Kind: "Namespace"}}
	_, err = clientSet.CoreV1().Namespaces().Create(ns)
	require.NoError(t, err)
	return nsName
}

func deleteNamespace(t *testing.T, nsName string) string {
	clientSet, err := kubernetes.NewForConfig(getKubernetesConfig(t))
	require.NoError(t, err)
	err = clientSet.CoreV1().Namespaces().Delete(nsName, &metav1.DeleteOptions{})
	require.NoError(t, err)
	return nsName
}

func createSecret(t *testing.T, nsName string, name string, value string) {
	clientSet, err := kubernetes.NewForConfig(getKubernetesConfig(t))
	name = strings.ToLower(name)
	require.NoError(t, err)
	data := make(map[string][]byte, 1)
	data[secrets.SecretDataKey] = []byte(value)
	secret := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name}, Data: data}
	_, err = clientSet.CoreV1().Secrets(nsName).Create(secret)
	require.NoError(t, err)
}
