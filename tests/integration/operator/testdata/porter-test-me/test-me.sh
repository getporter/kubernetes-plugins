#!/usr/bin/env bash
set -euo pipefail
action=${1}
delay=${2}
exitCode=${3}
password=${4}
insecureValue=${INSECURE_VALUE:-default}

jsonOut=$(jq -n \
  --arg action ${action} \
  --arg delay ${delay} \
  --arg exitStatus ${exitCode} \
  --arg password ${password} \
  --arg insecureValue ${insecureValue} \
  '{"config": { "action":$action, "parameters":{"delay":$delay|tonumber, "exitStatus":$exitStatus|tonumber, "password":$password }}, "credentials": {"insecureValue":$insecureValue}}')

sleep ${delay}
echo ${jsonOut} | jq .
exit ${exitCode}