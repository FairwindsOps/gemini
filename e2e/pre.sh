#! /bin/bash
set -eo pipefail
docker cp . e2e-command-runner:/project
