=================
Percona Datastore
=================

Percona Datastore is an open-source API for managing and monitoring MySQL. It is a static binary with only one external dependency: MySQL, where it stores data in a single schema: ``percona_datastore``. It runs on port 9001 (no SSL) as a normal, non-root user.

Install
=======

Requirements
------------

* Linux OS (Debian, Ubuntu, CentOS, Red Hat, etc.)
* MySQL/Percona Server 5.1 or newer
* Can run ``mysql`` without specifying user or password (set ``~/.my.cnf``)
* MySQL user ``qan-api@localhost`` exists or can be created
* Port 9001 open (TCP, inbound and outbound, HTTP and websocket)

Quick
-----

Install using the local MySQL instance:

.. code-block:: bash

    ./install

Schema ``percona_datastore`` and MySQL user ``qan-api`` are created. On success, the install script prints how to verify that the API is running.

Options
-------

The install script does not use command line options, it uses these environment variables:

=======     ===============
Env Var     Purpose
=======     ===============
BASEDIR     Where to install the datastore. All datastore-related files are stored in this directory.

LISTEN      `<IP>:<port>` to bind to and listen on (default: `0.0.0.0:9001`). The default is insecure and should be changed so that the API only listens on a private or trusted interface. Be sure that `HOSTNAME` resolves to this IP, else connections to the API will fail.

HOSTNAME    Hostname (or IP) that the datastore reports itself as for providing resource links. In theory, the only hard-coded API link is the root: ``http://<hostname>/``. The agent gets this link to discover other resource links, like ``http://<hostname>/instances`` for instances. Therefore, the datastore must report its hostname so clients can follow the links. If the datastore is not running behind a proxy (like nginx), an IP address can be used. Do not specify a port; use `LISTEN` to specify a port. Be sure that the hostname resolves to the `LISTEN` IP, else connections to the API will fail.

CREATE_DB   If "yes" (default), the ``percona_datastore`` schema is dropped and created if it does not already exist. Specify "force" to drop and create an existing schema. Specify "no" if you need to reinstall and preserve an existing schema.

CHECK_REQ   Check requirements before doing install (default: yes). If a check fails, installer exits 1.

START       Start API after installing (default: yes).
=======     ===============

Specify environment variables like:

.. code-block:: bash

    $ BASEDIR=/opt/percona/datastore CREATE_DB=no ./install

Run ``install help`` to list these environment variables and their defaults.

MySQL User
----------

The datastore requires a MySQL user will all privileges on the ``percona_datastore`` schema:

.. code-block:: sql

	GRANT ALL ON percona_datastore.* TO 'qan-api'@'localhost' IDENTIFIED BY <password>

During install,

* if the ``qan-api`` user exists, it’s used
* if the ``qan-api`` user does not exist, it’s created if the MySQL connection has "ALL PRIVILEGES" and "WITH GRANT OPTION" (i.e. you’re root or your ``~/.my.cnf`` specifies root credentials)
* else, you must create the user manually before installing using the grant statements above

The password is stored in ``templates/percona.my.cnf``. That file is removed after install.

Debug
-----

If the install fails, run it with "-x" (it’s just a Bash script):

.. code-block:: bash

    /bin/sh -x ./install

Copy all the output, then file a bug or contact Percona.

Operate
=======

The datastore uses a single directory, its "basedir", to store all files. The default basedir is ``/usr/local/percona/datastore``. These commands are relative to the basedir.

Start/Stop
----------

.. code-block:: bash

    $ percona-datastore start
    $ percona-datastore stop

Currently, the datastore does not ship with an init script for standard system process managers.

PID File
--------

Currently, the datastore does not use a PID file.

Log File
--------

``log/percona-datastore.log`` contains warnings and errors, if any. There are no settings or options (log level, etc.).

Version
-------

To check the version of the datastore while it's running:

.. code-block:: bash

    $ curl -s -I localhost:9001/ping | grep Version
    X-Percona-Datastore-Version: 1.0.0-20151125.b085563

As in the example above, the version can contain a ``-YYYYMMDD-rev`` development build suffix. *Development builds should not be used in production.*

Schema
======

It is best to use the API for all access to underlying data, but the schema (``percona_datastore`` by default) can be accessed manually if necessary. The following describes each table and column.

Note: foreign keys are not used.

agent_configs
-------------

This table contains configs from agents for agent internal services (data, log, etc.) and tools (QAN). The agent is the source of truth apropos its configs. When an agent connects to the API, the API sends a ``GetAllConfigs`` command and updates this table.

