// build integration

package storage

import (
	"fmt"
	"os"
	"testing"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/config"
	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/storage"
	"get.porter.sh/plugin/kubernetes/tests"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var logger hclog.Logger = hclog.New(&hclog.LoggerOptions{
	Name:   storage.PluginInterface,
	Output: os.Stderr,
	Level:  hclog.Error})

func Test_Default_Namespace(t *testing.T) {
	k8sConfig := config.Config{}
	store := storage.NewStore(k8sConfig, logger)
	t.Run("Test Default Namespace", func(t *testing.T) {
		_, err := store.Read("test", "test")
		require.Error(t, err)
		if tests.RunningInKubernetes() {
			require.EqualError(t, err, "secrets \"746573742d74657374\" not found")
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
	store := storage.NewStore(k8sConfig, logger)
	t.Run("Test Namepsace Does Not Exist", func(t *testing.T) {
		_, err := store.Read("test", "test")
		require.Error(t, err)
		require.EqualError(t, err, fmt.Sprintf("namespaces \"%s\" not found", namespace))
	})
}

func Test_Schema(t *testing.T) {

	logger := hclog.New(&hclog.LoggerOptions{
		Name:   t.Name(),
		Output: os.Stderr,
		Level:  hclog.Error,
	})

	nsName := tests.CreateNamespace(t)
	k8sConfig := config.Config{
		Namespace: nsName,
	}
	store := storage.NewStore(k8sConfig, logger)

	data, err := store.Read("", "schema")
	if err == nil {
		err = store.Delete("", "schema")
		require.NoError(t, err, "Deleting schema should not cause an error")
		data, err = store.Read("", "schema")
	}

	require.Error(t, err, "Reading non-existant schema should cause an error")
	assert.EqualError(t, err, "File does not exist")
	assert.Equal(t, 0, len(data))

	err = store.Save("", "", "schema", []byte("schema data"))
	require.NoError(t, err, "Saving schema should not cause an error")

	data, err = store.Read("", "schema")
	require.NoError(t, err, "Reading schema should not cause an error")
	assert.NotEqual(t, 0, len(data))
	assert.Equal(t, "schema data", string(data))

	err = store.Save("", "", "schema", []byte("schema data 1"))
	require.NoError(t, err, "Updating schema should not cause an error")

	//TODO: there is a bug in Kind that causes the original value to be read instead of the updated value when the test is running in kubernetes
	if !tests.RunningInKubernetes() {
		t.Log("Not running in Kubernetes")
		data, err = store.Read("", "schema")
		require.NoError(t, err, "Reading updated schema should not cause an error")
		assert.NotEqual(t, 0, len(data))
		assert.Equal(t, "schema data 1", string(data))
	} else {
		t.Log("Running in Kubernetes skipping test")
	}

}

func Test_NoGroup_NoData(t *testing.T) {

	testcases := []string{
		"installation1",
		"installation2",
		"installation3",
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Name:   t.Name(),
		Output: os.Stderr,
		Level:  hclog.Error,
	})

	nsName := tests.CreateNamespace(t)
	k8sConfig := config.Config{
		Namespace: nsName,
	}
	store := storage.NewStore(k8sConfig, logger)

	for _, tc := range testcases {
		t.Run(tc, func(t *testing.T) {
			err := store.Save("installations", "", tc, nil)
			require.NoError(t, err, "Saving installation %s should not result in an error", tc)
		})
	}

	for _, tc := range testcases {
		t.Run(tc, func(t *testing.T) {
			data, err := store.Read("installations", tc)
			require.NoError(t, err, "Reading installation %s should not result in an error", tc)
			assert.Equal(t, []byte(nil), data)
		})
	}

	data, err := store.List("installations", "")
	require.NoError(t, err, "Listing installations should not result in an error")
	assert.Equal(t, 3, len(data))

	count, err := store.Count("installations", "")
	require.NoError(t, err, "Counting installations should not result in an error")
	assert.Equal(t, 3, count)

	for _, tc := range testcases {
		t.Run(tc, func(t *testing.T) {
			err := store.Delete("installations", tc)
			require.NoError(t, err, "Deleting installation %s should not result in an error", tc)
		})
	}
}

func Test_With_Group_And_Data(t *testing.T) {

	testcases := map[string][]struct {
		name string
		data []byte
	}{
		"test1": {
			{
				"claim1",
				[]byte("claim1"),
			},
			{
				"claim2",
				[]byte("claim2"),
			},
			{
				"claim3",
				[]byte("claim3"),
			},
		},
		"test2": {
			{
				"claim4",
				[]byte("claim4"),
			},
			{
				"claim5",
				[]byte("claim5"),
			},
			{
				"claim6",
				[]byte("claim6"),
			},
			{
				"claim7",
				[]byte("claim7"),
			},
			{
				"claim8",
				[]byte("claim8"),
			},
		},
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Name:   t.Name(),
		Output: os.Stderr,
		Level:  hclog.Error,
	})

	nsName := tests.CreateNamespace(t)
	k8sConfig := config.Config{
		Namespace: nsName,
	}
	store := storage.NewStore(k8sConfig, logger)

	for group, tests := range testcases {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				err := store.Save("claims", group, tc.name, tc.data)
				require.NoError(t, err, "Saving claim %s in group %s should not result in an error", tc.name, group)
			})
		}
	}

	for group, tests := range testcases {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				data, err := store.Read("claims", tc.name)
				require.NoError(t, err, "Reading claim %s in group %s should not result in an error", tc.name, group)
				assert.Equal(t, tc.data, data)
			})
		}
	}

	for group, tests := range testcases {
		data, err := store.List("claims", group)
		require.NoError(t, err, "Listing claims for group %s should not result in an error", group)
		assert.Equal(t, len(tests), len(data))

		count, err := store.Count("claims", group)
		require.NoError(t, err, "Counting claims for group %s should not result in an error", group)
		assert.Equal(t, len(tests), count)
	}

	for group, tests := range testcases {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				err := store.Delete("claims", tc.name)
				require.NoError(t, err, "Deleting claim %s in group %s should not result in an error", tc.name, group)
			})
		}
	}

}
