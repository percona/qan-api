SET NAMES 'utf8';
SET time_zone='+0:00';
SET foreign_key_checks=0;
SET unique_checks=0;

INSERT INTO instances (instance_id, parent_uuid, subsystem_id, uuid, name, dsn) VALUES
  (1,     '', 1, '101', 'db01-os',                                 NULL),
  (2,  '101', 2, '212', 'db01-agent',                              NULL),
  (3,  '101', 3, '313', 'db01-mysql', 'percona:percona@tcp(localhost)/');
