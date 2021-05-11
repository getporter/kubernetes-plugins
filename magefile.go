//go:build mage
// +build mage

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "get.porter.sh/plugin/kubernetes/mage"
	. "get.porter.sh/porter/mage/docker"
	"github.com/carolynvs/magex/mgx"
	"github.com/carolynvs/magex/pkg"
	"github.com/carolynvs/magex/pkg/gopath"
	"github.com/carolynvs/magex/shx"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/target"
	"github.com/pkg/errors"

	// mage:import
	. "get.porter.sh/porter/mage/tests"
)

const (
	// Version of KIND to install if not already present
	kindVersion = "v0.11.1"

	// Name of the KIND cluster used for testing
	kindClusterName = "porter"

	// Relative location of the KUBECONFIG for the test cluster
	kubeconfig = "kind.config"

	// Namespace of the porter plugin
	testNamespace = "porter-plugin-test-ns"
	// Namespace of the porter operator
	operatorNamespace = "porter-operator-system"
	// Container name of the local registry
	registryContainer = "registry"
	// Operator bundle to deploy for integration tests
	operatorVersion  = "v0.5.1"
	operatorImage    = "porter-operator"
	operatorRegistry = "ghcr.io/getporter"
	// Porter version to use
	porterVersion = "v1.0.0-alpha.13"
	// Docker registry for porter client container
	porterRegistry   = "ghcr.io/getporter"
	porterConfigFile = "./tests/integration/operator/testdata/operator_porter_config.yaml"
)

// Dirs to watch recursively for build target
var srcDirs = []string{"cmd/", "pkg/"}
var binDir = "bin/plugins/kubernetes/"

// Image name for local agent
var localAgentImgRepository = "localhost:5000/porter-agent-kubernetes"
var localAgentImgVersion = "canary-dev"
var localAgentImgName = fmt.Sprintf("%s:%s", localAgentImgRepository, localAgentImgVersion)

var operatorBundleRef = fmt.Sprintf("%s/%s:%s", operatorRegistry, operatorImage, operatorVersion)

// Build a command that stops the build on if the command fails
var must = shx.CommandBuilder{StopOnError: true}

// We are migrating to mage, but for now keep using make as the main build script interface.

// Publish the cross-compiled binaries.
// func Publish(plugin string, version string, permalink string) {
// 	releases.PreparePluginForPublish(plugin, version, permalink)
// 	releases.PublishPlugin(plugin, version, permalink)
// 	releases.PublishPluginFeed(plugin, version)
// }

// Add GOPATH/bin to the path on the GitHub Actions agent
// TODO: Add to magex
func addGopathBinOnGithubActions() error {
	githubPath := os.Getenv("GITHUB_PATH")
	if githubPath == "" {
		return nil
	}

	log.Println("Adding GOPATH/bin to the PATH for the GitHub Actions Agent")
	gopathBin := gopath.GetGopathBin()
	return ioutil.WriteFile(githubPath, []byte(gopathBin), 0644)
}

// Ensure mage is installed.
func EnsureMage() error {
	addGopathBinOnGithubActions()
	return pkg.EnsureMage("v1.11.0")
}

func Fmt() {
	must.RunV("go", "fmt", "./...")
}

func Vet() {
	must.RunV("go", "vet", "./...")
}

func Build() {
	rebuild, err := target.Dir(binDir, srcDirs...)
	if err != nil {
		panic(err)
	}
	if rebuild {
		Clean()
		mg.Deps(EnsureTestCluster)
		must.RunV("make", "xbuild-all")
	}
}

func SetupLocalTestEnv() {
	must.RunV("make", "setup-tests")
}

// Run local integration tests against the test cluster.
func TestLocalIntegration() {
	mg.SerialDeps(Build, SetupLocalTestEnv)

	must.RunV("make", "test-integration")
}

// Run integration tests against the test cluster.
func TestIntegration() {
	// mg.Deps(UseTestEnvironment, CleanTestdata, EnsureGinkgo, EnsureDeployed)
	mg.SerialDeps(Build, SetupLocalTestEnv, EnsureTestNamespace, UseTestEnvironment)

	shx.Command("ginkgo").Args("-p", "-nodes", "4", "-v", "./tests/integration/operator/ginkgo", "-coverprofile=coverage-integration.out").
		Env(fmt.Sprintf("PORTER_AGENT_REPOSITORY=%s", localAgentImgRepository),
			fmt.Sprintf("PORTER_AGENT_VERSION=%s", localAgentImgVersion),
			"ACK_GINKGO_DEPRECATIONS=1.16.5",
			"ACK_GINKGO_RC=true",
			fmt.Sprintf("KUBECONFIG=%s/kind.config", pwd())).RunV()
	//mg.SerialDeps(UseTestEnvironment)
}

