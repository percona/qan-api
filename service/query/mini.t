#!/usr/bin/perl

# Copyright (c) 2015, Percona LLC and/or its affiliates. All rights reserved.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>

use strict;
use warnings FATAL => 'all';
use English qw(-no_match_vars);
use Test::More;
use File::Basename;
use lib dirname (__FILE__);

require "mini.pl";

is(
   Percona::Query::Mini::mini("SELECT /*!40001 SQL_NO_CACHE */ * FROM `film`"),
   "SELECT film",
   'Distills mysqldump SELECTs to selects',
);

is(
   Percona::Query::Mini::mini("CALL foo(1, 2, 3)"),
   "CALL foo",
   'Distills stored procedure calls specially',
);

is(
   Percona::Query::Mini::mini(
      q{REPLACE /*foo.bar:3/3*/ INTO checksum.checksum (db, tbl, }
      .q{chunk, boundaries, this_cnt, this_crc) SELECT 'foo', 'bar', }
      .q{2 AS chunk_num, '`id` >= 2166633', COUNT(*) AS cnt, }
      .q{LOWER(CONV(BIT_XOR(CAST(CRC32(CONCAT_WS('#', `id`, `created_by`, }
      .q{`created_date`, `updated_by`, `updated_date`, `ppc_provider`, }
      .q{`account_name`, `provider_account_id`, `campaign_name`, }
      .q{`provider_campaign_id`, `adgroup_name`, `provider_adgroup_id`, }
      .q{`provider_keyword_id`, `provider_ad_id`, `foo`, `reason`, }
      .q{`foo_bar_bazz_id`, `foo_bar_baz`, CONCAT(ISNULL(`created_by`), }
      .q{ISNULL(`created_date`), ISNULL(`updated_by`), ISNULL(`updated_date`), }
      .q{ISNULL(`ppc_provider`), ISNULL(`account_name`), }
      .q{ISNULL(`provider_account_id`), ISNULL(`campaign_name`), }
      .q{ISNULL(`provider_campaign_id`), ISNULL(`adgroup_name`), }
      .q{ISNULL(`provider_adgroup_id`), ISNULL(`provider_keyword_id`), }
      .q{ISNULL(`provider_ad_id`), ISNULL(`foo`), ISNULL(`reason`), }
      .q{ISNULL(`foo_base_foo_id`), ISNULL(`fooe_foo_id`)))) AS UNSIGNED)), 10, }
      .q{16)) AS crc FROM `foo`.`bar` USE INDEX (`PRIMARY`) WHERE }
      .q{(`id` >= 2166633); }),
   'REPLACE SELECT checksum.checksum foo.bar',
   'Distills mk-table-checksum query',
);

is(
   Percona::Query::Mini::mini("use `foo`"),
   "USE",
   'distills use',
);

is(
   Percona::Query::Mini::mini("USE `foo`"),
   "USE",
   'distills USE',
);

is(
   Percona::Query::Mini::mini(q{delete foo.bar b from foo.bar b left join baz.bat c on a=b where nine>eight}),
   'DELETE foo.bar baz.bat',
   'distills and then collapses same tables',
);

is(
   Percona::Query::Mini::mini("select \n--bar\n t from foo"),
   "SELECT foo",
   'distills queries with comments and newslines'
);

# http://dev.mysql.com/doc/refman/5.0/en/select.html
is(
   Percona::Query::Mini::mini("select 1+1 from dual"),
   "SELECT dual",
   'distills FROM DUAL'
);

is(
   Percona::Query::Mini::mini("select null, 5.001, 5001. from foo"),
   "SELECT foo",
   "distills simple select",
);

is(
   Percona::Query::Mini::mini("select 'hello', '\nhello\n', \"hello\", '\\'' from foo"),
   "SELECT foo",
   "distills with quoted strings",
);

is(
   Percona::Query::Mini::mini("select foo_1 from foo_2_3"),
   'SELECT foo_?_?',
   'distills numeric table names',
);

is(
   Percona::Query::Mini::mini("insert into abtemp.coxed select foo.bar from foo"),
   'INSERT SELECT abtemp.coxed foo',
   'distills insert/select',
);

is(
   Percona::Query::Mini::mini('insert into foo(a, b, c) values(2, 4, 5)'),
   'INSERT foo',
   'distills value lists',
);

