//go:build mage

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"get.porter.sh/magefiles/ci"
	"get.porter.sh/magefiles/docker"
	"get.porter.sh/magefiles/git"

	// mage:import
	_ "get.porter.sh/magefiles/docker"
	"get.porter.sh/magefiles/porter"
	"get.porter.sh/magefiles/releases"
	"get.porter.sh/magefiles/tests"

	// mage:import
	_ "get.porter.sh/magefiles/tests"
	"get.porter.sh/plugin/kubernetes/mage/setup"
	"github.com/carolynvs/magex/mgx"
	"github.com/carolynvs/magex/pkg"
	"github.com/carolynvs/magex/shx"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/target"
	"github.com/pkg/errors"
)

const (
	// Name of the KIND cluster used for testing
	kindClusterName = "porter"

	// Relative location of the KUBECONFIG for the test cluster
	kubeconfig = "kind.config"

	pluginName = "kubernetes"

	// Namespace of the porter plugin
	testNamespace = "porter-plugin-test-ns"
	// local plugin integration test namespace
	localTestNamespace = "porter-local-plugin-test-ns"
	// local plugin test kubernetes context for kind
	localTestKubernetesContext = "kind-porter"
	// Namespace of the porter operator
	operatorNamespace = "porter-operator-system"
	// Container name of the local registry
	registryContainer = "registry"
	// Operator bundle to deploy for integration tests
	operatorVersion  = "v0.7.2"
	operatorImage    = "porter-operator"
	operatorRegistry = "ghcr.io/getporter"
	// Porter version to use
	porterVersion = "v1.0.14"
	// Docker registry for porter client container
	porterRegistry   = "ghcr.io/getporter"
	porterConfigFile = "./tests/integration/operator/testdata/operator_porter_config.yaml"

	// The root package name of the plugin repository
	pluginPkg = "get.porter.sh/plugin/kubernetes"
)

// Dirs to watch recursively for build target
var (
	srcDirs             = []string{"cmd/", "pkg/", "go.mod", "magefile.go"}
	binDir              = "bin/plugins/kubernetes/"
	supportedClientGOOS = []string{"linux", "darwin", "windows"}
	// number of nodes for ginkgo parallel test execution (1=sequential)
	ginkgoNodes = "1"
)

// Image name for local agent
var localAgentImgRepository = "localhost:5000/porter-agent-kubernetes"
var localAgentImgVersion = "canary-dev"
var localAgentImgName = fmt.Sprintf("%s:%s", localAgentImgRepository, localAgentImgVersion)

var operatorBundleRef = fmt.Sprintf("%s/%s:%s", operatorRegistry, operatorImage, operatorVersion)

// Build a command that stops the build on if the command fails
var must = shx.CommandBuilder{StopOnError: true}

// Publish uploads the cross-compiled binaries for the plugin
func Publish() {
	mg.SerialDeps(porter.UseBinForPorterHome, porter.EnsurePorter)

	releases.PreparePluginForPublish(pluginName)
	releases.PublishPlugin(pluginName)
	releases.PublishPluginFeed(pluginName)
}

// TestPublish tries out publish locally, with your github forks
// Assumes that you forked and kept the repository name unchanged.
func TestPublish(username string) {
	pluginRepo := fmt.Sprintf("github.com/%s/%s-plugins", username, pluginName)
	pkgRepo := fmt.Sprintf("https://github.com/%s/packages.git", username)
	fmt.Printf("Publishing a release to %s and committing a mixin feed to %s\n", pluginRepo, pkgRepo)
	fmt.Printf("If you use different repository names, set %s and %s then call mage Publish instead.\n", releases.ReleaseRepository, releases.PackagesRemote)
	os.Setenv(releases.ReleaseRepository, pluginRepo)
	os.Setenv(releases.PackagesRemote, pkgRepo)

	Publish()
}

func Install() {
	pluginDir := filepath.Join(porter.GetPorterHome(), "plugins", pluginName)
	mgx.Must(os.MkdirAll(pluginDir, 0700))

	// Copy the plugin into PORTER_HOME
	mgx.Must(shx.Copy(filepath.Join(binDir, pluginName), pluginDir))
}

// Ensure mage is installed.
func ConfigureAgent() error {
	return ci.ConfigureAgent()
}

func Fmt() {
	must.RunV("go", "fmt", "./...")
}

func Vet() {
	must.RunV("go", "vet", "./...")
}
func Test() {
	mg.SerialDeps(Build, TestUnit, TestLocalIntegration, TestIntegration)
}

// Run unit tests defined in srcDirs
func TestUnit() {
	mg.Deps(tests.EnsureTestCluster)

	v := ""
	if mg.Verbose() {
		v = "-v"
	}

	must.Command("go", "test", v, "./...").CollapseArgs().RunV()

	// Verify integration tests compile
	must.Run("go", "test", "-run=non", "./tests/...")
}

