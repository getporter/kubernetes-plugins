# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Kubernetes secrets plugin for [Porter](https://getporter.sh), a cloud-native application management tool. The plugin enables Porter to use Kubernetes secrets as a source for CredentialSets and stores sensitive data (parameter or output values) as secrets in Kubernetes clusters.

**Key Components:**
- **Secrets Plugin**: Primary functionality for managing secrets in Kubernetes clusters
- **Porter Integration**: Implements Porter's plugin interface for secrets management
- **Kubernetes Client**: Direct integration with Kubernetes API for secret operations
- **Dual Environment Support**: Works both inside and outside Kubernetes clusters

## Development Commands

### Build and Test
```bash
# Build the plugin
mage Build

# Run all tests (unit + integration)
mage Test

# Run only unit tests
mage TestUnit

# Run local integration tests (using local Porter command)
mage TestLocalIntegration

# Run operator integration tests (using Porter Operator)
mage TestIntegration

# Cross-compile for all platforms
mage XBuildAll
```

### Development Environment Setup
```bash
# Install mage build tool
go run mage.go ConfigureAgent

# Format code
mage Fmt

# Vet code
mage Vet

# Install plugin to local Porter environment
mage Install

# Clean build artifacts and test clusters
mage Clean
```

### Testing Infrastructure
```bash
# Ensure KIND test cluster is running
mage tests.EnsureTestCluster

# Deploy Porter Operator to test cluster
mage DeployOperator

# Remove Porter Operator from test cluster
mage DeleteOperator

# Setup namespace for testing
mage SetupNamespace <namespace-name>

# Clean test data from clusters
mage CleanTestdata
```

## Architecture

### Plugin Structure
- `cmd/kubernetes/`: Main plugin entry point and CLI commands
- `pkg/kubernetes/`: Core plugin implementation
  - `config/`: Configuration handling for plugin settings
  - `secrets/`: Kubernetes secrets store implementation
  - `helper/`: Kubernetes client utilities
- `mage/setup/`: Build and setup utilities
- `tests/`: Integration and unit tests
  - `integration/local/`: Tests for local Porter command usage
  - `integration/operator/`: Tests for Porter Operator usage

### Key Interfaces
- **SecretsProtocol**: Main plugin interface for Porter secrets operations
- **Store**: Kubernetes secrets backend implementation
- **Plugin**: HashiCorp go-plugin wrapper for Porter integration

### Configuration
The plugin supports two deployment modes:
1. **In-cluster**: Automatically detects namespace when running inside Kubernetes
2. **Out-of-cluster**: Requires namespace configuration and kubeconfig access

### Dependencies
- **Porter Framework**: v1.2.1 - Application packaging and deployment
- **Kubernetes Client**: v0.32.1 - Direct K8s API interaction
- **HashiCorp go-plugin**: Plugin architecture foundation
- **CNAB**: Cloud Native Application Bundle specifications

## Testing Strategy

### Local Integration Tests
- Tests Porter command execution with the plugin
- Uses KIND cluster with local namespace isolation
- Validates secret creation, retrieval, and credential resolution

### Operator Integration Tests  
- Tests plugin functionality within Porter Operator environment
- Uses Ginkgo test framework for structured testing
- Validates end-to-end installation workflows with Kubernetes secrets

### Test Requirements
- KIND cluster for Kubernetes testing
- Docker for container operations
- Local Porter binary for integration testing
- Ginkgo for operator integration tests

## Security Considerations

- Plugin requires `get`, `list`, `create`, `delete`, and `patch` permissions on secrets
- Supports both service account (in-cluster) and kubeconfig (out-of-cluster) authentication
- Secret values are stored with `value` key in Kubernetes secret data
- Namespace isolation enforced for multi-tenant environments