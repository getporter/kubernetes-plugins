//go:build integration
// +build integration

package controllers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	porterv1 "get.porter.sh/operator/api/v1"
	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/secrets"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"github.com/pkg/errors"
	"github.com/tidwall/pretty"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SecretsConfig struct {
	Namespace string `json:"namespace,omitempty"`
}

var _ = Describe("Porter using default secrets plugin config", func() {
	When("applying an Installation with a CredentialSet referencing a secret in the same namespace as the Installation resource", func() {
		It("successfully installs", func() {
			By("fetching CredentialSet secret from the kubernetes secret")
			randId := uuid.New()
			installationName := fmt.Sprintf("default-plugin-%v", randId)
			ns := createTestNamespace(context.Background())
			ctx := context.Background()
			createSecret(ns, secrets.SecretDataKey, "password", "test")
			credSet := NewCredSet("test", "insecureValue", "password")
			agentAction := createCredentialSetAgentAction(ns, credSet)
			pollAA := func() bool { return agentActionPoll(agentAction) }
			Eventually(pollAA, time.Second*120, time.Second*3).Should(BeTrue())
			inst := NewInstallation(installationName, ns)
			Expect(k8sClient.Create(ctx, inst)).Should(Succeed())

			// Wait for the job to be created
			installations := waitForInstallationStarted(ctx, ns, installationName)
			installation := installations.Items[0]

			// Wait for the installation to complete
			installation = waitForInstallationFinished(ctx, installation)

			// Validate that the installation status was updated
			validateInstallStatus(inst, porterv1.PhaseSucceeded)
		})
	})
	When("applying an Installation with a CredentialSet referencing a missing secret in the same namespace as the Installation resource", func() {
		It("fails to install", func() {
			randId := uuid.New()
			ctx := context.Background()
			installationName := fmt.Sprintf("default-plugin-%v", randId)
			installationNs := createTestNamespace(ctx)
			secretsNs := createTestNamespace(ctx)
			createSecret(secretsNs, secrets.SecretDataKey, "password", "test")
			credSet := NewCredSet("test", "insecureValue", "password")
			agentAction := createCredentialSetAgentAction(installationNs, credSet)
			pollAA := func() bool { return agentActionPoll(agentAction) }
			Eventually(pollAA, time.Second*120, time.Second*3).Should(BeTrue())
			inst := NewInstallation(installationName, installationNs)
			Expect(k8sClient.Create(ctx, inst)).Should(Succeed())

			// Wait for the job to be created
			installations := waitForInstallationStarted(ctx, installationNs, installationName)
			installation := installations.Items[0]

			// Wait for the installation to complete
			installation = waitForInstallationFinished(ctx, installation)

			// Validate that the installation status was updated
			validateInstallStatus(inst, porterv1.PhaseFailed)
		})
	})
	When("applying an Installation with a CredentialSet referencing a secret with an invalid data key in the same namespace as the Installation resource", func() {
		It("fails to install", func() {
			randId := uuid.New()
			ctx := context.Background()
			installationName := fmt.Sprintf("default-plugin-%v", randId)
			installationNs := createTestNamespace(ctx)
			createSecret(installationNs, "invalidKey", "password", "test")
			credSet := NewCredSet("test", "insecureValue", "password")
			agentAction := createCredentialSetAgentAction(installationNs, credSet)
			pollAA := func() bool { return agentActionPoll(agentAction) }
			Eventually(pollAA, time.Second*120, time.Second*3).Should(BeTrue())
			inst := NewInstallation(installationName, installationNs)
			Expect(k8sClient.Create(ctx, inst)).Should(Succeed())

			// Wait for the job to be created
			installations := waitForInstallationStarted(ctx, installationNs, installationName)
			installation := installations.Items[0]

			// Wait for the installation to complete
			installation = waitForInstallationFinished(ctx, installation)

			// Validate that the installation status was updated
			validateInstallStatus(inst, porterv1.PhaseFailed)
		})
	})
})