func Build() {
	rebuild, err := target.Dir(filepath.Join(binDir, "kubernetes"), srcDirs...)
	if err != nil {
		mgx.Must(fmt.Errorf("error inspecting source dirs %s: %w", srcDirs, err))
	}
	if rebuild {
		mgx.Must(releases.BuildClient(pluginPkg, pluginName, binDir))
	} else {
		fmt.Println("target is up-to-date")
	}
}

func XBuildAll() {
	rebuild, err := target.Dir(filepath.Join(binDir, "dev/kubernetes-linux-amd64"), srcDirs...)
	if err != nil {
		mgx.Must(fmt.Errorf("error inspecting source dirs %s: %w", srcDirs, err))
	}
	if rebuild {
		releases.XBuildAll(pluginPkg, pluginName, binDir)
	} else {
		fmt.Println("target is up-to-date")
	}

	releases.PreparePluginForPublish(pluginName)
	verifyVersionStamp()
}

// verifyVersionStamp checks that the version was set on the cross-compiled binaries
func verifyVersionStamp() {
	// When this test fails, pluginPkg is set incorrectly or not passed to the releases functions properly
	pluginBinaryPath := filepath.Join(binDir, fmt.Sprintf("dev/kubernetes-%s-amd64", runtime.GOOS))
	versionOutput, _ := must.OutputV(pluginBinaryPath, "version", "-o=json")

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(versionOutput), &raw); err != nil {
		mgx.Must(fmt.Errorf("error parsing the version command output as json: %w", err))
	}

	meta := releases.LoadMetadata()
	gotVersion := raw["version"].(string)
	if gotVersion != meta.Version {
		mgx.Must(fmt.Errorf("the version was not set correctly on the kubernetes plugin binary %s: expected %q got %q", pluginBinaryPath, meta.Version, gotVersion))
	}
}

// Run local integration tests against the test cluster.
func TestLocalIntegration() {
	mg.Deps(Build)

	ctx, _ := kubectl("config", "current-context").OutputV()
	testLocalIntegration()
	must.RunV("go", "test", "-v", "-tags=integration", "./tests/integration/local/...")
	if ctx != "" {
		kubectl("config", "use-context", ctx).RunV()
	}
}

func testLocalIntegration() {
	kubectl("config", "use-context", localTestKubernetesContext).RunV()
	kubectl("create", "namespace", localTestNamespace).RunV()
	defer func() {
		kubectl("delete", "namespace", localTestNamespace).RunV()
	}()
	kubectl("create", "secret", "generic", "password", "--from-literal=value=test", "--namespace", localTestNamespace).RunV()
	mgx.Must(shx.Copy("tests/integration/local/scripts/config-secret-test-local.toml", "bin/config.toml"))
	kubectl("apply", "-f", "tests/testdata/credentials-secret.yaml", "-n", localTestNamespace).RunV()
	buildPorterCmd("credentials", "apply", "kubernetes-plugin-test-secret.json").
		In("tests/testdata").RunV()
	buildPorterCmd("install", "--force", "--cred", "kubernetes-plugin-test", "--verbosity=debug").
		In("tests/testdata").RunV()
}

// Run integration tests against the test cluster.
func TestIntegration() {
	mg.Deps(CleanTestdata, XBuildAll, EnsureGinkgo)
	mg.Deps(EnsureTestNamespace)

	if os.Getenv("PORTER_AGENT_REPOSITORY") != "" && os.Getenv("PORTER_AGENT_VERSION") != "" {
		localAgentImgRepository = os.Getenv("PORTER_AGENT_REPOSITORY")
		localAgentImgVersion = os.Getenv("PORTER_AGENT_VERSION")
	}
	must.Command("ginkgo").Args("-p", "-nodes", ginkgoNodes, "-tags=integration", "-v", "./tests/integration/operator/ginkgo", "-coverprofile=coverage-integration.out").
		Env(fmt.Sprintf("PORTER_AGENT_REPOSITORY=%s", localAgentImgRepository),
			fmt.Sprintf("PORTER_AGENT_VERSION=%s", localAgentImgVersion),
			"ACK_GINKGO_DEPRECATIONS=1.16.5",
			"ACK_GINKGO_RC=true",
			fmt.Sprintf("KUBECONFIG=%s", filepath.Join(pwd(), "kind.config"))).RunV()
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
	mg.Deps(tests.EnsureTestCluster)

	buildPorterCmd("credentials", "apply", "hack/creds.yaml", "-n=operator", "--verbosity=debug").Must().RunV()
	if os.Getenv("PORTER_OPERATOR_REF") != "" {
		operatorBundleRef = os.Getenv("PORTER_OPERATOR_REF")
	}
	buildPorterCmd("install", "operator", "-r", operatorBundleRef, "-c=kind", "--force", "-n=operator").Must().RunV()
}

