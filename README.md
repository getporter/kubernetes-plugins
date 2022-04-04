# Kubernetes Plugins for Porter

This is a set of Kubernetes plugins for [Porter](https://github.com/getporter/porter).

[![Build Status](https://dev.azure.com/getporter/porter/_apis/build/status/kubernetes-plugins-release?branchName=main)](https://dev.azure.com/getporter/porter/_build/latest?definitionId=23&branchName=main)

## Installation

The plugin is distributed as a single binary, `kubernetes`. The following snippet will clone this repository, build the binary
and install it to **~/.porter/plugins/**.

```shell
go get get.porter.sh/plugin/azure/cmd/kubernetes-plugins
cd $(go env GOPATH)/src/get.porter.sh/plugin/kubernetes-plugins
make build install
```

## Usage

After installion, you must modify your porter configuration file and select which types of data you want the plugin to store. The plugin supports both porter data (storage) and secret values (secrets), either or both of these capabilities can be used depending on configuration.

The plugin can be used when porter is running inside a Kubernetes cluster - in which case it will connect automatically, it can also be used from outside a cluster in which case it will either use the kubeconfig file sourced from the `KUBECONFIG` environment variable or `$HOME/.kube/config` if this is not set.

When running outside a cluster the plugin requires configuration to specify which namespace it should store data in, when running inside a cluster it will use the namespace of the pod that porter is running in.

The plugin also requires that the user or serivce account that is being used with Kubernetes has `"get","list","create","delete",` and `"patch"` permissions on secrets in the namepsace.

### Storage

The `Kubernetes.storage` plugin enables Porter to store data, such as claims, parameters and credentials, in a Kubernetes cluster. The plugin stores data in Kubernetes as secrets.

1. Open, or create, `~/.porter/config.toml`.

1. Add the following lines:

    ```toml
    default-storage = "kubernetes-storage"

    [[storage]]
    name = "kubernetes-storage"
    plugin = "kubernetes.storage"
    ```

* If the plugin is being used outside of a Kubernetes cluster then add the following lines to specify the namespace to be used to store data:

    ```toml
    [storage.config]
    namespace = "<namespace name>"
    ```

### Secrets

The `kubernetes.secrets` plugin enables resolution of credential or parameter values as secrets in Kubernetes.

1. Open, or create, `~/.porter/config.toml`
1. Add the following lines1:

    ```toml
    default-secrets = "kubernetes-secrets"

    [[secrets]]
    name = "kubernetes-secrets"
    plugin = "kubernetes.secrets"
    ```

* If the plugin is being used outside of a Kubernetes cluster then add the following lines to specify the namespace to be used to store data:

    ```toml
    [secrets.config]
    namespace = "<namespace name>"
    ```

### Storage and Secrets combined

When both storage and secrets are configured, be sure to place the `default-*` stanzas
at the top of the file, like so:

  ```toml
  default-secrets = "kubernetes-secrets"
  default-storage = "kubernetes-storage"

  [[secrets]]
  name = "kubernetes-secrets"
  plugin = "kubernetes.secrets"

  [[storage]]
  name = "kubernetes-storage"
  plugin = "kubernetes.storage"
  ```

If runing outside of Kubernetes then also include the namespace configuration
  
  ```toml
  [secrets.config]
  namespace = "<namespace name>"

  [storage.config]
  namespace = "<namespace name>"
  ```

Otherwise, Porter won't be able to parse the configuration correctly.
