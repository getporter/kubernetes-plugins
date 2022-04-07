//go:build mage
// +build mage

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	. "get.porter.sh/magefiles/docker"
	. "get.porter.sh/magefiles/releases"
	. "get.porter.sh/magefiles/tests"
	"github.com/carolynvs/magex/mgx"
	"github.com/carolynvs/magex/pkg"
	"github.com/carolynvs/magex/pkg/gopath"
	"github.com/carolynvs/magex/shx"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/target"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	// mage:import
)

const (
	// Version of KIND to install if not already present
	kindVersion = "v0.11.1"

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
	operatorVersion  = "v0.5.2"
	operatorImage    = "porter-operator"
	operatorRegistry = "ghcr.io/getporter"
	// Porter version to use
	porterVersion = "v1.0.0-alpha.13"
	// Docker registry for porter client container
	porterRegistry   = "ghcr.io/getporter"
	porterConfigFile = "./tests/integration/operator/testdata/operator_porter_config.yaml"
)

// Dirs to watch recursively for build target
var (
	srcDirs             = []string{"cmd/", "pkg/"}
	binDir              = "bin/plugins/kubernetes/"
	pluginPkg           = fmt.Sprintf("./cmd/%s", pluginName)
	supportedClientGOOS = []string{"linux", "darwin", "windows"}
)

// Image name for local agent
var localAgentImgRepository = "localhost:5000/porter-agent-kubernetes"
var localAgentImgVersion = "canary-dev"
var localAgentImgName = fmt.Sprintf("%s:%s", localAgentImgRepository, localAgentImgVersion)

var operatorBundleRef = fmt.Sprintf("%s/%s:%s", operatorRegistry, operatorImage, operatorVersion)
var porterHome = filepath.Join(pwd(), "bin")

// Build a command that stops the build on if the command fails
var must = shx.CommandBuilder{StopOnError: true}

// Publish the cross-compiled binaries.
func Publish() {
	PublishPlugin(pluginName)
	PublishPluginFeed(pluginName)
}

func Install() {
	userPorterHome, found := os.LookupEnv("PORTER_HOME")
	if !found {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			mgx.Must(err)
		}
		userPorterHome = filepath.Join(homeDir, ".porter")
	}
	pluginDir := filepath.Join(userPorterHome, "plugins", pluginName)
	err := shx.Command("mkdir", "-p", pluginDir).RunV()
	mgx.Must(err)
	err = shx.Command("install", filepath.Join(binDir, pluginName), filepath.Join(pluginDir, pluginName)).RunV()
	mgx.Must(err)
}

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
func Test() {
	mg.SerialDeps(Build, TestLocalIntegration, TestIntegration)
}

// Run unit tests defined in srcDirs
func TestUnit() {
	mg.Deps(EnsureTestCluster)
	v := ""
	if mg.Verbose() {
		v = "-v"
	}
	args := []string{"test", v}
	for _, dir := range srcDirs {
		args = append(args, fmt.Sprintf("./%s...", dir))
	}
	must.Command("go", args...).CollapseArgs().RunV()

	// Verify integration tests compile
	must.Run("go", "test", "-run=non", "./tests/...")
}

func Build() {
	rebuild, err := target.Dir(binDir, srcDirs...)
	if err != nil {
		mgx.Must(fmt.Errorf("error inspecting source dirs %s: %w", srcDirs, err))
	}
	if rebuild {
		mg.SerialDeps(Clean, TestUnit)
		//TODO: fix issue with XBuildAll https://github.com/getporter/magefiles/issues/4
		var g errgroup.Group
		for _, goos := range supportedClientGOOS {
			goos := goos
			g.Go(func() error {
				return XBuild(pluginPkg, pluginName, binDir, goos, "amd64")
			})
		}
		mgx.Must(g.Wait())
		info := LoadMetadata()
		os.RemoveAll(filepath.Join(binDir, "dev"))
		shx.Copy(filepath.Join(binDir, info.Version), filepath.Join(binDir, "dev"), shx.CopyRecursive)
		//copy local arch bin to bin/plugins/kubernetes/kubernetes for local integration testing
		shx.Copy(filepath.Join(binDir, info.Version, fmt.Sprintf("%s-%s-%s", pluginName, runtime.GOOS, "amd64")), filepath.Join(binDir, pluginName))
		PreparePluginForPublish(pluginName)
	}
}

func SetupLocalTestEnv() {
	SetupTests()
}

