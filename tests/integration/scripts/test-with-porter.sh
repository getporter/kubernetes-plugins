#!/usr/bin/env bash
set -euo pipefail
cd /app/bin
export IN_CLUSTER="true"
export KUBE_NAMESPACE="porter-plugin-test-ns"
export SERVICE_ACCOUNT="porter-plugin-test-sa"
export JOB_VOLUME_NAME="cnab-driver-share"
export JOB_VOLUME_PATH="/driverio"
export CLEANUP_JOBS="false"
function run-test {
  porter storage migrate
  porter install -r registry:5000/kubernetes-plugin-test:v1.0.0  --cred kubernetes-plugin-test  -d kubernetes --insecure-registry
  TEST_OUTPUT=$(porter installations outputs show test_out -i kubernetes-plugin-test)
  if [[ ${TEST_OUTPUT} != "test" ]]; then \
    echo "Unexpected Value for test credential:${TEST_OUTPUT}"
	  exit 1
  fi
  porter installations show kubernetes-plugin-test
}
# Run test with secrets only
cp $HOME/.porter/config-secret.toml $HOME/.porter/config.toml 
cp $HOME/.porter/credentials/kubernetes-plugin-test-secret.json $HOME/.porter/credentials/kubernetes-plugin-test.json 
run-test
# Run test with storage only
kubectl apply -f /app/tests/testdata/credentials-storage.yaml -n $KUBE_NAMESPACE 
cp $HOME/.porter/config-storage.toml $HOME/.porter/config.toml
cp $HOME/.porter/credentials/kubernetes-plugin-test-storage.json $HOME/.porter/credentials/kubernetes-plugin-test.json 
run-test
# Run test with secrets and storage
kubectl apply -f /app/tests/testdata/credentials-secret.yaml -n $KUBE_NAMESPACE 
cp $HOME/.porter/config-both.toml $HOME/.porter/config.toml 
cp $HOME/.porter/credentials/kubernetes-plugin-test-secret.json $HOME/.porter/credentials/kubernetes-plugin-test.json 
run-test