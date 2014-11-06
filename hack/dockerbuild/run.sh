#!/bin/bash
echo Running with args "${@}"

set -e
set -o pipefail
set -v

test ${#} -eq 0 && MAKE_ARGS=( bootstrap install ) || MAKE_ARGS=( "${@}" )
GOPKG=github.com/mesosphere/kubernetes-mesos

mkdir -pv /pkg/src/${GOPKG} && cd /pkg/src/${GOPKG}
git clone https://${GOPKG}.git .

test "x${GIT_BRANCH}" = "x" || git checkout "${GIT_BRANCH}"
make "${MAKE_ARGS[@]}"
