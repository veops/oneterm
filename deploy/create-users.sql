-- create database
CREATE DATABASE IF NOT EXISTS acl;
CREATE DATABASE IF NOT EXISTS oneterm;

-- create user 
CREATE USER 'oneterm'@'%' IDENTIFIED BY '123456';
CREATE USER 'acl'@'%' IDENTIFIED BY '123456';

-- grant privileges
GRANT ALL PRIVILEGES ON `oneterm`.* TO 'oneterm'@'%' WITH GRANT OPTION;
GRANT ALL PRIVILEGES ON `acl`.* TO 'acl'@'%';
