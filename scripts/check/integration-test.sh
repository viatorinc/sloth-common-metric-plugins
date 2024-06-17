#!/bin/bash
# vim: ai:ts=8:sw=8:noet
set -efCo pipefail
export SHELLOPTS
IFS=$'\t\n'

# integration tests are only run on the final version
if [[ -d dev-plugins ]];then
  echo "skipping integration-tests on development version (since they wouldn't work)"
  exit 0
fi

command -v sloth >/dev/null 2>&1 || { echo 'please install sloth'; exit 1; }

sloth validate -p ./plugins -i ./test/integration/ --debug