is(
   Percona::Query::Mini::mini('select 1 union select 2 union select 4'),
   'SELECT UNION',
   'distill unions together',
);

is(
   Percona::Query::Mini::mini(
      'delete from foo where bar = baz',
   ),
   'DELETE foo',
   'distills delete',
);

is(
   Percona::Query::Mini::mini('set timestamp=134'),
   'SET',
   'distills set',
);

is(
   Percona::Query::Mini::mini(
      'replace into foo(a, b, c) values(1, 3, 5) on duplicate key update foo=bar',
   ),
   'REPLACE UPDATE foo',
   'distills ODKU',
);

is(Percona::Query::Mini::mini(
   q{UPDATE GARDEN_CLUPL PL, GARDENJOB GC, APLTRACT_GARDENPLANT ABU SET }
   . q{GC.MATCHING_POT = 5, GC.LAST_GARDENPOT = 5, GC.LAST_NAME=}
   . q{'Rotary', GC.LAST_BUCKET='Pail', GC.LAST_UPDATE='2008-11-27 04:00:59'WHERE}
   . q{ PL.APLTRACT_GARDENPLANT_ID = GC.APLTRACT_GARDENPLANT_ID AND PL.}
   . q{APLTRACT_GARDENPLANT_ID = ABU.ID AND GC.MATCHING_POT = 0 AND GC.PERFORM_DIG=1}
   . q{ AND ABU.DIG = 6 AND ( ((SOIL-COST) > -80.0}
   . q{ AND BUGS < 60.0 AND (SOIL-COST) < 200.0) AND POTS < 10.0 )}),
   'UPDATE GARDEN_CLUPL GARDENJOB APLTRACT_GARDENPLANT',
   'distills where there is alias and comma-join',
);

is(
   Percona::Query::Mini::mini(q{SELECT STRAIGHT_JOIN distinct foo, bar FROM A, B, C}),
   'SELECT A B C',
   'distill with STRAIGHT_JOIN',
);

is (
   Percona::Query::Mini::mini(q{
REPLACE DELAYED INTO
`db1`.`tbl2`(`col1`,col2)
VALUES ('617653','2007-09-11')}),
   'REPLACE db?.tbl?',
   'distills replace-delayed',
);

is(
   Percona::Query::Mini::mini(
      'update foo inner join bar using(baz) set big=little',
   ),
   'UPDATE foo bar',
   'distills update-multi',
);