func RunIntegrationTest() {
	must.RunV("go", "test", "-v", "./tests/integration/operator/ginkgo", "-coverprofile=coverage-integration.out")
}

// Remove data created by running the test suite
func CleanTestdata() {
	if useCluster() {
		// find all test namespaces
		output, _ := kubectl("get", "ns", "-l", "porter.sh/testdata=true", `--template={{range .items}}{{.metadata.name}},{{end}}`).
			OutputE()
		namespaces := strings.Split(output, ",")

		// Remove the finalizers from any testdata in that namespace
		// Otherwise they will block when you delete the namespace
		for _, namespace := range namespaces {
			if namespace == "" {
				continue
			}

			output, _ = kubectl("get", "installation", "-n", namespace, `--template={{range .items}}{{.kind}}/{{.metadata.name}},{{end}}`).
				Output()
			resources := strings.Split(output, ",")
			for _, resource := range resources {
				if resource == "" {
					continue
				}
				//kubectl("patch", "-n", namespace, resource, "-p", `{"metadata":{"finalizers":null}}`, "--type=merge").RunV()
				kubectl("patch", "-n", namespace, resource, "-p", `[{"op": "remove", "path": "/metadata/finalizers"}]`, "--type=json").Must(false).RunV()
				kubectl("delete", "-n", namespace, resource).Must(false).RunV()
				time.Sleep(time.Second * 6)
			}
		}
		for _, namespace := range namespaces {
			if namespace == "" {
				continue
			}
			kubectl("delete", "ns", namespace).Run()
		}

	}
}

// Build the operator and deploy it to the test cluster using
func DeployOperator() {
	mg.Deps(SetupLocalTestEnv)
	porter("credentials", "apply", "hack/creds.yaml", "-n=operator", "--debug", "--debug-plugins").Must().RunV()
	if os.Getenv("PORTER_OPERATOR_REF") != "" {
		operatorBundleRef = os.Getenv("PORTER_OPERATOR_REF")
	}
	porter("install", "operator", "-r", operatorBundleRef, "-c=kind", "--force", "-n=operator").Must().RunV()
}

// Delete the operator from the test cluster
func DeleteOperator() {
	porter("uninstall", "operator", "-r", operatorBundleRef, "-c=kind", "--force", "-n=operator").Must().RunV()
}

// get the config of the current kind cluster, if available
func getClusterConfig() (kubeconfig string, ok bool) {
	contents, err := shx.OutputE("kind", "get", "kubeconfig", "--name", kindClusterName)
	return contents, err == nil
}

// setup environment to use the current kind cluster, if available
func useCluster() bool {
	contents, ok := getClusterConfig()
	if ok {
		log.Println("Reusing existing kind cluster")

		userKubeConfig, _ := filepath.Abs(os.Getenv("KUBECONFIG"))
		currentKubeConfig := filepath.Join(pwd(), kubeconfig)
		if userKubeConfig != currentKubeConfig {
			fmt.Printf("ATTENTION! You should set your KUBECONFIG to match the cluster used by this project\n\n\texport KUBECONFIG=%s\n\n", currentKubeConfig)
		}
		os.Setenv("KUBECONFIG", currentKubeConfig)

		err := ioutil.WriteFile(kubeconfig, []byte(contents), 0644)
		mgx.Must(errors.Wrapf(err, "error writing %s", kubeconfig))

		setClusterNamespace(testNamespace)
		return true
	}

	return false
}

func setClusterNamespace(name string) {
	shx.RunE("kubectl", "config", "set-context", "--current", "--namespace", name)
}

// Check if the operator is deployed to the test cluster.
func EnsureDeployed() {
	if !isDeployed() {
		DeployOperator()
	}
}

func isDeployed() bool {
	if useCluster() {
		if err := kubectl("rollout", "status", "deployment", "porter-operator-controller-manager", "--namespace", operatorNamespace).Must(false).Run(); err != nil {
			log.Println("the operator is not installed")
			return false
		}
		if err := kubectl("rollout", "status", "deployment", "mongodb", "--namespace", operatorNamespace).Must(false).Run(); err != nil {
			log.Println("the database is not installed")
			return false
		}
		log.Println("the operator is installed and ready to use")
		return true
	}
	log.Println("could not connect to the test cluster")
	return false
}

// Ensures that a namespace named "test" exists.
func EnsureTestNamespace() {
	mg.Deps(EnsureDeployed)
	if !namespaceExists(testNamespace) {
		setupTestNamespace()
	}
}

// Ensure ginkgo is installed.
func EnsureGinkgo() {
	mgx.Must(pkg.EnsurePackage("github.com/onsi/ginkgo/ginkgo", "", ""))
}

