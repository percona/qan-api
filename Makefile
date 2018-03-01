RUN=docker run --rm -v ${PWD}:/go$(subst ${GOPATH},,${PWD}) -w /go$(subst ${GOPATH},,${PWD}) golang

all: build2 deploy2

build:
	GOOS=linux revel build github.com/percona/qan-api distro


deploy:
	(docker exec pmm-server supervisorctl stop qan-api || true) && \
        docker cp dist/qan-api pmm-server:/usr/sbin/percona-qan-api && \
	docker exec pmm-server supervisorctl start qan-api
	tput bel

log:
	docker exec pmm-server tail -f /var/log/qan-api.log

run-tests:
	docker run --name mysql-server -e  MYSQL_ALLOW_EMPTY_PASSWORD=true -p 3306:3306 -d mysql/mysql-server --secure-file-priv=''
	sleep 60
	docker exec mysql-server mysql -e "update mysql.user set host='%' where user = 'root'; flush privileges;"
	sleep 60
	# UPDATE_TEST_DATA=true go test -p 1 -v github.com/percona/qan-api/app/... -args -v 3 -logtostderr true
	go test -p 1 -v github.com/percona/qan-api/app/qan/...
	sleep 60
	docker rm -f mysql-server

build2:
	$(RUN) go build -tags api2

deploy2:
	(docker exec pmm-server supervisorctl stop qan-api || true) && \
        docker cp qan-api pmm-server:/usr/sbin/percona-qan-api && \
	docker exec pmm-server supervisorctl start qan-api
	tput bel

containers:
	docker create -v /opt/prometheus/data -v /opt/consul-data -v /var/lib/mysql -v /var/lib/grafana --name pmm-data percona/pmm-server:latest /bin/true
	docker run -d -p 80:80 --volumes-from pmm-data --name pmm-server --restart always perconalab/pmm-server:clickhouse

rm-containers:
	docker rm -f pmm-server pmm-data

gen:
	go-bindata -pkg migrations -o migrations/bindata.go -ignore=migrations/bindata.go migrations/...

ch:
	docker exec -ti pmm-server clickhouse-client
