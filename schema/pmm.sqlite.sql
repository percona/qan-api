/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `instances` (
  `instance_id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `subsystem_id` int(10) unsigned NOT NULL,
  `parent_uuid` char(32) DEFAULT NULL,
  `uuid` char(32) NOT NULL,
  `name` varchar(100) CHARACTER SET utf8 NOT NULL,
  `dsn` varchar(500) CHARACTER SET utf8 DEFAULT NULL,
  `distro` varchar(100) CHARACTER SET utf8 DEFAULT NULL,
  `version` varchar(50) CHARACTER SET utf8 DEFAULT NULL,
  `created` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `deleted` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`instance_id`),
  UNIQUE KEY `uuid` (`uuid`),
  UNIQUE KEY `name` (`name`,`subsystem_id`,`deleted`)
) ENGINE=InnoDB AUTO_INCREMENT=15 DEFAULT CHARSET=latin1;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `query_classes` (
  `query_class_id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `checksum` char(32) NOT NULL,
  `abstract` varchar(100) DEFAULT NULL,
  `fingerprint` varchar(5000) NOT NULL,
  `tables` text,
  `first_seen` timestamp NULL DEFAULT NULL,
  `last_seen` timestamp NULL DEFAULT NULL,
  `status` char(3) NOT NULL DEFAULT 'new',
  PRIMARY KEY (`query_class_id`),
  UNIQUE KEY `checksum` (`checksum`)
) ENGINE=InnoDB AUTO_INCREMENT=565 DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `query_examples` (
  `query_class_id` int(10) unsigned NOT NULL,
  `instance_id` int(10) unsigned NOT NULL DEFAULT '0',
  `period` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `ts` timestamp NULL DEFAULT NULL,
  `db` varchar(255) NOT NULL DEFAULT '',
  `Query_time` float NOT NULL DEFAULT '0',
  `query` text NOT NULL,
  PRIMARY KEY (`query_class_id`,`instance_id`,`period`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;
