#!/bin/bash
set -euo pipefail

KUBERNETES_CONTEXT=kind-porter
TEST_NAMESPACE=porter-local-plugin-test-ns
CURRENT_CONTEXT=$(kubectl config current-context)
PORTER_HOME=${PWD}/bin
PORTER_CMD="${PWD}/bin/porter --debug --debug-plugins"
kubectl config use-context ${KUBERNETES_CONTEXT}
kubectl create namespace ${TEST_NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -
kubectl create secret generic password --from-literal=credential=test --namespace ${TEST_NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -
echo
cp ./tests/integration/local/scripts/config-secret-test-local.toml ${PORTER_HOME}/config.toml
kubectl apply -f ./tests/testdata/credentials-secret.yaml -n ${TEST_NAMESPACE}
${PORTER_CMD} plugins list
${PORTER_CMD} credential apply ./tests/testdata/kubernetes-plugin-test-secret.json
cd tests/testdata && ${PORTER_CMD} install --force --cred kubernetes-plugin-test --debug --debug-plugins
if [[ "$(${PORTER_CMD} installations outputs show test_out -i kubernetes-plugin-test)" != "test" ]]; then (exit 1); fi
${PORTER_CMD} installations show kubernetes-plugin-test
echo