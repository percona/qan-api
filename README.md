# Percona Query Analytics API

Percona Query Analytics (QAN) API is part of Percona Monitoring and Management (PMM).
See the [PMM docs](https://www.percona.com/doc/percona-monitoring-and-management/index.html) for more information.

##Updating dependencies

Install govendor: `go get -u github.com/kardianos/govendor`  
Fetch dependencies from the original repo (not local copy on GOPATH): `govendor sync`  

##Building
  
In the main dir run:  
`revel build github.com/percona/qan-api <destination dir> prod`  