===============     ===========================================
Column              Purpose
===============     ===========================================
agent_instance_id   instance.instancd_id of the agent
service             agent, data, log, qan, etc. (internals parts of the agent)
other_instance_id   If service is a tool (e.g. qan), then this is the instance_id that the tool config applies to; else, the config is for some internal part of the agent (data, log, etc.)
config              JSON config, specific to each service
updated             Last time config was updated
===============     ===========================================

agent_log
---------

=============== ===========================================
Column          Purpose
=============== ===========================================
agent_log_id    Auto-inc identifier
instance_id     instance.instancd_id of the agent
sec             Unix timestamp of log event
nsec            Unix timestamp nanoseconds
level           Log level number
service         The part of the agent logging (many)
msg             The actual log entry
=============== ===========================================

instances
---------
Instances are central to everything because all data must be related to an instance. Instances are, as the name suggests, instances of some subsystem, as defined by ``subsystems``. So a MySQL instance (i.e. a single mysqld process) is an instance of the MySQL subsystem. There can be N-many instances of each subsystem, but all instances must be unique by ``name`` (and ``uuid``). Agents are also instances, even though the protocol defines an agent resource (``proto.Agent``).

=============== ===========================================
Column          Purpose
=============== ===========================================
instance_id     Auto-inc identifier
subsystem_id    ``subsystems.subsystem_id`` identifier
parent_uuid     UUID of parent instance
uuid            Primary identifier, does not change
name            Friendly identifier, user-configurable (e.g. hostname)
dsn             For accessible subsystems (currently just MySQL)
distro          Distribution of the instance software (e.g. Percona Server)
version         Version of the instance software (e.g. 5.6.29)
created         When instance was created
deleted         Set if instance was deleted, else zero date
=============== ===========================================

query_classes
-------------

This table contains all unique queries (classes) reported by all agents for QAN. A query class is defined by its fingerprint, which can be further reduced to an ID (checksum). A query class is unique regardless of db. Classes are collected and reported in global aggregates and individually (see the next two tables).

=============== ===========================================
Column          Purpose
=============== ===========================================
query_class_id  Auto-inc identifier
checksum        16-character hex checksum of fingerprint
abstract        SQL verb followed by table refs
fingerprint     Canonical form of query
tables          JSON containing table refs for real-time table info
first_seen      First time query class was seen
last_seen       Last time query class was seen
status          “new”, “reviewed”, “needs attention”
=============== ===========================================

query_global_metrics
--------------------

This table contains summarized query reports for all queries in a report period (i.e. all query class metrics having the same instance_id and start_ts). Individual query classes are compared to global values, joined by instance_id and start_ts, to determine what percentage the query class comprises of the total, i.e. to establish query rankings.

===============     ===========================================
Column              Purpose
===============     ===========================================
instance_id         instance_id of the MySQL server where data is from
start_ts            When data collection began
end_ts              When data collection stopped
run_time            How long data collection took (seconds)
total_query_count   Total number of queries executed during [start_ts, end_ts)
unique_query_count  Number of unique queries (classes) executed during [start_ts, end_ts)
rate_type           “session” if using Percona Server sampling
rate_limit          Sample rate if using Percona Server sampling
log_file            Slow query log file
log_file_size       Size of log_file when parsed
start_offset        File offset of log_file at start_ts
end_offset          File offset of log_file at end_ts
stop_ts             File offset of log_file where parsing stopped
<metrics>           The 100+ {metric}_{stat}, most only available from Percona Server
===============     ===========================================

query_class_metrics
-------------------

This table contains the long list of query metrics reported by agents for the top 200 query classes per report. It is denormalized for speed; normally, it would reference query_global_metrics.

=============== ===========================================
Column          Purpose
=============== ===========================================
query_class_id  Refers to query_classes.query_class_id
instancd_id     instance_id of the MySQL server where data is from
start_ts        When data collection began
end_ts          When data collection stopped
query_count     Number of times this query was executed during [start_ts, end_ts)
lrq_count       Number of queries < top 200 rolled into a special query class called "LRQ": Low Ranking Queries
<metrics>       The 100+ {metric}_{stat}, most only available from Percona Server, e.g. “Query_time_avg”, “Lock_time_median”, etc.
=============== ===========================================

query_examples
--------------

This table contains real examples of query classes if this feature is enabled in the QAN config (enabled by default). Only one example per hour is stored. The example with the greatest Query_time is kept. It is denormalized for speed. Real-time EXPLAIN uses query examples.

=============== ===========================================
Column          Purpose
=============== ===========================================
query_class_id  Refers to query_classes.query_class_id
instance_id     instance_id of the MySQL server where query is from
period          A day, e.g. 2015-07-01 00:00:00. one example per day
ts              Timestamp of the query (can be null)
db              Default database of the query (can be empty string)
Query_time      Query_time metric of the query
query           The full, actual query (as long as it fits in a TEXT column)
=============== ===========================================

