//go:build integration
// +build integration

package controllers_test

import (
	"context"
	"os"
	"testing"

	porterv1 "get.porter.sh/operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var k8sClient client.Client

var testEnv *envtest.Environment

func TestKubernetesPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		UseExistingCluster: pointer.BoolPtr(true),
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(scheme.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(porterv1.AddToScheme(scheme.Scheme)).To(Succeed())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	close(done)
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func createTestNamespace(ctx context.Context) string {
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "ginkgo-tests-",
			Labels: map[string]string{
				"porter.sh/testdata": "true",
			},
		},
	}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed())

	// porter-agent service account
	svc := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "porter-agent",
			Namespace: ns.Name,
		},
	}
	Expect(k8sClient.Create(ctx, svc)).To(Succeed())

	// Configure rbac for porter-agent
	svcRole := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "porter-agent-rolebinding",
			Namespace: ns.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      svc.Name,
				Namespace: svc.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "porter-operator-agent-role",
		},
	}
	Expect(k8sClient.Create(ctx, svcRole)).To(Succeed())

	// installation image service account
	instsa := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "installation-agent",
			Namespace: ns.Name,
		},
	}
	Expect(k8sClient.Create(ctx, instsa)).To(Succeed())

	agentRepo := os.Getenv("PORTER_AGENT_REPOSITORY")
	if agentRepo == "" {
		agentRepo = "ghcr.io/getporter/porter-agent"
	}
	agentVersion := os.Getenv("PORTER_AGENT_VERSION")
	if agentVersion == "" {
		agentVersion = "latest"
	}
	// Tweak porter agent config for testing
	porterOpsCfg := &porterv1.AgentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: ns.Name,
		},
		Spec: porterv1.AgentConfigSpec{
			PorterRepository:           agentRepo,
			PorterVersion:              agentVersion,
			ServiceAccount:             svc.Name,
			InstallationServiceAccount: "installation-agent",
		},
	}
	Expect(k8sClient.Create(ctx, porterOpsCfg)).To(Succeed())

	return ns.Name
}
