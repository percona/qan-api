PRAGMA synchronous = OFF;
PRAGMA journal_mode = MEMORY;
BEGIN TRANSACTION;
CREATE TABLE `instances` (
  `instance_id` integer  NOT NULL PRIMARY KEY AUTOINCREMENT
,  `subsystem_id` integer  NOT NULL
,  `parent_uuid` char(32) NOT NULL
,  `uuid` char(32) NOT NULL
,  `name` varchar(100) NOT NULL
,  `dsn` varchar(500) NOT NULL
,  `distro` varchar(100) NOT NULL
,  `version` varchar(50) NOT NULL
,  `created` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
,  `deleted` timestamp NULL DEFAULT '1970-01-01 00:00:01'
,  UNIQUE (`uuid`)
,  UNIQUE (`name`,`subsystem_id`,`deleted`)
);
CREATE TABLE `query_classes` (
  `query_class_id` integer  NOT NULL PRIMARY KEY AUTOINCREMENT
,  `checksum` char(32) NOT NULL
,  `abstract` varchar(100) DEFAULT NULL
,  `fingerprint` varchar(5000) NOT NULL
,  `tables` text
,  `first_seen` timestamp NULL DEFAULT NULL
,  `last_seen` timestamp NULL DEFAULT NULL
,  `status` char(3) NOT NULL DEFAULT 'new'
,  UNIQUE (`checksum`)
);
CREATE TABLE `query_examples` (
  `query_class_id` integer  NOT NULL
,  `instance_id` integer  NOT NULL DEFAULT '0'
,  `period` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
,  `ts` timestamp NULL DEFAULT NULL
,  `db` varchar(255) NOT NULL DEFAULT ''
,  `Query_time` float NOT NULL DEFAULT '0'
,  `query` text NOT NULL
,  PRIMARY KEY (`query_class_id`,`instance_id`,`period`)
);
CREATE TABLE `agent_configs` (
  `agent_instance_id` integer  NOT NULL
,  `service` varchar(10) NOT NULL
,  `other_instance_id` integer  NOT NULL DEFAULT 0
,  `in_file` text NOT NULL
,  `running` text NOT NULL
,  `updated` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
,  PRIMARY KEY (`agent_instance_id`,`service`,`other_instance_id`)
);
END TRANSACTION;
