package tests

import (
	"math/rand"
	"testing"
	"time"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes"
	"get.porter.sh/porter/pkg/context"
)

type TestPlugin struct {
	*kubernetes.Plugin
	TestContext *context.TestContext
}

// NewTestPlugin initializes a plugin test client, with the output buffered, and an in-memory file system.
func NewTestPlugin(t *testing.T) *TestPlugin {
	c := context.NewTestContext(t)
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