func setupTestNamespace() {
	SetupNamespace(testNamespace)
	EnsureTestSecret()
	SetupTestCredentialSet()
}

func namespaceExists(name string) bool {
	err := kubectl("get", "namespace", name).Must(false).RunS()
	return err == nil
}

// Create a namespace, usage: mage SetupNamespace demo.
// Configures the namespace for use with the operator.
func SetupNamespace(name string) {
	mg.Deps(EnsureTestCluster)

	// Only specify the parameter set we have the env vars set
	// It would be neat if Porter could handle this for us
	PublishLocalPorterAgent()
	porter("parameters", "apply", "./hack/params.yaml", "-n=operator").RunV()
	ps := "-p=dev-build"

	porter("invoke", "operator", "--action=configureNamespace", ps, "--param", "namespace="+name, "--param", "porterConfig="+porterConfigFile, "-c", "kind", "-n=operator").
		CollapseArgs().Must().RunV()
	kubectl("label", "namespace", name, "--overwrite=true", "porter.sh/devenv=true").Must().RunV()

	setClusterNamespace(name)
}

// Remove the test cluster and registry.
func Clean() {
	mg.Deps(DeleteTestCluster, StopDockerRegistry)
	os.RemoveAll("bin")
}

func pwd() string {
	wd, _ := os.Getwd()
	return wd
}

func EnsureTestSecret() {
	if !testSecretExists() {
		setupTestSecret()
	}
}
func testSecretExists() bool {
	err := kubectl("-n", testNamespace, "get", "secrets", "password").Must(false).RunS()
	return err == nil
}
func setupTestSecret() {
	kubectl("create", "secret", "generic", "password", "--from-literal=credential=test", "-n", testNamespace).Must(false).Run()
}

func SetupTestCredentialSet() {
	kubectl("-n", testNamespace, "apply", "-f", "./tests/integration/operator/testdata/agent_action_create_password_creds.yaml").Must(false).Run()
}

func TestInstallation() {
	kubectl("-n", testNamespace, "create", "-f", "./tests/integration/operator/testdata/porter-test-me.yaml").Must(false).Run()
}

func kubectl(args ...string) shx.PreparedCommand {
	kubeconfig := fmt.Sprintf("KUBECONFIG=%s", os.Getenv("KUBECONFIG"))
	return must.Command("kubectl", args...).Env(kubeconfig)
}

// Run porter using the local storage, not the in-cluster storage
func porter(args ...string) shx.PreparedCommand {
	return shx.Command("bin/porter").Args(args...).
		Env("PORTER_DEFAULT_STORAGE=", "PORTER_DEFAULT_STORAGE_PLUGIN=mongodb-docker")
}

func PublishLocalPorterAgent() {
	// Check if we have a local porter build
	// TODO: let's move some of these helpers into Porter
	mg.Deps(EnsureTestCluster, SetupLocalTestEnv)
	BuildLocalPorterAgent()
	imageExists := func(img string) (bool, error) {
		out, err := shx.Output("docker", "image", "inspect", img)
		if err != nil {
			if strings.Contains(out, "No such image") {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}

	pushImage := func(img string) error {
		return shx.Run("docker", "push", img)
	}
	// if os.Getenv("PORTER_AGENT_REPOSITORY") != "" && os.Getenv("PORTER_AGENT_VERSION") != "" {
	// 	localAgentImgName = fmt.Sprintf("%s:%s", os.Getenv("PORTER_AGENT_REPOSITORY"), os.Getenv("PORTER_AGENT_VERSION"))
	// }

	if ok, _ := imageExists(localAgentImgName); ok {
		err := pushImage(localAgentImgName)
		mgx.Must(err)
	}
}

// Workaround to get the plugin built into an agent image for testing in the operator
func BuildLocalPorterAgent() {
	buildImage := func(img string) error {
		_, err := shx.Output("docker", "build", "-t", img, "--build-arg", fmt.Sprintf("PORTER_VERSION=%s",
			porterVersion), "--build-arg", fmt.Sprintf("REGISTRY=%s", porterRegistry),
			"-f", "tests/integration/operator/testdata/Dockerfile.customAgent", ".")
		if err != nil {
			return err
		}
		return nil
	}
	// if os.Getenv("PORTER_AGENT_REPOSITORY") != "" && os.Getenv("PORTER_AGENT_VERSION") != "" {
	// 	localAgentImgName = fmt.Sprintf("%s:%s", os.Getenv("PORTER_AGENT_REPOSITORY"), os.Getenv("PORTER_AGENT_VERSION"))
	// }
	err := buildImage(localAgentImgName)
	mgx.Must(err)
}