// Run local integration tests against the test cluster.
func TestLocalIntegration() {
	mg.SerialDeps(Build, SetupLocalTestEnv)
	ctx, _ := kubectl("config", "current-context").OutputV()
	testLocalIntegration()
	must.RunV("go", "test", "-v", "./tests/integration/local/...")
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
	shx.Copy(filepath.Join(pwd(), "tests/integration/local/scripts/config-secret-test-local.toml"), filepath.Join(porterHome, "config.toml"))
	kubectl("apply", "-f", filepath.Join(pwd(), "tests/testdata/credentials-secret.yaml"), "-n", localTestNamespace).RunV()
	porter("plugins", "list").RunV()
	porter("credentials", "apply", filepath.Join(pwd(), "tests/testdata/kubernetes-plugin-test-secret.json")).RunV()
	porter("install", "--force", "--cred", "kubernetes-plugin-test", "-f", filepath.Join(pwd(), "tests/testdata/porter.yaml"), "--debug", "--debug-plugins").RunV()
}

// Run integration tests against the test cluster.
func TestIntegration() {
	mg.Deps(Build, EnsureGinkgo)
	mg.SerialDeps(SetupTests, EnsureTestNamespace)
	if os.Getenv("PORTER_AGENT_REPOSITORY") != "" && os.Getenv("PORTER_AGENT_VERSION") != "" {
		localAgentImgRepository = os.Getenv("PORTER_AGENT_REPOSITORY")
		localAgentImgVersion = os.Getenv("PORTER_AGENT_VERSION")
	}
	must.Command("ginkgo").Args("-p", "-nodes", "4", "-v", "./tests/integration/operator/ginkgo", "-coverprofile=coverage-integration.out").
		Env(fmt.Sprintf("PORTER_AGENT_REPOSITORY=%s", localAgentImgRepository),
			fmt.Sprintf("PORTER_AGENT_VERSION=%s", localAgentImgVersion),
			"ACK_GINKGO_DEPRECATIONS=1.16.5",
			"ACK_GINKGO_RC=true",
			fmt.Sprintf("KUBECONFIG=%s/kind.config", pwd())).RunV()
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
	ps := ""
	if os.Getenv("PORTER_AGENT_REPOSITORY") != "" && os.Getenv("PORTER_AGENT_VERSION") != "" {
		ps = "-p=dev-build"
	}

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

func CleanLastTestRun() {
	os.RemoveAll(".cnab")
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
	return must.Command("bin/porter").Args(args...).
		Env("PORTER_DEFAULT_STORAGE=",
			"PORTER_DEFAULT_STORAGE_PLUGIN=mongodb-docker",
			fmt.Sprintf("PORTER_HOME=%s", porterHome))
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

func EnsurePorterAt(version string) {
	if _, err := os.Stat(filepath.Join(porterHome, "porter")); err != nil {
		// TODO: pkg should support downloading to specific directory instead of just GOPATH to allow for namespaced versions of porter
		err := pkg.DownloadToGopathBin("https://cdn.porter.sh/{{.VERSION}}/porter-linux-amd64", "porter-runtime", version)
		mgx.Must(err)
		err = pkg.DownloadToGopathBin("https://cdn.porter.sh/{{.VERSION}}/porter-{{.GOOS}}-{{.GOARCH}}{{.EXT}}", "porter", version)
		mgx.Must(err)
	}
}

func SetupTests() {
	mg.Deps(EnsurePorterHomeBin)
	err := shx.Command("mkdir", "-p", filepath.Join(porterHome, "credentials")).RunV()
	mgx.Must(err)
	err = shx.Copy(filepath.Join(pwd(), "tests/integration/local/scripts/config-*.toml"), porterHome)
	mgx.Must(err)
	err = shx.Copy(filepath.Join(pwd(), "tests/testdata/kubernetes-plugin-test-*.json"), porterHome)
	mgx.Must(err)
	porter("mixin", "install", "exec").RunV()
}
func EnsurePorterHome() {
	if _, err := os.Stat(porterHome); err != nil {
		os.MkdirAll(porterHome, 0755)
	}
	if _, err := os.Stat(filepath.Join(porterHome, "runtimes")); err != nil {
		os.MkdirAll(filepath.Join(porterHome, "runtimes"), 0755)
	}
}

func EnsurePorterHomeBin() {
	mg.Deps(EnsurePorterHome)
	EnsurePorterAt(porterVersion)
	err := shx.Copy(filepath.Join(gopath.GetGopathBin(), "porter-runtime"), filepath.Join(porterHome, "runtimes/porter-runtime"))
	mgx.Must(err)
	err = shx.Copy(filepath.Join(gopath.GetGopathBin(), "porter"), porterHome)
	mgx.Must(err)
}