// Delete the operator from the test cluster
func DeleteOperator() {
	buildPorterCmd("uninstall", "operator", "-r", operatorBundleRef, "-c=kind", "--force", "-n=operator").Must().RunV()
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
	mgx.Must(pkg.EnsurePackage("github.com/onsi/ginkgo/ginkgo", "1.16.5", "version"))
}

func setupTestNamespace() {
	SetupNamespace(testNamespace)
	EnsureTestSecret()
}

func namespaceExists(name string) bool {
	err := kubectl("get", "namespace", name).Must(false).RunS()
	return err == nil
}

// Create a namespace, usage: mage SetupNamespace demo.
// Configures the namespace for use with the operator.
func SetupNamespace(name string) {
	mg.Deps(tests.EnsureTestCluster)

	// Only specify the parameter set we have the env vars set
	// It would be neat if Porter could handle this for us
	PublishLocalPorterAgent()
	buildPorterCmd("parameters", "apply", "./hack/params.yaml", "-n=operator").RunV()
	ps := ""
	if os.Getenv("PORTER_AGENT_REPOSITORY") != "" && os.Getenv("PORTER_AGENT_VERSION") != "" {
		ps = "-p=dev-build"
	}

	buildPorterCmd("invoke", "operator", "--action=configureNamespace", ps, "--param", "namespace="+name, "--param", "porterConfig="+porterConfigFile, "-c", "kind", "-n=operator").
		CollapseArgs().Must().RunV()
	kubectl("label", "namespace", name, "--overwrite=true", "porter.sh/devenv=true").Must().RunV()

	setClusterNamespace(name)
}

// Remove the test cluster and registry.
func Clean() {
	os.RemoveAll("bin")
	CleanCluster()
}

// Remove the test cluster and registry.
func CleanCluster() {
	mg.Deps(tests.DeleteTestCluster, docker.StopDockerRegistry)
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
	kubectl("create", "secret", "generic", "password", "--from-literal=value=test", "-n", testNamespace).Must(false).Run()
}

func TestInstallation() {
	kubectl("-n", testNamespace, "create", "-f", "./tests/integration/operator/testdata/porter-test-me.yaml").Must(false).Run()
}

func kubectl(args ...string) shx.PreparedCommand {
	mg.Deps(tests.EnsureKubectl)

	kubeconfig := fmt.Sprintf("KUBECONFIG=%s", os.Getenv("KUBECONFIG"))
	return must.Command("kubectl", args...).Env(kubeconfig)
}

// Run porter using the local storage, not the in-cluster storage
func buildPorterCmd(args ...string) shx.PreparedCommand {
	mg.SerialDeps(porter.UseBinForPorterHome, ensurePorter, setup.InstallMixins)

	return must.Command(filepath.Join(pwd(), "bin/porter")).Args(args...).
		Env("PORTER_DEFAULT_STORAGE=",
			"PORTER_DEFAULT_STORAGE_PLUGIN=mongodb-docker",
			fmt.Sprintf("PORTER_HOME=%s", filepath.Join(pwd(), "bin")))
}

// ensurePorter makes sure the specified version of porter is installed.
func ensurePorter() {
	porter.EnsurePorterAt(porterVersion)
}

func PublishLocalPorterAgent() {
	mg.Deps(docker.StartDockerRegistry)

	// Check if we have a local porter build and use it in the agent
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
	if os.Getenv("PORTER_AGENT_REPOSITORY") != "" && os.Getenv("PORTER_AGENT_VERSION") != "" {
		localAgentImgName = fmt.Sprintf("%s:%s", os.Getenv("PORTER_AGENT_REPOSITORY"), os.Getenv("PORTER_AGENT_VERSION"))
	}

	if ok, _ := imageExists(localAgentImgName); ok {
		err := pushImage(localAgentImgName)
		mgx.Must(err)
	}
}

// Workaround to get the plugin built into an agent image for testing in the operator
func BuildLocalPorterAgent() {
	buildImage := func(img string) error {
		_, err := shx.Output("docker", "build", "-t", img,
			"--build-arg", fmt.Sprintf("PORTER_VERSION=%s", porterVersion),
			"--build-arg", fmt.Sprintf("REGISTRY=%s", porterRegistry),
			"-f", "tests/integration/operator/testdata/Dockerfile.customAgent", ".")
		if err != nil {
			return err
		}
		return nil
	}
	if os.Getenv("PORTER_AGENT_REPOSITORY") != "" && os.Getenv("PORTER_AGENT_VERSION") != "" {
		localAgentImgName = fmt.Sprintf("%s:%s", os.Getenv("PORTER_AGENT_REPOSITORY"), os.Getenv("PORTER_AGENT_VERSION"))
	}
	err := buildImage(localAgentImgName)
	mgx.Must(err)
}

// SetupDCO configures your git repository to automatically sign your commits
// to comply with our DCO
func SetupDCO() error {
	return git.SetupDCO()
}
