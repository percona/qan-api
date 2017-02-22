# Percona Query Analytics API

[![Travis CI Build Status](https://travis-ci.org/percona/qan-api.svg?branch=master)](https://travis-ci.org/percona/qan-api)
[![GoDoc](https://godoc.org/github.com/percona/qan-api?status.svg)](https://godoc.org/github.com/percona/qan-api)
[![Report Card](http://goreportcard.com/badge/percona/qan-api)](http://goreportcard.com/report/percona/qan-api)

Percona Query Analytics (QAN) API is part of Percona Monitoring and Management (PMM).
See the [PMM docs](https://www.percona.com/doc/percona-monitoring-and-management/index.html) for more information.

##Building

In the empty dir run:
```
export GOPATH=$(pwd)
git clone http://github.com/percona/qan-api ./src/github.com/percona/qan-api
go build -o ./revel ./src/github.com/percona/qan-api/vendor/github.com/revel/cmd/revel
ln -s $(pwd)/src/github.com/percona/qan-api/vendor/github.com/revel src/github.com/revel
./revel build github.com/percona/qan-api <destination dir> prod
```
