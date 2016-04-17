#!/bin/bash
set -e

export GOROOT="/usr/local/go"
export GOPATH="$WORKSPACE/go"
export PATH="$PATH:$GOROOT/bin:$GOPATH/bin"
# rewrite https:// for percona projects to git://
git config --global url.git@github.com:percona/.insteadOf https://github.com/percona/
cd "$WORKSPACE/go/src/github.com/percona/qan-api"
CLOUD_API_CONF="test.conf" tests/runner.sh -u