is(
   Percona::Query::Mini::mini('
update db2.tbl1 as p
   inner join (
      select p2.col1, p2.col2
      from db2.tbl1 as p2
         inner join db2.tbl3 as ba
            on p2.col1 = ba.tbl3
      where col4 = 0
      order by priority desc, col1, col2
      limit 10
   ) as chosen on chosen.col1 = p.col1
      and chosen.col2 = p.col2
   set p.col4 = 149945'),
   'UPDATE SELECT db?.tbl?',
   'distills complex subquery',
);

is(
   Percona::Query::Mini::mini(
      'replace into checksum.checksum select `last_update`, `foo` from foo.foo'),
   'REPLACE SELECT checksum.checksum foo.foo',
   'distill with reserved words');

is(Percona::Query::Mini::mini('SHOW STATUS'), 'SHOW STATUS', 'distill SHOW STATUS');

is(Percona::Query::Mini::mini('commit'), 'COMMIT', 'distill COMMIT');

is(Percona::Query::Mini::mini('FLUSH TABLES WITH READ LOCK'), 'FLUSH', 'distill FLUSH');

is(Percona::Query::Mini::mini('BEGIN'), 'BEGIN', 'distill BEGIN');

is(Percona::Query::Mini::mini('start'), 'START', 'distill START');

is(Percona::Query::Mini::mini('ROLLBACK'), 'ROLLBACK', 'distill ROLLBACK');

is(
   Percona::Query::Mini::mini(
      'insert into foo select * from bar join baz using (bat)',
   ),
   'INSERT SELECT foo bar baz',
   'distills insert select',
);

is(
   Percona::Query::Mini::mini('create database foo'),
   'CREATE DATABASE foo',
   'distills create database'
);
is(
   Percona::Query::Mini::mini('create table foo'),
   'CREATE TABLE foo',
   'distills create table'
);
is(
   Percona::Query::Mini::mini('alter database foo'),
   'ALTER DATABASE foo',
   'distills alter database'
);
is(
   Percona::Query::Mini::mini('alter table foo'),
   'ALTER TABLE foo',
   'distills alter table'
);
is(
   Percona::Query::Mini::mini('drop database foo'),
   'DROP DATABASE foo',
   'distills drop database'
);
is(
   Percona::Query::Mini::mini('drop table foo'),
   'DROP TABLE foo',
   'distills drop table'
);
is(
   Percona::Query::Mini::mini('rename database foo'),
   'RENAME DATABASE foo',
   'distills rename database'
);
is(
   Percona::Query::Mini::mini('rename table foo'),
   'RENAME TABLE foo',
   'distills rename table'
);
is(
   Percona::Query::Mini::mini('truncate table foo'),
   'TRUNCATE TABLE foo',
   'distills truncate table'
);

is(
   Percona::Query::Mini::mini(
      'update foo set bar=baz where bat=fiz',
   ),
   'UPDATE foo',
   'distills update',
);

# Issue 563: Lock tables is not distilled
is(
   Percona::Query::Mini::mini('LOCK TABLES foo WRITE'),
   'LOCK foo',
   'distills lock tables'
);
is(
   Percona::Query::Mini::mini('LOCK TABLES foo READ, bar WRITE'),
   'LOCK foo bar',
   'distills lock tables (2 tables)'
);
is(
   Percona::Query::Mini::mini('UNLOCK TABLES'),
   'UNLOCK',
   'distills unlock tables'
);

#  Issue 712: Queries not handled by "distill"
is(
   Percona::Query::Mini::mini('XA START 0x123'),
   'XA_START',
   'distills xa start'
);
is(
   Percona::Query::Mini::mini('XA PREPARE 0x123'),
   'XA_PREPARE',
   'distills xa prepare'
);
is(
   Percona::Query::Mini::mini('XA COMMIT 0x123'),
   'XA_COMMIT',
   'distills xa commit'
);
is(
   Percona::Query::Mini::mini('XA END 0x123'),
   'XA_END',
   'distills xa end'
);

is(
   Percona::Query::Mini::mini('prepare'),
   'PREPARE',
   'distills prepare'
);

is(
   Percona::Query::Mini::mini("/* mysql-connector-java-5.1-nightly-20090730 ( Revision: \${svn.Revision} ) */SHOW VARIABLES WHERE Variable_name ='language' OR Variable_name =
   'net_write_timeout' OR Variable_name = 'interactive_timeout' OR
   Variable_name = 'wait_timeout' OR Variable_name = 'character_set_client' OR
   Variable_name = 'character_set_connection' OR Variable_name =
   'character_set' OR Variable_name = 'character_set_server' OR Variable_name
   = 'tx_isolation' OR Variable_name = 'transaction_isolation' OR
   Variable_name = 'character_set_results' OR Variable_name = 'timezone' OR
   Variable_name = 'time_zone' OR Variable_name = 'system_time_zone' OR
   Variable_name = 'lower_case_table_names' OR Variable_name =
   'max_allowed_packet' OR Variable_name = 'net_buffer_length' OR
   Variable_name = 'sql_mode' OR Variable_name = 'query_cache_type' OR
   Variable_name = 'query_cache_size' OR Variable_name = 'init_connect'"),
   'SHOW VARIABLES',
   'distills /* comment */SHOW VARIABLES'
);

# This is a list of all the types of syntax for SHOW on
# http://dev.mysql.com/doc/refman/5.0/en/show.html
my %status_tests = (
   'SHOW BINARY LOGS'                           => 'SHOW BINARY LOGS',
   'SHOW BINLOG EVENTS in "log_name"'           => 'SHOW BINLOG EVENTS',
   'SHOW CHARACTER SET LIKE "pattern"'          => 'SHOW CHARACTER SET',
   'SHOW COLLATION WHERE "something"'           => 'SHOW COLLATION',
   'SHOW COLUMNS FROM tbl'                      => 'SHOW COLUMNS',
   'SHOW FULL COLUMNS FROM tbl'                 => 'SHOW COLUMNS',
   'SHOW COLUMNS FROM tbl in db'                => 'SHOW COLUMNS',
   'SHOW COLUMNS FROM tbl IN db LIKE "pattern"' => 'SHOW COLUMNS',
   'SHOW CREATE DATABASE db_name'               => 'SHOW CREATE DATABASE',
   'SHOW CREATE SCHEMA db_name'                 => 'SHOW CREATE DATABASE',
   'SHOW CREATE FUNCTION func'                  => 'SHOW CREATE FUNCTION',
   'SHOW CREATE PROCEDURE proc'                 => 'SHOW CREATE PROCEDURE',
   'SHOW CREATE TABLE tbl_name'                 => 'SHOW CREATE TABLE',
   'SHOW CREATE VIEW vw_name'                   => 'SHOW CREATE VIEW',
   'SHOW DATABASES'                             => 'SHOW DATABASES',
   'SHOW SCHEMAS'                               => 'SHOW DATABASES',
   'SHOW DATABASES LIKE "pattern"'              => 'SHOW DATABASES',
   'SHOW DATABASES WHERE foo=bar'               => 'SHOW DATABASES',
   'SHOW ENGINE ndb status'                     => 'SHOW NDB STATUS',
   'SHOW ENGINE innodb status'                  => 'SHOW INNODB STATUS',
   'SHOW ENGINES'                               => 'SHOW ENGINES',
   'SHOW STORAGE ENGINES'                       => 'SHOW ENGINES',
   'SHOW ERRORS'                                => 'SHOW ERRORS',
   'SHOW ERRORS limit 5'                        => 'SHOW ERRORS',
   'SHOW COUNT(*) ERRORS'                       => 'SHOW ERRORS',
   'SHOW FUNCTION CODE func'                    => 'SHOW FUNCTION CODE',
   'SHOW FUNCTION STATUS'                       => 'SHOW FUNCTION STATUS',
   'SHOW FUNCTION STATUS LIKE "pattern"'        => 'SHOW FUNCTION STATUS',
   'SHOW FUNCTION STATUS WHERE foo=bar'         => 'SHOW FUNCTION STATUS',
   'SHOW GRANTS'                                => 'SHOW GRANTS',
   'SHOW GRANTS FOR user@localhost'             => 'SHOW GRANTS',
   'SHOW INDEX'                                 => 'SHOW INDEX',
   'SHOW INDEXES'                               => 'SHOW INDEX',
   'SHOW KEYS'                                  => 'SHOW INDEX',
   'SHOW INDEX FROM tbl'                        => 'SHOW INDEX',
   'SHOW INDEX FROM tbl IN db'                  => 'SHOW INDEX',
   'SHOW INDEX IN tbl FROM db'                  => 'SHOW INDEX',
   'SHOW INNODB STATUS'                         => 'SHOW INNODB STATUS',
   'SHOW LOGS'                                  => 'SHOW LOGS',
   'SHOW MASTER STATUS'                         => 'SHOW MASTER STATUS',
   'SHOW MUTEX STATUS'                          => 'SHOW MUTEX STATUS',
   'SHOW OPEN TABLES'                           => 'SHOW OPEN TABLES',
   'SHOW OPEN TABLES FROM db'                   => 'SHOW OPEN TABLES',
   'SHOW OPEN TABLES IN db'                     => 'SHOW OPEN TABLES',
   'SHOW OPEN TABLES IN db LIKE "pattern"'      => 'SHOW OPEN TABLES',
   'SHOW OPEN TABLES IN db WHERE foo=bar'       => 'SHOW OPEN TABLES',
   'SHOW OPEN TABLES WHERE foo=bar'             => 'SHOW OPEN TABLES',
   'SHOW PRIVILEGES'                            => 'SHOW PRIVILEGES',
   'SHOW PROCEDURE CODE proc'                   => 'SHOW PROCEDURE CODE',
   'SHOW PROCEDURE STATUS'                      => 'SHOW PROCEDURE STATUS',
   'SHOW PROCEDURE STATUS LIKE "pattern"'       => 'SHOW PROCEDURE STATUS',
   'SHOW PROCEDURE STATUS WHERE foo=bar'        => 'SHOW PROCEDURE STATUS',
   'SHOW PROCESSLIST'                           => 'SHOW PROCESSLIST',
   'SHOW FULL PROCESSLIST'                      => 'SHOW PROCESSLIST',
   'SHOW PROFILE'                               => 'SHOW PROFILE',
   'SHOW PROFILES'                              => 'SHOW PROFILES',
   'SHOW PROFILES CPU FOR QUERY 1'              => 'SHOW PROFILES CPU',
   'SHOW SLAVE HOSTS'                           => 'SHOW SLAVE HOSTS',
   'SHOW SLAVE STATUS'                          => 'SHOW SLAVE STATUS',
   'SHOW STATUS'                                => 'SHOW STATUS',
   'SHOW GLOBAL STATUS'                         => 'SHOW GLOBAL STATUS',
   'SHOW SESSION STATUS'                        => 'SHOW STATUS',
   'SHOW STATUS LIKE "pattern"'                 => 'SHOW STATUS',
   'SHOW STATUS WHERE foo=bar'                  => 'SHOW STATUS',
   'SHOW TABLE STATUS'                          => 'SHOW TABLE STATUS',
   'SHOW TABLE STATUS FROM db_name'             => 'SHOW TABLE STATUS',
   'SHOW TABLE STATUS IN db_name'               => 'SHOW TABLE STATUS',
   'SHOW TABLE STATUS LIKE "pattern"'           => 'SHOW TABLE STATUS',
   'SHOW TABLE STATUS WHERE foo=bar'            => 'SHOW TABLE STATUS',
   'SHOW TABLES'                                => 'SHOW TABLES',
   'SHOW FULL TABLES'                           => 'SHOW TABLES',
   'SHOW TABLES FROM db'                        => 'SHOW TABLES',
   'SHOW TABLES IN db'                          => 'SHOW TABLES',
   'SHOW TABLES LIKE "pattern"'                 => 'SHOW TABLES',
   'SHOW TABLES FROM db LIKE "pattern"'         => 'SHOW TABLES',
   'SHOW TABLES WHERE foo=bar'                  => 'SHOW TABLES',
   'SHOW TRIGGERS'                              => 'SHOW TRIGGERS',
   'SHOW TRIGGERS IN db'                        => 'SHOW TRIGGERS',
   'SHOW TRIGGERS FROM db'                      => 'SHOW TRIGGERS',
   'SHOW TRIGGERS LIKE "pattern"'               => 'SHOW TRIGGERS',
   'SHOW TRIGGERS WHERE foo=bar'                => 'SHOW TRIGGERS',
   'SHOW VARIABLES'                             => 'SHOW VARIABLES',
   'SHOW GLOBAL VARIABLES'                      => 'SHOW GLOBAL VARIABLES',
   'SHOW SESSION VARIABLES'                     => 'SHOW VARIABLES',
   'SHOW VARIABLES LIKE "pattern"'              => 'SHOW VARIABLES',
   'SHOW VARIABLES WHERE foo=bar'               => 'SHOW VARIABLES',
   'SHOW WARNINGS'                              => 'SHOW WARNINGS',
   'SHOW WARNINGS LIMIT 5'                      => 'SHOW WARNINGS',
   'SHOW COUNT(*) WARNINGS'                     => 'SHOW WARNINGS',
   'SHOW COUNT ( *) WARNINGS'                   => 'SHOW WARNINGS',
);

foreach my $key ( keys %status_tests ) {
   is(Percona::Query::Mini::mini($key), $status_tests{$key}, "distills $key");
}

is(
   Percona::Query::Mini::mini('SHOW SLAVE STATUS'),
   'SHOW SLAVE STATUS',
   'distills SHOW SLAVE STATUS'
);
is(
   Percona::Query::Mini::mini('SHOW INNODB STATUS'),
   'SHOW INNODB STATUS',
   'distills SHOW INNODB STATUS'
);
is(
   Percona::Query::Mini::mini('SHOW CREATE TABLE'),
   'SHOW CREATE TABLE',
   'distills SHOW CREATE TABLE'
);

my @show = qw(COLUMNS GRANTS INDEX STATUS TABLES TRIGGERS WARNINGS);
foreach my $show ( @show ) {
   is(
      Percona::Query::Mini::mini("SHOW $show"),
      "SHOW $show",
      "distills SHOW $show"
   );
}

#  Issue 735: mk-query-digest doesn't distill query correctly
is( 
	Percona::Query::Mini::mini('SHOW /*!50002 GLOBAL */ STATUS'),
	'SHOW GLOBAL STATUS',
	"distills SHOW /*!50002 GLOBAL */ STATUS"
);

is( 
	Percona::Query::Mini::mini('SHOW /*!50002 ENGINE */ INNODB STATUS'),
	'SHOW INNODB STATUS',
	"distills SHOW INNODB STATUS"
);

is( 
	Percona::Query::Mini::mini('SHOW MASTER LOGS'),
	'SHOW MASTER LOGS',
	"distills SHOW MASTER LOGS"
);

is( 
	Percona::Query::Mini::mini('SHOW GLOBAL STATUS'),
	'SHOW GLOBAL STATUS',
	"distills SHOW GLOBAL STATUS"
);

is( 
	Percona::Query::Mini::mini('SHOW GLOBAL VARIABLES'),
	'SHOW GLOBAL VARIABLES',
	"distills SHOW GLOBAL VARIABLES"
);

is( 
	Percona::Query::Mini::mini('administrator command: Statistics'),
	'ADMIN STATISTICS',
	"distills ADMIN STATISTICS"
);

is( 
	Percona::Query::Mini::mini('administrator command: Quit'),
	'ADMIN QUIT',
	"distills ADMIN QUIT"
);

is( 
	Percona::Query::Mini::mini("administrator command: Ping\n"),
	'ADMIN PING',
	"distills ADMIN PING"
);

# Issue 781: mk-query-digest doesn't distill or extract tables properly
is( 
	Percona::Query::Mini::mini("SELECT `id` FROM (`field`) WHERE `id` = '10000016228434112371782015185031'"),
	'SELECT field',
	'distills SELECT clm from (`tbl`)'
);

is(  
	Percona::Query::Mini::mini("INSERT INTO (`jedi_forces`) (name, side, email) values ('Anakin Skywalker', 'jedi', 'anakin_skywalker_at_jedi.sw')"),
	'INSERT jedi_forces',
	'distills INSERT INTO (`tbl`)' 
);

is(  
	Percona::Query::Mini::mini("UPDATE (`jedi_forces`) set side = 'dark' and name = 'Lord Vader' where name = 'Anakin Skywalker'"),
	'UPDATE jedi_forces',
	'distills UPDATE (`tbl`)'
);

is(
	Percona::Query::Mini::mini("select c from (tbl1 JOIN tbl2 on (id)) where x=y"),
	'SELECT tbl?',
	'distills SELECT (t1 JOIN t2)'
);

is(
	Percona::Query::Mini::mini("insert into (t1) value('a')"),
	'INSERT t?',
	'distills INSERT (tbl)'
);

# Something that will (should) never distill.
is(
	Percona::Query::Mini::mini("-- how /*did*/ `THIS` #happen?"),
	'',
	'distills nonsense'
);

is(
	Percona::Query::Mini::mini("peek tbl poke db"),
	'PEEK (?)',
	'distills non-SQL'
);

# Issue 1176: mk-query-digest incorrectly distills queries with certain keywords

# I want to see first how this is handled.  It's correct because the query
# really does read from tables a and c; table b is just an alias.
is(
   Percona::Query::Mini::mini("select c from (select * from a) as b where exists (select * from c where id is null)"),
   "SELECT a c",
   "distills SELECT with subquery in FROM and WHERE"
);

is(
	Percona::Query::Mini::mini("select c from t where col='delete'"),
	'SELECT t',
   'distills SELECT with keyword as value (issue 1176)'
);

is(
   Percona::Query::Mini::mini('SELECT c, replace(foo, bar) FROM t WHERE col <> "insert"'),
   'SELECT t',
   'distills SELECT with REPLACE function (issue 1176)'
);

# LOAD DATA
# https://bugs.launchpad.net/percona-toolkit/+bug/821692
# INSERT and REPLACE without INTO
# https://bugs.launchpad.net/percona-toolkit/+bug/984053
is(
   Percona::Query::Mini::mini("LOAD DATA LOW_PRIORITY LOCAL INFILE 'file' INTO TABLE tbl"),
   "LOAD DATA tbl",
   "distill LOAD DATA (bug 821692)"
);

is(
   Percona::Query::Mini::mini("LOAD DATA LOW_PRIORITY LOCAL INFILE 'file' INTO TABLE `tbl`"),
   "LOAD DATA tbl",
   "distill LOAD DATA (bug 821692)"
);

is(
   Percona::Query::Mini::mini("insert ignore_bar (id) values (4029731)"),
   "INSERT ignore_bar",
   "distill INSERT without INTO (bug 984053)"
);

is(
   Percona::Query::Mini::mini("replace ignore_bar (id) values (4029731)"),
   "REPLACE ignore_bar",
   "distill REPLACE without INTO (bug 984053)"
);

# IF EXISTS
# https://bugs.launchpad.net/percona-toolkit/+bug/821690
is(
   Percona::Query::Mini::mini("DROP TABLE IF EXISTS foo"),
   "DROP TABLE foo",
   "distill DROP TABLE IF EXISTS foo (bug 821690)"
);

is(
   Percona::Query::Mini::mini("CREATE TABLE IF NOT EXISTS foo"),
   "CREATE TABLE foo",
   "distill CREATE TABLE IF NOT EXISTS foo",
);

is(
   Percona::Query::Mini::mini("describe `roles`"),
   "DESCRIBE roles",
   "distill describe `roles`"
);

is(
   Percona::Query::Mini::mini("describe roles"),
   "DESCRIBE roles",
   "distill describe roles"
);

is(
   Percona::Query::Mini::mini("desc `roles`"),
   "DESCRIBE roles",
   "distill desc `roles`"
);

is(
   Percona::Query::Mini::mini("set global slow_query_log=on"),
   "SET",
   "set global slow_query_log=on" 
);

is(
   Percona::Query::Mini::mini("\n"),
   "",
   "newline"
);

is(
   Percona::Query::Mini::mini("show /*!? global */ status"),
   "SHOW GLOBAL STATUS",
   "show /*!? global */ status"
);

is(
   Percona::Query::Mini::mini("init db"),
   "INIT DB",
   "init db"
);

is(
   Percona::Query::Mini::mini("close stmt"),
   "CLOSE STMT",
   "close stmt"
);

is(
   Percona::Query::Mini::mini("reset stmt"),
   "RESET STMT",
   "reset stmt"
);

is(
   Percona::Query::Mini::mini("long data"),
   "LONG DATA",
   "long data"
);

is(
   Percona::Query::Mini::mini("register slave"),
   "REGISTER SLAVE",
   "register slave"
);

is(
   Percona::Query::Mini::mini("select \@\@session.sql_mode"),
   "SELECT",
   "select \@\@session.sql_mode",
);

is(
   Percona::Query::Mini::mini("select found_rows()"),
   "SELECT",
   "select found_rows()",
);

is(
   Percona::Query::Mini::mini("start transaction"),
   "START",
   "start transaction",
);

is(
   Percona::Query::Mini::mini("checksum table foo.bar"),
   "CHECKSUM foo.bar",
   "checksum table"
);

is(
   Percona::Query::Mini::mini("grant all privileges on `db`.* to ?@? identified by password ?"),
   "GRANT",
   "grant",
);

is(
   Percona::Query::Mini::mini("revoke all privileges from `db`.* to ?@?"),
   "REVOKE",
   "revoke",
);

# Savepoint
is(
   Percona::Query::Mini::mini("savepoint x"),
   "SAVEPOINT",
   "savepoint x",
);
is(
   Percona::Query::Mini::mini("rollback to x"),
   "ROLLBACK",
   "rollback to x",
);
is(
   Percona::Query::Mini::mini("release savepoint x"),
   "RELEASE SAVEPOINT",
   "release savepoint x",
);

# KILL
is(
   Percona::Query::Mini::mini("kill ?"),
   "KILL",
   "kill query",
);
is(
   Percona::Query::Mini::mini("kill connection ?"),
   "KILL CONNECTION",
   "kill query",
);
is(
   Percona::Query::Mini::mini("kill query ?"),
   "KILL QUERY",
   "kill query",
);

# EXPLAIN
is(
   Percona::Query::Mini::mini("EXPLAIN select * from foo"),
   "EXPLAIN",
   "explain select"
);

# ###########################################################################
# Performance schema digests
# ###########################################################################

is(
   Percona::Query::Mini::mini("TRUNCATE `performance_schema` . `events_statements_summary_by_digest` "),
   "TRUNCATE performance_schema.events_statements_summary_by_digest",
   "pfs: truncate `db` . `table`"
);

is(
   Percona::Query::Mini::mini("SELECT * FROM `mysql` . `user` "),
   "SELECT mysql.user",
   "pfs: SELECT * FROM `mysql` . `user`",
);

is(
   Percona::Query::Mini::mini("BEGIN\n"),
   "BEGIN",
   "pfs: BEGIN\\n"
);

is(
   Percona::Query::Mini::mini("OPTIMIZE TABLE `o2408`.`agent_log`\n"),
   "OPTIMIZE `o2408`.`agent_log`",
   "distills OPTIMIZE"
);

done_testing;