var _ = Describe("Porter using a secrets plugin config that doesn't specify the namespace", func() {
	When("applying an Installation with a CredentialSet referencing a secret that exists in the installation namespace", func() {
		It("successfully installs", func() {
			By("fetching CredentialSet secret from the kubernetes secret")
			randId := uuid.New()
			installationName := fmt.Sprintf("default-plugin-%v", randId)
			defaultSecretsCfgName := "kubernetes-secrets"
			ns := createTestNamespace(context.Background())
			ctx := context.Background()
			createSecret(ns, secrets.SecretDataKey, "password", "test")
			porterCfg := NewPorterConfig(ns)
			k8sSecretsCfg := NewSecretsPluginConfig(defaultSecretsCfgName, nil)
			SetPorterConfigSecrets(porterCfg, k8sSecretsCfg)
			porterCfg.Spec.DefaultSecrets = pointer.String(defaultSecretsCfgName)
			Expect(k8sClient.Create(context.Background(), porterCfg)).Should(Succeed())
			credSet := NewCredSet("test", "insecureValue", "password")
			agentAction := createCredentialSetAgentAction(ns, credSet)
			pollAA := func() bool { return agentActionPoll(agentAction) }
			Eventually(pollAA, time.Second*120, time.Second*3).Should(BeTrue())
			inst := NewInstallation(installationName, ns)
			Expect(k8sClient.Create(ctx, inst)).Should(Succeed())

			// Wait for the job to be created
			installations := waitForInstallationStarted(ctx, ns, installationName)
			installation := installations.Items[0]

			// Wait for the installation to complete
			installation = waitForInstallationFinished(ctx, installation)

			// Validate that the installation status was updated
			validateInstallStatus(inst, porterv1.PhaseSucceeded)
		})
	})
})

var _ = Describe("Porter using secrets plugin configured using same namespace as the Installation resource", func() {
	When("applying an Installation with a CredentialSet referencing a secret that doesn't exist in the Installation namespace", func() {
		It("fails to install", func() {
			By("failing to fetch the secret")
		})
	})
	When("applying an Installation CredentialSet referencing a secret in the Installation namespace", func() {
		It("successfully installs", func() {
			By("fetching CredentialSet secrets from the kubernetes secret")
			randId := uuid.New()
			installationName := fmt.Sprintf("porter-hello-%v", randId)
			ns := createTestNamespace(context.Background())
			ctx := context.Background()
			createSecret(ns, secrets.SecretDataKey, "password", "test")
			defaultSecretsCfgName := "kubernetes-secrets"
			porterCfg := NewPorterConfig(ns)
			secretsNamespaceCfg := &SecretsConfig{Namespace: ns}
			k8sSecretsCfg := NewSecretsPluginConfig(defaultSecretsCfgName, secretsNamespaceCfg)
			SetPorterConfigSecrets(porterCfg, k8sSecretsCfg)
			porterCfg.Spec.DefaultSecrets = pointer.String(defaultSecretsCfgName)
			Expect(k8sClient.Create(context.Background(), porterCfg)).Should(Succeed())
			credSet := NewCredSet("test", "insecureValue", "password")
			agentAction := createCredentialSetAgentAction(ns, credSet)
			pollAA := func() bool { return agentActionPoll(agentAction) }
			Eventually(pollAA, time.Second*120, time.Second*3).Should(BeTrue())
			inst := NewInstallation(installationName, ns)
			Expect(k8sClient.Create(ctx, inst)).Should(Succeed())

			// Wait for the job to be created
			installations := waitForInstallationStarted(ctx, ns, installationName)
			installation := installations.Items[0]

			// Validate that the job succeeded
			installation = waitForInstallationFinished(ctx, installation)

			// Validate that the installation status was updated
			validateInstallStatus(inst, porterv1.PhaseSucceeded)
		})
	})
})

