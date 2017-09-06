# sudo docker exec pmm-server mysqldump pmm | mysql  pmm
#curl -LOs https://raw.githubusercontent.com/dumblob/mysql2sqlite/master/mysql2sqlite
#chmod +x mysql2sqlite 
rm -f pmm.db
mysqldump --no-data --compact  pmm2 instances query_classes query_examples | ./mysql2sqlite - | sqlite3 pmm.db
mysqldump --skip-extended-insert --compact pmm2 instances query_classes query_examples > toSQLite.data.sql
./mysql2sqlite toSQLite.sql | sqlite3 pmm.db

#sudo rm -f /var/lib/mysql-files/query_class_metrics.csv
#sudo ls -alih /var/lib/mysql-files/
clickhouse-client --port 9000 --query="CREATE DATABASE pmm"
clickhouse-client --port 9000 -n -m -d pmm < pmm.schema.ch.sql
mysql pmm2 < export_query_class_metrics_to_csv.sql
sudo cat /var/lib/mysql-files/query_class_metrics.csv | clickhouse-client --port 9000 --query="INSERT INTO pmm.query_class_metrics FORMAT CSV"