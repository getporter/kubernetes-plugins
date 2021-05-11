package tests

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes"
	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/secrets"
	portercontext "get.porter.sh/porter/pkg/context"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type TestPlugin struct {
	*kubernetes.Plugin
	TestContext *portercontext.TestContext
}

// NewTestPlugin initializes a plugin test client, with the output buffered, and an in-memory file system.
func NewTestPlugin(t *testing.T) *TestPlugin {
	c := portercontext.NewTestContext(t)
	m := &TestPlugin{
		Plugin: &kubernetes.Plugin{
			Context: c.Context,
		},
		TestContext: c,
	}

	return m
}

// GenerateNamespaceName generates a random namespace name

func GenerateNamespaceName() string {
	rand.Seed(time.Now().UTC().UnixNano())
	bytes := make([]byte, 10)
	for i := 0; i < 10; i++ {
		bytes[i] = byte(97 + rand.Intn(26))
	}
	return string(bytes)
}

func GetKubernetesConfig(t *testing.T) *restclient.Config {
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

func CreateNamespace(t *testing.T) string {
	nsName := GenerateNamespaceName()
	clientSet, err := k8s.NewForConfig(GetKubernetesConfig(t))
	require.NoError(t, err)
	ns := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}, TypeMeta: metav1.TypeMeta{Kind: "Namespace"}}
	_, err = clientSet.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	require.NoError(t, err)
	return nsName
}

func DeleteNamespace(t *testing.T, nsName string) string {
	clientSet, err := k8s.NewForConfig(GetKubernetesConfig(t))
	require.NoError(t, err)
	err = clientSet.CoreV1().Namespaces().Delete(context.Background(), nsName, metav1.DeleteOptions{})
	require.NoError(t, err)
	return nsName
}

func CreateSecret(t *testing.T, nsName string, name string, value string) {
	clientSet, err := k8s.NewForConfig(GetKubernetesConfig(t))
	name = strings.ToLower(name)
	require.NoError(t, err)
	data := make(map[string][]byte, 1)
	data[secrets.SecretDataKey] = []byte(value)
	secret := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name}, Data: data}
	_, err = clientSet.CoreV1().Secrets(nsName).Create(context.Background(), secret, metav1.CreateOptions{})
	require.NoError(t, err)
}

func RunningInKubernetes() bool {
	return len(os.Getenv("KUBERNETES_SERVICE_HOST")) > 0
}
