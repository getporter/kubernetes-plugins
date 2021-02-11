#!/usr/bin/env bash
set -euo pipefail
cd /app/bin
export IN_CLUSTER="true"
export KUBE_NAMESPACE="porter-plugin-test-ns"
export SERVICE_ACCOUNT="porter-plugin-test-sa"
export JOB_VOLUME_NAME="cnab-driver-share"
export JOB_VOLUME_PATH="/driverio"
export CLEANUP_JOBS="false"
porter storage migrate
porter install -r registry:5000/kubernetes-plugin-test:v1.0.0  --cred kubernetes-plugin-test  -d kubernetes --insecure-registry
TEST_OUTPUT=$(porter installations outputs show test_out -i kubernetes-plugin-test)
if [[ ${TEST_OUTPUT} != "test" ]]; then \
  echo "Unexpected Value for test credential:${TEST_OUTPUT}"
	exit 1
fi
