#!/usr/bin/env bash
set -euo pipefail

trap 'mage clean' EXIT
mage testLocalIntegration
mage testIntegration