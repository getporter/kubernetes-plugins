#!/usr/bin/env bash
set -euo pipefail

trap 'make -f ./tests/Makefile.kind delete-kind-cluster' EXIT
make -f ./tests/Makefile.kind install-kind create-kind-cluster
make test-integration
make test-in-kubernetes