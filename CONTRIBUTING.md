# Contributing Guide

---
* [New Contributor Guide](#new-contributor-guide)
* [Developer Tasks](#developer-tasks)
  * [Initial setup](#initial-setup)
  * [Makefile explained](#makefile-explained)
  * [Run a test installation](#run-a-test-installation)  
---

# New Contributor Guide

The [Porter New Contributor Guide](https://porter.sh/src/CONTRIBUTING.md) has information on how to find issues, what
type of help we are looking for, what to expect in your first pull request and
more.

# Developer Tasks

## Initial setup

We have a [tutorial] that walks you through how to setup your developer
environment for Porter, make a change and test it.

We recommend that you start there so that you understand how to use Porter.

You will need a Porter, Porter Operator KinD, Go, and Docker to work on the Kubernetes secrets plugin.

[tutorial]: https://porter.sh/contribute/tutorial/

```
# Install mage
go run mage.go EnsureMage

# Build and deploy the to a local porter test environment. 
# The Porter install is relative to this project directory (PORTER_HOME=${PWD}/bin)

mage TestLocalIntegration


# Build and deploy the to a k8s porter operator test environment.

mage TestIntegration
```

## Magefile explained

We use [mage](https://magefile.org) instead of make. If you don't have mage installed already,
you can install it with `go run mage.go EnsureMage`.

[mage]: https://magefile.org

Mage targets are not case-sensitive, but in our docs we use camel case to make
it easier to read. You can run either `mage Build` or `mage build` for
example. Run `mage` without any arguments to see a list of the available targets.

* **Build** builds the plugin, runs unit-test and cross compiles for local and in cluster testing.

**NOTE** This project is still moving targets from the original `Makefile` into mage. Please
file an issue for any problems encountered. Make will continue to be deprecated as functionality is
migrated into mage and all new functionality should be added directly to mage.

### Utility Targets
These are targets that you won't usually run directly, other targets use them as dependencies.

* **BuildLocalPorterAgent** builds an agent image with plugin for operator integration testing
* **EnsureTestCluster** starts a KIND cluster if it's not already running.
* **CreateTestCluster** creates a new KIND cluster named porter.
* **DeleteTestCluster** deletes the KIND cluster named porter.
* **Clean** removes all
* **CleanTestdata** removes any namespaces created by the test suite (with label porter.sh/testdata=true).

## Run a test installation

There are two primary test integration environments. The first for testing the `porter` command 
locally with the Kubernetes secrets plugin. The second environment is for testing via 
the [porter operator](https://github.com/getporter/operator).

The `mage EnsureTestCluster` target sets up everything needed to manually troubleshoot the
environment.

`mage TestLocalIntegration` run the porter command locally with the tests defined at
[tests/integration/local](tests/integration/local).

`mage TestIntegration` run the porter command via the operator with the tests defined at
[tests/integration/operator](tests/integration/operator).



