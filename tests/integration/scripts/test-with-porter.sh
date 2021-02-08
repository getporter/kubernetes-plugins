#!/usr/bin/env bash
set -euo pipefail
cd /app/tests/testdata
porter storage migrate
porter build
porter install --cred kubernetes-plugin-test
TEST_OUTPUT=$(porter installations outputs show test_out -i kubernetes-plugin-test)
if [[ ${TEST_OUTPUT} != "test" ]]; then \
  echo "Unexpected Value for test credential:${TEST_OUTPUT}"
	exit 1
fi