var _ = Describe("Porter k8s secrets plugin configured using a different namespace than the Installation resource", func() {
	When("applying an Installation with a CredentialSet referencing a secret exists in the namespace defined in the porter config", func() {
		It("fails to install the installation", func() {
			By("failing to fetch the secret")
			randId := uuid.New()
			installName := fmt.Sprintf("secrets-elsewhere-%v", randId)
			installNamespace := createTestNamespace(context.Background())
			secretNamespace := createTestNamespace(context.Background())
			ctx := context.Background()
			var defaultSecretsCfgName, secretName, secretValue, credSetName = "kubernetes-secrets", "password", "test", "test"
			createSecret(secretNamespace, secrets.SecretDataKey, secretName, secretValue)
			porterCfg := NewPorterConfig(installNamespace)
			secretsNamespaceCfg := &SecretsConfig{Namespace: secretNamespace}
			k8sSecretsCfg := NewSecretsPluginConfig(defaultSecretsCfgName, secretsNamespaceCfg)
			porterCfg.Spec.DefaultSecrets = pointer.String(defaultSecretsCfgName)
			SetPorterConfigSecrets(porterCfg, k8sSecretsCfg)
			Expect(k8sClient.Create(context.Background(), porterCfg)).Should(Succeed())
			credSet := NewCredSet(credSetName, "insecureValue", secretName)

			//Create the porter credset via an AgentAction
			agentAction := createCredentialSetAgentAction(installNamespace, credSet)
			pollAA := func() bool { return agentActionPoll(agentAction) }
			Eventually(pollAA, time.Second*120, time.Second*3).Should(BeTrue())
			inst := NewInstallation(installName, installNamespace)
			Expect(k8sClient.Create(ctx, inst)).Should(Succeed())

			// Wait for the job to be created
			installations := waitForInstallationStarted(ctx, installNamespace, installName)
			installation := installations.Items[0]

			// Validate that the job succeeded
			installation = waitForInstallationFinished(ctx, installation)

			// Validate that the installation status was updated
			validateInstallStatus(inst, porterv1.PhaseFailed)
		})
	})
})

func validateInstallStatus(inst *porterv1.Installation, expectedPhase porterv1.AgentPhase) {
	ctx := context.Background()
	instName := types.NamespacedName{Namespace: inst.Namespace, Name: inst.Name}
	Expect(k8sClient.Get(ctx, instName, inst)).To(Succeed())
	Expect(apimeta.IsStatusConditionTrue(inst.Status.Conditions, string(porterv1.ConditionScheduled)))
	Expect(apimeta.IsStatusConditionTrue(inst.Status.Conditions, string(porterv1.ConditionStarted)))
	Expect(apimeta.IsStatusConditionTrue(inst.Status.Conditions, string(porterv1.ConditionComplete)))
	Expect(inst.Status.Phase).To(Equal(expectedPhase))
}

func agentActionPoll(aa *porterv1.AgentAction) bool {
	aaName := types.NamespacedName{Name: aa.Name, Namespace: aa.Namespace}
	Expect(k8sClient.Get(context.Background(), aaName, aa)).To(Succeed())
	return aa.Status.Phase == porterv1.PhaseSucceeded
}

func NewInstallation(installationName, installationNamespace string) *porterv1.Installation {
	return &porterv1.Installation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "porter.sh/v1",
			Kind:       "Installation",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      installationName,
			Namespace: installationNamespace,
		},
		Spec: porterv1.InstallationSpec{
			SchemaVersion: "1.0.0",
			Name:          installationName,
			Namespace:     installationNamespace,
			Bundle: porterv1.OCIReferenceParts{
				Repository: "ghcr.io/bdegeeter/porter-test-me",
				Version:    "0.2.0",
			},
			Parameters:     runtime.RawExtension{Raw: []byte("{\"delay\": \"1\", \"exitStatus\": \"0\"}")},
			CredentialSets: []string{"test"},
		},
	}
}

func NewCredSet(name, credName, source string) []byte {
	return []byte(fmt.Sprintf(`# Use the local kind cluster created by mage EnsureTestCluster
schemaVersion: 1.0.1
name: %s
credentials:
  - name: %s
    source:
      secret: %s
`, name, credName, source))
}

func waitForInstallationStarted(ctx context.Context, ns, installationName string) porterv1.InstallationList {
	installations := porterv1.InstallationList{}
	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	for {
		select {
		case <-waitCtx.Done():
			Fail(errors.Wrap(waitCtx.Err(), "timeout waiting for installation").Error())
		default:
			err := k8sClient.List(ctx, &installations, client.InNamespace(ns), client.MatchingFields{"metadata.name": installationName})
			Expect(err).Should(Succeed())
			if len(installations.Items) > 0 {
				return installations
			}
			time.Sleep(time.Second)
			continue
		}
	}
}

