-- This user defined labels.
-- In Go code we will also have predefined labels ex: schema | user | server_host based

CREATE TABLE `labels` (
  `label_id` integer  NOT NULL PRIMARY KEY AUTOINCREMENT
,  `name` varchar(100) NOT NULL
,  `rule` varchar(100) NOT NULL -- $DB == 'db1' OR $USER == 'root'
,  `created` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
,  `deleted` timestamp NULL DEFAULT '1970-01-01 00:00:01'
,  UNIQUE (`label_id`)
);