func waitForInstallationFinished(ctx context.Context, installation porterv1.Installation) porterv1.Installation {
	installationNameMatch := types.NamespacedName{Name: installation.Name, Namespace: installation.Namespace}
	Eventually(func() bool {
		Expect(k8sClient.Get(ctx, installationNameMatch, &installation)).To(Succeed())
		return IsInstallationDone(installation.Status)
	}, time.Second*120, time.Second*3).Should(BeTrue())
	return installation
}

func IsVolume(name string) gomegatypes.GomegaMatcher {
	return WithTransform(func(v corev1.Volume) string { return v.Name }, Equal(name))
}

func IsVolumeMount(name string) gomegatypes.GomegaMatcher {
	return WithTransform(func(v corev1.VolumeMount) string { return v.Name }, Equal(name))
}

func IsInstallationDone(status porterv1.InstallationStatus) bool {
	for _, c := range status.Conditions {
		if c.Type == string(porterv1.ConditionFailed) || c.Type == string(porterv1.ConditionComplete) {
			return true
		}
	}
	return false
}

func Log(value string, args ...interface{}) {
	GinkgoWriter.Write([]byte(fmt.Sprintf(value, args...)))
}

func LogJson(value string) {
	GinkgoWriter.Write(pretty.Pretty([]byte(value)))
}

func createCredentialSetAgentAction(namespace string, credSet []byte) *porterv1.AgentAction {
	agentAction := &porterv1.AgentAction{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "porter.sh/v1",
			Kind:       "AgentAction",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: namespace,
		},
		Spec: porterv1.AgentActionSpec{
			Args: []string{"credential", "apply", "creds.yaml"},
			Files: map[string][]byte{
				"creds.yaml": credSet,
			},
		},
	}
	Expect(k8sClient.Create(context.Background(), agentAction)).Should(Succeed())
	return agentAction
}

type PorterCfgOpts struct {
	ConfigNamespace  string
	SecretsName      string `default:"kubernetes-secrets"`
	SecretsNamespace string
}

//NewPorterCofnig GetDefaultPorterConfig - DefaultStorage, DefaultSecretsPlugin and DefaultStorageConfig set.  We tweak DefaultSecrets and SecretsConfig as needed
func NewPorterConfig(ns string) *porterv1.PorterConfig {
	porterCfg := &porterv1.PorterConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "porter.sh/v1",
			Kind:       "PorterConfig",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: ns,
		},
		Spec: porterv1.PorterConfigSpec{
			Namespace:            &ns,
			DefaultStorage:       pointer.StringPtr("in-cluster-mongodb"),
			DefaultSecretsPlugin: pointer.StringPtr("kubernetes.secrets"),
			Storage: []porterv1.StorageConfig{
				{
					PluginConfig: porterv1.PluginConfig{
						Name:         "in-cluster-mongodb",
						PluginSubKey: "mongodb",
						Config:       runtime.RawExtension{Raw: []byte(`{"url": "mongodb://mongodb.porter-operator-system.svc.cluster.local"}`)},
					},
				},
			},
		},
	}
	return porterCfg

}

func SetPorterConfigSecrets(cfg *porterv1.PorterConfig, secretsConfigs ...porterv1.SecretsConfig) {
	cfg.Spec.Secrets = secretsConfigs
}

func NewSecretsPluginConfig(name string, secretsCfg *SecretsConfig) porterv1.SecretsConfig {
	cfg := porterv1.SecretsConfig{
		PluginConfig: porterv1.PluginConfig{
			Name:         name,
			PluginSubKey: "kubernetes.secrets",
		},
	}
	if secretsCfg != nil {
		snc, _ := json.Marshal(*secretsCfg)
		cfg.Config.Raw = snc
	}
	return cfg
}

func createSecret(namespace, key, secretName, secretContents string) {
	k8sSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type:      corev1.SecretTypeOpaque,
		Immutable: pointer.BoolPtr(true),
		Data: map[string][]byte{
			key: []byte(secretContents),
		},
	}
	Expect(k8sClient.Create(context.Background(), k8sSecret)).Should(Succeed())
}
