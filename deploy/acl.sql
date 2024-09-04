-- MySQL dump 10.13  Distrib 8.2.0, for Linux (x86_64)
--
-- Host: localhost    Database: acl
-- ------------------------------------------------------
-- Server version	8.2.0

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!50503 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Table structure for table `acl_apps`
--

USE acl;

DROP TABLE IF EXISTS `acl_apps`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `acl_apps` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `description` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `app_id` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `secret_key` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  PRIMARY KEY (`id`),
  KEY `ix_acl_apps_name` (`name`),
  KEY `ix_acl_apps_deleted` (`deleted`)
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `acl_apps`
--

LOCK TABLES `acl_apps` WRITE;
/*!40000 ALTER TABLE `acl_apps` DISABLE KEYS */;
INSERT INTO `acl_apps` VALUES (NULL,0,'2024-01-31 17:33:25',NULL,1,'backend','backend','c726e077650546ceaa73032025b56ca9','MdP8lYfjBzHuLDC@qhxK2nt~4ZAXpGN*'),(NULL,0,'2024-01-31 17:33:48',NULL,2,'acl',NULL,NULL,NULL),('2024-01-31 17:56:04',1,'2024-01-31 17:38:41','2024-01-31 17:56:04',3,'oneterm',NULL,NULL,NULL),(NULL,0,'2024-01-31 17:56:13',NULL,4,'oneterm','oneterm','5867e079dfd1437e9ae07576ab24b391','2qlTA4z@#KyigJLYHGrev?0WD6hjX*8E');
/*!40000 ALTER TABLE `acl_apps` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `acl_audit_login_logs`
--

DROP TABLE IF EXISTS `acl_audit_login_logs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `acl_audit_login_logs` (
  `created_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `username` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `channel` enum('web','api') CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `ip` varchar(15) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `browser` varchar(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `description` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `is_ok` tinyint(1) DEFAULT NULL,
  `login_at` datetime DEFAULT NULL,
  `logout_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_acl_audit_login_logs_username` (`username`),
  KEY `ix_acl_audit_login_logs_created_at` (`created_at`)
) ENGINE=InnoDB AUTO_INCREMENT=14 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `acl_audit_login_logs`
--

LOCK TABLES `acl_audit_login_logs` WRITE;
/*!40000 ALTER TABLE `acl_audit_login_logs` DISABLE KEYS */;
INSERT INTO `acl_audit_login_logs` VALUES ('2024-01-31 17:34:01',1,'admin','web','192.168.65.74','Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36','login successful',1,'2024-01-31 17:34:01',NULL),('2024-01-31 17:35:36',2,'admin','web','192.168.65.81','Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36','login successful',1,'2024-01-31 17:35:36',NULL),('2024-01-31 17:35:36',3,'admin','web','192.168.65.74','Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36','login successful',1,'2024-01-31 17:35:36',NULL),('2024-01-31 17:36:01',4,'admin','web','192.168.65.81','Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36','login successful',1,'2024-01-31 17:36:01',NULL),('2024-01-31 17:37:48',5,'admin','web','192.168.65.81','Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36','login successful',1,'2024-01-31 17:37:48',NULL),('2024-01-31 17:38:25',6,'admin','web','192.168.65.74','Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36','login successful',1,'2024-01-31 17:38:25',NULL),('2024-01-31 17:38:45',7,'admin','web','192.168.65.81','Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36','login successful',1,'2024-01-31 17:38:45',NULL),('2024-01-31 17:38:51',8,'admin','web','192.168.65.74','Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36','login successful',1,'2024-01-31 17:38:51',NULL),('2024-01-31 17:40:29',9,'admin','web','192.168.29.75','Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/111.0.0.0 Safari/537.36','login successful',1,'2024-01-31 17:40:29',NULL),('2024-01-31 17:41:24',10,'admin','web','192.168.65.51','Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36','login successful',1,'2024-01-31 17:41:24',NULL),('2024-01-31 17:41:39',11,'admin','web','192.168.65.80','Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36','login successful',1,'2024-01-31 17:41:39',NULL),('2024-01-31 17:42:50',12,'admin','web','192.168.65.64','Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36','login successful',1,'2024-01-31 17:42:50','2024-01-31 17:57:35'),('2024-02-01 10:22:36',13,'admin','web','192.168.65.74','Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36','login successful',1,'2024-02-01 10:22:36',NULL);
/*!40000 ALTER TABLE `acl_audit_login_logs` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `acl_audit_permission_logs`
--

DROP TABLE IF EXISTS `acl_audit_permission_logs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `acl_audit_permission_logs` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `app_id` int DEFAULT NULL,
  `operate_uid` int DEFAULT NULL COMMENT '操作人uid',
  `operate_type` varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '操作类型',
  `rid` int DEFAULT NULL COMMENT '角色id',
  `resource_type_id` int DEFAULT NULL COMMENT '资源类型id',
  `resource_ids` json DEFAULT NULL COMMENT '资源',
  `group_ids` json DEFAULT NULL COMMENT '资源组',
  `permission_ids` json DEFAULT NULL COMMENT '权限',
  `source` varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '来源',
  PRIMARY KEY (`id`),
  KEY `ix_acl_audit_permission_logs_operate_type` (`operate_type`),
  KEY `ix_acl_audit_permission_logs_app_id` (`app_id`),
  KEY `ix_acl_audit_permission_logs_rid` (`rid`),
  KEY `ix_acl_audit_permission_logs_deleted` (`deleted`),
  KEY `ix_acl_audit_permission_logs_resource_type_id` (`resource_type_id`),
  KEY `ix_acl_audit_permission_logs_operate_uid` (`operate_uid`)
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `acl_audit_permission_logs`
--

LOCK TABLES `acl_audit_permission_logs` WRITE;
/*!40000 ALTER TABLE `acl_audit_permission_logs` DISABLE KEYS */;
INSERT INTO `acl_audit_permission_logs` VALUES (NULL,0,'2024-01-31 17:37:41',NULL,1,1,NULL,'grant',1,1,'[1]','[]','[1, 2, 3, 4]','acl'),(NULL,0,'2024-01-31 17:37:41',NULL,2,1,NULL,'grant',1,1,'[2]','[]','[1, 2, 3, 4]','acl'),(NULL,0,'2024-01-31 17:37:41',NULL,3,1,NULL,'grant',1,1,'[3]','[]','[1, 2, 3, 4]','acl');
/*!40000 ALTER TABLE `acl_audit_permission_logs` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `acl_audit_resource_logs`
--

DROP TABLE IF EXISTS `acl_audit_resource_logs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `acl_audit_resource_logs` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `app_id` int DEFAULT NULL,
  `operate_uid` int DEFAULT NULL COMMENT '操作人uid',
  `operate_type` varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '操作类型',
  `scope` varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '范围',
  `link_id` int DEFAULT NULL COMMENT '资源名',
  `origin` json DEFAULT NULL COMMENT '原始数据',
  `current` json DEFAULT NULL COMMENT '当前数据',
  `extra` json DEFAULT NULL COMMENT '权限名',
  `source` varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '来源',
  PRIMARY KEY (`id`),
  KEY `ix_acl_audit_resource_logs_link_id` (`link_id`),
  KEY `ix_acl_audit_resource_logs_operate_uid` (`operate_uid`),
  KEY `ix_acl_audit_resource_logs_app_id` (`app_id`),
  KEY `ix_acl_audit_resource_logs_operate_type` (`operate_type`),
  KEY `ix_acl_audit_resource_logs_deleted` (`deleted`)
) ENGINE=InnoDB AUTO_INCREMENT=26 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `acl_audit_resource_logs`
--

LOCK TABLES `acl_audit_resource_logs` WRITE;
/*!40000 ALTER TABLE `acl_audit_resource_logs` DISABLE KEYS */;
INSERT INTO `acl_audit_resource_logs` VALUES (NULL,0,'2024-01-31 17:33:25',NULL,1,1,NULL,'create','app',1,'{}','{\"id\": 1, \"name\": \"backend\", \"app_id\": \"c726e077650546ceaa73032025b56ca9\", \"deleted\": false, \"created_at\": \"2024-01-31 17:33:25\", \"deleted_at\": null, \"secret_key\": \"MdP8lYfjBzHuLDC@qhxK2nt~4ZAXpGN*\", \"updated_at\": null, \"description\": \"backend\"}','{}','acl'),(NULL,0,'2024-01-31 17:33:25',NULL,2,1,NULL,'create','resource_type',1,'{}','{\"id\": 1, \"name\": \"操作权限\", \"app_id\": 1, \"deleted\": false, \"created_at\": \"2024-01-31 17:33:25\", \"deleted_at\": null, \"updated_at\": null, \"description\": \"\"}','{\"permission_ids\": {\"origin\": [], \"current\": [1, 2, 3, 4]}}','acl'),(NULL,0,'2024-01-31 17:33:26',NULL,3,1,NULL,'create','resource',1,'{}','{\"id\": 1, \"uid\": null, \"name\": \"公司信息\", \"app_id\": 1, \"deleted\": false, \"created_at\": \"2024-01-31 17:33:25\", \"deleted_at\": null, \"updated_at\": null, \"resource_type_id\": 1}','{}','acl'),(NULL,0,'2024-01-31 17:33:26',NULL,4,1,NULL,'create','resource',2,'{}','{\"id\": 2, \"uid\": null, \"name\": \"公司架构\", \"app_id\": 1, \"deleted\": false, \"created_at\": \"2024-01-31 17:33:26\", \"deleted_at\": null, \"updated_at\": null, \"resource_type_id\": 1}','{}','acl'),(NULL,0,'2024-01-31 17:33:26',NULL,5,1,NULL,'create','resource',3,'{}','{\"id\": 3, \"uid\": null, \"name\": \"通知设置\", \"app_id\": 1, \"deleted\": false, \"created_at\": \"2024-01-31 17:33:26\", \"deleted_at\": null, \"updated_at\": null, \"resource_type_id\": 1}','{}','acl'),(NULL,0,'2024-01-31 17:41:59',NULL,6,3,1,'create','resource_type',2,'{}','{\"id\": 2, \"name\": \"account\", \"app_id\": 3, \"deleted\": false, \"created_at\": \"2024-01-31 17:41:59\", \"deleted_at\": null, \"updated_at\": null, \"description\": \"\"}','{\"permission_ids\": {\"origin\": [], \"current\": [5, 6, 7, 8]}}','acl'),(NULL,0,'2024-01-31 17:42:14',NULL,7,3,1,'create','resource_type',3,'{}','{\"id\": 3, \"name\": \"asset\", \"app_id\": 3, \"deleted\": false, \"created_at\": \"2024-01-31 17:42:14\", \"deleted_at\": null, \"updated_at\": null, \"description\": \"\"}','{\"permission_ids\": {\"origin\": [], \"current\": [9, 10, 11, 12]}}','acl'),(NULL,0,'2024-01-31 17:42:29',NULL,8,3,1,'create','resource_type',4,'{}','{\"id\": 4, \"name\": \"command\", \"app_id\": 3, \"deleted\": false, \"created_at\": \"2024-01-31 17:42:29\", \"deleted_at\": null, \"updated_at\": null, \"description\": \"\"}','{\"permission_ids\": {\"origin\": [], \"current\": [13, 14, 15, 16]}}','acl'),(NULL,0,'2024-01-31 17:42:44',NULL,9,3,1,'create','resource_type',5,'{}','{\"id\": 5, \"name\": \"gateway\", \"app_id\": 3, \"deleted\": false, \"created_at\": \"2024-01-31 17:42:44\", \"deleted_at\": null, \"updated_at\": null, \"description\": \"\"}','{\"permission_ids\": {\"origin\": [], \"current\": [17, 18, 19, 20]}}','acl'),(NULL,0,'2024-01-31 17:42:59',NULL,10,3,1,'create','resource_type',6,'{}','{\"id\": 6, \"name\": \"menu\", \"app_id\": 3, \"deleted\": false, \"created_at\": \"2024-01-31 17:42:59\", \"deleted_at\": null, \"updated_at\": null, \"description\": \"\"}','{\"permission_ids\": {\"origin\": [], \"current\": [21, 22, 23, 24]}}','acl'),(NULL,0,'2024-01-31 17:43:14',NULL,11,3,1,'create','resource_type',7,'{}','{\"id\": 7, \"name\": \"authorization\", \"app_id\": 3, \"deleted\": false, \"created_at\": \"2024-01-31 17:43:14\", \"deleted_at\": null, \"updated_at\": null, \"description\": \"\"}','{\"permission_ids\": {\"origin\": [], \"current\": [25, 26, 27, 28]}}','acl'),(NULL,0,'2024-01-31 17:43:46',NULL,12,3,1,'create','resource',4,'{}','{\"id\": 4, \"uid\": 1, \"name\": \"仪表盘\", \"app_id\": 3, \"deleted\": false, \"created_at\": \"2024-01-31 17:43:46\", \"deleted_at\": null, \"updated_at\": null, \"resource_type_id\": 6}','{}','acl'),(NULL,0,'2024-01-31 17:43:52',NULL,13,3,1,'create','resource',5,'{}','{\"id\": 5, \"uid\": 1, \"name\": \"资产管理\", \"app_id\": 3, \"deleted\": false, \"created_at\": \"2024-01-31 17:43:52\", \"deleted_at\": null, \"updated_at\": null, \"resource_type_id\": 6}','{}','acl'),(NULL,0,'2024-01-31 17:43:57',NULL,14,3,1,'create','resource',6,'{}','{\"id\": 6, \"uid\": 1, \"name\": \"资产列表\", \"app_id\": 3, \"deleted\": false, \"created_at\": \"2024-01-31 17:43:57\", \"deleted_at\": null, \"updated_at\": null, \"resource_type_id\": 6}','{}','acl'),(NULL,0,'2024-01-31 17:44:02',NULL,15,3,1,'create','resource',7,'{}','{\"id\": 7, \"uid\": 1, \"name\": \"网关列表\", \"app_id\": 3, \"deleted\": false, \"created_at\": \"2024-01-31 17:44:02\", \"deleted_at\": null, \"updated_at\": null, \"resource_type_id\": 6}','{}','acl'),(NULL,0,'2024-01-31 17:44:08',NULL,16,3,1,'create','resource',8,'{}','{\"id\": 8, \"uid\": 1, \"name\": \"账号列表\", \"app_id\": 3, \"deleted\": false, \"created_at\": \"2024-01-31 17:44:08\", \"deleted_at\": null, \"updated_at\": null, \"resource_type_id\": 6}','{}','acl'),(NULL,0,'2024-01-31 17:44:14',NULL,17,3,1,'create','resource',9,'{}','{\"id\": 9, \"uid\": 1, \"name\": \"命令过滤\", \"app_id\": 3, \"deleted\": false, \"created_at\": \"2024-01-31 17:44:14\", \"deleted_at\": null, \"updated_at\": null, \"resource_type_id\": 6}','{}','acl'),(NULL,0,'2024-01-31 17:44:20',NULL,18,3,1,'create','resource',10,'{}','{\"id\": 10, \"uid\": 1, \"name\": \"会话审计\", \"app_id\": 3, \"deleted\": false, \"created_at\": \"2024-01-31 17:44:20\", \"deleted_at\": null, \"updated_at\": null, \"resource_type_id\": 6}','{}','acl'),(NULL,0,'2024-01-31 17:44:24',NULL,19,3,1,'create','resource',11,'{}','{\"id\": 11, \"uid\": 1, \"name\": \"在线会话\", \"app_id\": 3, \"deleted\": false, \"created_at\": \"2024-01-31 17:44:24\", \"deleted_at\": null, \"updated_at\": null, \"resource_type_id\": 6}','{}','acl'),(NULL,0,'2024-01-31 17:44:29',NULL,20,3,1,'create','resource',12,'{}','{\"id\": 12, \"uid\": 1, \"name\": \"历史会话\", \"app_id\": 3, \"deleted\": false, \"created_at\": \"2024-01-31 17:44:29\", \"deleted_at\": null, \"updated_at\": null, \"resource_type_id\": 6}','{}','acl'),(NULL,0,'2024-01-31 17:44:33',NULL,21,3,1,'create','resource',13,'{}','{\"id\": 13, \"uid\": 1, \"name\": \"日志审计\", \"app_id\": 3, \"deleted\": false, \"created_at\": \"2024-01-31 17:44:33\", \"deleted_at\": null, \"updated_at\": null, \"resource_type_id\": 6}','{}','acl'),(NULL,0,'2024-01-31 17:44:38',NULL,22,3,1,'create','resource',14,'{}','{\"id\": 14, \"uid\": 1, \"name\": \"登录日志\", \"app_id\": 3, \"deleted\": false, \"created_at\": \"2024-01-31 17:44:38\", \"deleted_at\": null, \"updated_at\": null, \"resource_type_id\": 6}','{}','acl'),(NULL,0,'2024-01-31 17:44:45',NULL,23,3,1,'create','resource',15,'{}','{\"id\": 15, \"uid\": 1, \"name\": \"操作日志\", \"app_id\": 3, \"deleted\": false, \"created_at\": \"2024-01-31 17:44:45\", \"deleted_at\": null, \"updated_at\": null, \"resource_type_id\": 6}','{}','acl'),(NULL,0,'2024-01-31 17:56:04',NULL,24,3,1,'delete','app',3,'{\"id\": 3, \"name\": \"oneterm\", \"app_id\": null, \"deleted\": false, \"created_at\": \"2024-01-31 17:38:41\", \"deleted_at\": null, \"secret_key\": null, \"updated_at\": null, \"description\": null}','{}','{}','acl'),(NULL,0,'2024-01-31 17:56:13',NULL,25,4,1,'create','app',4,'{}','{\"id\": 4, \"name\": \"oneterm\", \"app_id\": \"5867e079dfd1437e9ae07576ab24b391\", \"deleted\": false, \"created_at\": \"2024-01-31 17:56:13\", \"deleted_at\": null, \"secret_key\": \"2qlTA4z@#KyigJLYHGrev?0WD6hjX*8E\", \"updated_at\": null, \"description\": \"oneterm\"}','{}','acl');
/*!40000 ALTER TABLE `acl_audit_resource_logs` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `acl_audit_role_logs`
--

DROP TABLE IF EXISTS `acl_audit_role_logs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `acl_audit_role_logs` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `app_id` int DEFAULT NULL,
  `operate_uid` int DEFAULT NULL COMMENT '操作人uid',
  `operate_type` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '操作类型',
  `scope` varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '范围',
  `link_id` int DEFAULT NULL COMMENT '资源id',
  `origin` json DEFAULT NULL COMMENT '原始数据',
  `current` json DEFAULT NULL COMMENT '当前数据',
  `extra` json DEFAULT NULL COMMENT '其他内容',
  `source` varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '来源',
  PRIMARY KEY (`id`),
  KEY `ix_acl_audit_role_logs_link_id` (`link_id`),
  KEY `ix_acl_audit_role_logs_app_id` (`app_id`),
  KEY `ix_acl_audit_role_logs_operate_uid` (`operate_uid`),
  KEY `ix_acl_audit_role_logs_operate_type` (`operate_type`),
  KEY `ix_acl_audit_role_logs_deleted` (`deleted`)
) ENGINE=InnoDB AUTO_INCREMENT=8 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `acl_audit_role_logs`
--

LOCK TABLES `acl_audit_role_logs` WRITE;
/*!40000 ALTER TABLE `acl_audit_role_logs` DISABLE KEYS */;
INSERT INTO `acl_audit_role_logs` VALUES (NULL,0,'2024-01-31 17:33:48',NULL,1,NULL,NULL,'create','role',1,'{}','{\"id\": 1, \"key\": \"948a8e6eaa724ff9b4d7ccfbbae37b6e\", \"uid\": 1, \"name\": \"admin\", \"app_id\": null, \"deleted\": false, \"created_at\": \"2024-01-31 17:33:48\", \"deleted_at\": null, \"updated_at\": null, \"is_app_admin\": false}','{}','acl'),(NULL,0,'2024-01-31 17:33:48',NULL,2,NULL,NULL,'create','user',1,'{}','{\"key\": \"a5704726392648b7b5a15cc39091a166\", \"uid\": 1, \"block\": false, \"email\": \"admin@veops.cn\", \"wx_id\": null, \"avatar\": null, \"mobile\": null, \"catalog\": null, \"deleted\": false, \"nickname\": \"admin\", \"username\": \"admin\", \"deleted_at\": null, \"department\": null, \"last_login\": \"2024-01-31 09:33:48\", \"date_joined\": \"2024-01-31 09:33:48\", \"employee_id\": \"0001\", \"has_logined\": false}','{}','acl'),(NULL,0,'2024-01-31 17:33:48',NULL,3,NULL,NULL,'create','role',2,'{}','{\"id\": 2, \"key\": \"7b845b56a0bb41bc9b1f96dc71dfb00d\", \"uid\": null, \"name\": \"acl_admin\", \"app_id\": null, \"deleted\": false, \"created_at\": \"2024-01-31 17:33:48\", \"deleted_at\": null, \"updated_at\": null, \"is_app_admin\": true}','{}','acl'),(NULL,0,'2024-01-31 17:33:48',NULL,4,2,NULL,'role_relation_add','role_relation',2,'{}','{}','{\"child_ids\": [1], \"parent_ids\": [2]}','acl'),(NULL,0,'2024-01-31 17:59:30',NULL,5,3,1,'create','role',3,'{}','{\"id\": 3, \"key\": \"7c4404b8233a431b8be70ccf39a96e5d\", \"uid\": null, \"name\": \"oneterm_admin\", \"app_id\": 3, \"deleted\": false, \"created_at\": \"2024-01-31 17:59:30\", \"deleted_at\": null, \"updated_at\": null, \"is_app_admin\": true}','{}','acl'),(NULL,0,'2024-02-01 10:24:55',NULL,6,NULL,1,'update','role',1,'{\"id\": 1, \"key\": \"948a8e6eaa724ff9b4d7ccfbbae37b6e\", \"uid\": 1, \"name\": \"admin\", \"app_id\": null, \"deleted\": false, \"created_at\": \"2024-01-31 17:33:48\", \"deleted_at\": null, \"updated_at\": null, \"is_app_admin\": false}','{\"id\": 1, \"key\": \"948a8e6eaa724ff9b4d7ccfbbae37b6e\", \"uid\": 1, \"name\": \"admin\", \"app_id\": null, \"deleted\": false, \"created_at\": \"2024-01-31 17:33:48\", \"deleted_at\": null, \"updated_at\": null, \"is_app_admin\": false}','{}','acl'),(NULL,0,'2024-02-01 10:24:55',NULL,7,4,1,'role_relation_add','role_relation',3,'{}','{}','{\"child_ids\": [1], \"parent_ids\": [3]}','acl');
/*!40000 ALTER TABLE `acl_audit_role_logs` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `acl_audit_trigger_logs`
--

DROP TABLE IF EXISTS `acl_audit_trigger_logs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `acl_audit_trigger_logs` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `app_id` int DEFAULT NULL,
  `trigger_id` int DEFAULT NULL COMMENT 'trigger',
  `operate_uid` int DEFAULT NULL COMMENT '操作人uid',
  `operate_type` varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '操作类型',
  `origin` json DEFAULT NULL COMMENT '原始数据',
  `current` json DEFAULT NULL COMMENT '当前数据',
  `extra` json DEFAULT NULL COMMENT '权限名',
  `source` varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '来源',
  PRIMARY KEY (`id`),
  KEY `ix_acl_audit_trigger_logs_trigger_id` (`trigger_id`),
  KEY `ix_acl_audit_trigger_logs_app_id` (`app_id`),
  KEY `ix_acl_audit_trigger_logs_operate_uid` (`operate_uid`),
  KEY `ix_acl_audit_trigger_logs_deleted` (`deleted`),
  KEY `ix_acl_audit_trigger_logs_operate_type` (`operate_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `acl_audit_trigger_logs`
--

LOCK TABLES `acl_audit_trigger_logs` WRITE;
/*!40000 ALTER TABLE `acl_audit_trigger_logs` DISABLE KEYS */;
/*!40000 ALTER TABLE `acl_audit_trigger_logs` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `acl_operation_records`
--

DROP TABLE IF EXISTS `acl_operation_records`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `acl_operation_records` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `app` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `rolename` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `operate` enum('0','4','3','2','1') CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `obj` json DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_acl_operation_records_deleted` (`deleted`),
  KEY `ix_acl_operation_records_app` (`app`),
  KEY `ix_acl_operation_records_rolename` (`rolename`)
) ENGINE=InnoDB AUTO_INCREMENT=35 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `acl_operation_records`
--

LOCK TABLES `acl_operation_records` WRITE;
/*!40000 ALTER TABLE `acl_operation_records` DISABLE KEYS */;
INSERT INTO `acl_operation_records` VALUES (NULL,0,'2024-01-31 17:37:49',NULL,1,'backend','admin','1','[\"resources\"]'),(NULL,0,'2024-01-31 17:38:25',NULL,2,'backend','admin','1','[\"resources\"]'),(NULL,0,'2024-01-31 17:38:47',NULL,3,'backend','admin','1','[\"resources\"]'),(NULL,0,'2024-01-31 17:38:47',NULL,4,'oneterm','admin','1','[\"resources\"]'),(NULL,0,'2024-01-31 17:39:03',NULL,5,'oneterm','admin','1','[\"resources\"]'),(NULL,0,'2024-01-31 17:39:03',NULL,6,'backend','admin','1','[\"resources\"]'),(NULL,0,'2024-01-31 17:40:29',NULL,7,'backend','admin','1','[\"resources\"]'),(NULL,0,'2024-01-31 17:40:29',NULL,8,'oneterm','admin','1','[\"resources\"]'),(NULL,0,'2024-01-31 17:41:24',NULL,9,'oneterm','admin','1','[\"resources\"]'),(NULL,0,'2024-01-31 17:41:24',NULL,10,'backend','admin','1','[\"resources\"]'),(NULL,0,'2024-01-31 17:41:39',NULL,11,'backend','admin','1','[\"resources\"]'),(NULL,0,'2024-01-31 17:41:39',NULL,12,'oneterm','admin','1','[\"resources\"]'),(NULL,0,'2024-01-31 17:42:50',NULL,13,'backend','admin','1','[\"resources\"]'),(NULL,0,'2024-01-31 17:42:50',NULL,14,'oneterm','admin','1','[\"resources\"]'),(NULL,0,'2024-01-31 17:47:14',NULL,15,'backend','admin','1','[\"resources\"]'),(NULL,0,'2024-01-31 17:47:14',NULL,16,'oneterm','admin','1','[\"resources\"]'),(NULL,0,'2024-01-31 17:54:43',NULL,17,'backend','admin','1','[\"resources\"]'),(NULL,0,'2024-01-31 17:54:43',NULL,18,'oneterm','admin','1','[\"resources\"]'),(NULL,0,'2024-01-31 17:56:49',NULL,19,'oneterm','admin','1','[\"resources\"]'),(NULL,0,'2024-01-31 17:56:49',NULL,20,'backend','admin','1','[\"resources\"]'),(NULL,0,'2024-02-01 10:22:36',NULL,21,'oneterm','admin','1','[\"resources\"]'),(NULL,0,'2024-02-01 10:22:36',NULL,22,'backend','admin','1','[\"resources\"]'),(NULL,0,'2024-02-01 10:22:55',NULL,23,'backend','admin','1','[\"resources\"]'),(NULL,0,'2024-02-01 10:22:55',NULL,24,'oneterm','admin','1','[\"resources\"]'),(NULL,0,'2024-02-01 10:23:02',NULL,25,'oneterm','admin','1','[\"resources\"]'),(NULL,0,'2024-02-01 10:23:02',NULL,26,'backend','admin','1','[\"resources\"]'),(NULL,0,'2024-02-01 10:23:20',NULL,27,'oneterm','admin','1','[\"resources\"]'),(NULL,0,'2024-02-01 10:23:20',NULL,28,'backend','admin','1','[\"resources\"]'),(NULL,0,'2024-02-01 10:23:24',NULL,29,'oneterm','admin','1','[\"resources\"]'),(NULL,0,'2024-02-01 10:23:24',NULL,30,'backend','admin','1','[\"resources\"]'),(NULL,0,'2024-02-01 10:23:45',NULL,31,'backend','admin','1','[\"resources\"]'),(NULL,0,'2024-02-01 10:23:45',NULL,32,'oneterm','admin','1','[\"resources\"]'),(NULL,0,'2024-02-01 10:24:58',NULL,33,'backend','admin','1','[\"resources\"]'),(NULL,0,'2024-02-01 10:24:58',NULL,34,'oneterm','admin','1','[\"resources\"]');
/*!40000 ALTER TABLE `acl_operation_records` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `acl_permissions`
--

DROP TABLE IF EXISTS `acl_permissions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `acl_permissions` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `resource_type_id` int DEFAULT NULL,
  `app_id` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `resource_type_id` (`resource_type_id`),
  KEY `app_id` (`app_id`),
  KEY `ix_acl_permissions_deleted` (`deleted`),
  CONSTRAINT `acl_permissions_ibfk_1` FOREIGN KEY (`resource_type_id`) REFERENCES `acl_resource_types` (`id`),
  CONSTRAINT `acl_permissions_ibfk_2` FOREIGN KEY (`app_id`) REFERENCES `acl_apps` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=29 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `acl_permissions`
--

LOCK TABLES `acl_permissions` WRITE;
/*!40000 ALTER TABLE `acl_permissions` DISABLE KEYS */;
INSERT INTO `acl_permissions` VALUES (NULL,0,'2024-01-31 17:33:25',NULL,1,'read',1,1),(NULL,0,'2024-01-31 17:33:25',NULL,2,'grant',1,1),(NULL,0,'2024-01-31 17:33:25',NULL,3,'delete',1,1),(NULL,0,'2024-01-31 17:33:25',NULL,4,'update',1,1),(NULL,0,'2024-01-31 17:41:59',NULL,5,'read',2,3),(NULL,0,'2024-01-31 17:41:59',NULL,6,'write',2,3),(NULL,0,'2024-01-31 17:41:59',NULL,7,'delete',2,3),(NULL,0,'2024-01-31 17:41:59',NULL,8,'grant',2,3),(NULL,0,'2024-01-31 17:42:14',NULL,9,'read',3,3),(NULL,0,'2024-01-31 17:42:14',NULL,10,'write',3,3),(NULL,0,'2024-01-31 17:42:14',NULL,11,'delete',3,3),(NULL,0,'2024-01-31 17:42:14',NULL,12,'grant',3,3),(NULL,0,'2024-01-31 17:42:29',NULL,13,'read',4,3),(NULL,0,'2024-01-31 17:42:29',NULL,14,'write',4,3),(NULL,0,'2024-01-31 17:42:29',NULL,15,'delete',4,3),(NULL,0,'2024-01-31 17:42:29',NULL,16,'grant',4,3),(NULL,0,'2024-01-31 17:42:44',NULL,17,'read',5,3),(NULL,0,'2024-01-31 17:42:44',NULL,18,'write',5,3),(NULL,0,'2024-01-31 17:42:44',NULL,19,'delete',5,3),(NULL,0,'2024-01-31 17:42:44',NULL,20,'grant',5,3),(NULL,0,'2024-01-31 17:42:59',NULL,21,'read',6,3),(NULL,0,'2024-01-31 17:42:59',NULL,22,'write',6,3),(NULL,0,'2024-01-31 17:42:59',NULL,23,'delete',6,3),(NULL,0,'2024-01-31 17:42:59',NULL,24,'grant',6,3),(NULL,0,'2024-01-31 17:43:14',NULL,25,'read',7,3),(NULL,0,'2024-01-31 17:43:14',NULL,26,'write',7,3),(NULL,0,'2024-01-31 17:43:14',NULL,27,'delete',7,3),(NULL,0,'2024-01-31 17:43:14',NULL,28,'grant',7,3);
/*!40000 ALTER TABLE `acl_permissions` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `acl_resource_group_items`
--

DROP TABLE IF EXISTS `acl_resource_group_items`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `acl_resource_group_items` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `group_id` int NOT NULL,
  `resource_id` int NOT NULL,
  PRIMARY KEY (`id`),
  KEY `group_id` (`group_id`),
  KEY `resource_id` (`resource_id`),
  KEY `ix_acl_resource_group_items_deleted` (`deleted`),
  CONSTRAINT `acl_resource_group_items_ibfk_1` FOREIGN KEY (`group_id`) REFERENCES `acl_resource_groups` (`id`),
  CONSTRAINT `acl_resource_group_items_ibfk_2` FOREIGN KEY (`resource_id`) REFERENCES `acl_resources` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `acl_resource_group_items`
--

LOCK TABLES `acl_resource_group_items` WRITE;
/*!40000 ALTER TABLE `acl_resource_group_items` DISABLE KEYS */;
/*!40000 ALTER TABLE `acl_resource_group_items` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `acl_resource_groups`
--

DROP TABLE IF EXISTS `acl_resource_groups`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `acl_resource_groups` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `resource_type_id` int DEFAULT NULL,
  `uid` int DEFAULT NULL,
  `app_id` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `resource_type_id` (`resource_type_id`),
  KEY `app_id` (`app_id`),
  KEY `ix_acl_resource_groups_uid` (`uid`),
  KEY `ix_acl_resource_groups_name` (`name`),
  KEY `ix_acl_resource_groups_deleted` (`deleted`),
  CONSTRAINT `acl_resource_groups_ibfk_1` FOREIGN KEY (`resource_type_id`) REFERENCES `acl_resource_types` (`id`),
  CONSTRAINT `acl_resource_groups_ibfk_2` FOREIGN KEY (`app_id`) REFERENCES `acl_apps` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `acl_resource_groups`
--

LOCK TABLES `acl_resource_groups` WRITE;
/*!40000 ALTER TABLE `acl_resource_groups` DISABLE KEYS */;
/*!40000 ALTER TABLE `acl_resource_groups` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `acl_resource_types`
--

DROP TABLE IF EXISTS `acl_resource_types`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `acl_resource_types` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `description` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `app_id` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `app_id` (`app_id`),
  KEY `ix_acl_resource_types_deleted` (`deleted`),
  KEY `ix_acl_resource_types_name` (`name`),
  CONSTRAINT `acl_resource_types_ibfk_1` FOREIGN KEY (`app_id`) REFERENCES `acl_apps` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=8 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `acl_resource_types`
--

LOCK TABLES `acl_resource_types` WRITE;
/*!40000 ALTER TABLE `acl_resource_types` DISABLE KEYS */;
INSERT INTO `acl_resource_types` VALUES (NULL,0,'2024-01-31 17:33:25',NULL,1,'操作权限','',1),(NULL,0,'2024-01-31 17:41:59',NULL,2,'account','',4),(NULL,0,'2024-01-31 17:42:14',NULL,3,'asset','',4),(NULL,0,'2024-01-31 17:42:29',NULL,4,'command','',4),(NULL,0,'2024-01-31 17:42:44',NULL,5,'gateway','',4),(NULL,0,'2024-01-31 17:42:59',NULL,6,'menu','',4),(NULL,0,'2024-01-31 17:43:14',NULL,7,'authorization','',4);
/*!40000 ALTER TABLE `acl_resource_types` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `acl_resources`
--

DROP TABLE IF EXISTS `acl_resources`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `acl_resources` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `resource_type_id` int DEFAULT NULL,
  `uid` int DEFAULT NULL,
  `app_id` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `resource_type_id` (`resource_type_id`),
  KEY `app_id` (`app_id`),
  KEY `ix_acl_resources_deleted` (`deleted`),
  KEY `ix_acl_resources_uid` (`uid`),
  CONSTRAINT `acl_resources_ibfk_1` FOREIGN KEY (`resource_type_id`) REFERENCES `acl_resource_types` (`id`),
  CONSTRAINT `acl_resources_ibfk_2` FOREIGN KEY (`app_id`) REFERENCES `acl_apps` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=16 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `acl_resources`
--

LOCK TABLES `acl_resources` WRITE;
/*!40000 ALTER TABLE `acl_resources` DISABLE KEYS */;
INSERT INTO `acl_resources` VALUES (NULL,0,'2024-01-31 17:33:25',NULL,1,'公司信息',1,NULL,1),(NULL,0,'2024-01-31 17:33:26',NULL,2,'公司架构',1,NULL,1),(NULL,0,'2024-01-31 17:33:26',NULL,3,'通知设置',1,NULL,1),(NULL,0,'2024-01-31 17:43:46',NULL,4,'仪表盘',6,1,4),(NULL,0,'2024-01-31 17:43:52',NULL,5,'资产管理',6,1,4),(NULL,0,'2024-01-31 17:43:57',NULL,6,'资产列表',6,1,4),(NULL,0,'2024-01-31 17:44:02',NULL,7,'网关列表',6,1,4),(NULL,0,'2024-01-31 17:44:08',NULL,8,'账号列表',6,1,4),(NULL,0,'2024-01-31 17:44:14',NULL,9,'命令过滤',6,1,4),(NULL,0,'2024-01-31 17:44:20',NULL,10,'会话审计',6,1,4),(NULL,0,'2024-01-31 17:44:24',NULL,11,'在线会话',6,1,4),(NULL,0,'2024-01-31 17:44:29',NULL,12,'历史会话',6,1,4),(NULL,0,'2024-01-31 17:44:33',NULL,13,'日志审计',6,1,4),(NULL,0,'2024-01-31 17:44:38',NULL,14,'登录日志',6,1,4),(NULL,0,'2024-01-31 17:44:45',NULL,15,'操作日志',6,1,4);
/*!40000 ALTER TABLE `acl_resources` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `acl_role_permissions`
--

DROP TABLE IF EXISTS `acl_role_permissions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `acl_role_permissions` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `rid` int DEFAULT NULL,
  `resource_id` int DEFAULT NULL,
  `group_id` int DEFAULT NULL,
  `perm_id` int DEFAULT NULL,
  `app_id` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `rid` (`rid`),
  KEY `resource_id` (`resource_id`),
  KEY `group_id` (`group_id`),
  KEY `perm_id` (`perm_id`),
  KEY `app_id` (`app_id`),
  KEY `ix_acl_role_permissions_deleted` (`deleted`),
  CONSTRAINT `acl_role_permissions_ibfk_1` FOREIGN KEY (`rid`) REFERENCES `acl_roles` (`id`),
  CONSTRAINT `acl_role_permissions_ibfk_2` FOREIGN KEY (`resource_id`) REFERENCES `acl_resources` (`id`),
  CONSTRAINT `acl_role_permissions_ibfk_3` FOREIGN KEY (`group_id`) REFERENCES `acl_resource_groups` (`id`),
  CONSTRAINT `acl_role_permissions_ibfk_4` FOREIGN KEY (`perm_id`) REFERENCES `acl_permissions` (`id`),
  CONSTRAINT `acl_role_permissions_ibfk_5` FOREIGN KEY (`app_id`) REFERENCES `acl_apps` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=13 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `acl_role_permissions`
--

LOCK TABLES `acl_role_permissions` WRITE;
/*!40000 ALTER TABLE `acl_role_permissions` DISABLE KEYS */;
INSERT INTO `acl_role_permissions` VALUES (NULL,0,'2024-01-31 17:37:41',NULL,1,1,1,NULL,4,1),(NULL,0,'2024-01-31 17:37:41',NULL,2,1,1,NULL,1,1),(NULL,0,'2024-01-31 17:37:41',NULL,3,1,1,NULL,3,1),(NULL,0,'2024-01-31 17:37:41',NULL,4,1,1,NULL,2,1),(NULL,0,'2024-01-31 17:37:41',NULL,5,1,2,NULL,4,1),(NULL,0,'2024-01-31 17:37:41',NULL,6,1,2,NULL,1,1),(NULL,0,'2024-01-31 17:37:41',NULL,7,1,2,NULL,3,1),(NULL,0,'2024-01-31 17:37:41',NULL,8,1,2,NULL,2,1),(NULL,0,'2024-01-31 17:37:41',NULL,9,1,3,NULL,4,1),(NULL,0,'2024-01-31 17:37:41',NULL,10,1,3,NULL,1,1),(NULL,0,'2024-01-31 17:37:41',NULL,11,1,3,NULL,3,1),(NULL,0,'2024-01-31 17:37:41',NULL,12,1,3,NULL,2,1);
/*!40000 ALTER TABLE `acl_role_permissions` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `acl_role_relations`
--

DROP TABLE IF EXISTS `acl_role_relations`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `acl_role_relations` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `parent_id` int DEFAULT NULL,
  `child_id` int DEFAULT NULL,
  `app_id` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `parent_id` (`parent_id`),
  KEY `child_id` (`child_id`),
  KEY `app_id` (`app_id`),
  KEY `ix_acl_role_relations_deleted` (`deleted`),
  CONSTRAINT `acl_role_relations_ibfk_1` FOREIGN KEY (`parent_id`) REFERENCES `acl_roles` (`id`),
  CONSTRAINT `acl_role_relations_ibfk_2` FOREIGN KEY (`child_id`) REFERENCES `acl_roles` (`id`),
  CONSTRAINT `acl_role_relations_ibfk_3` FOREIGN KEY (`app_id`) REFERENCES `acl_apps` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `acl_role_relations`
--

LOCK TABLES `acl_role_relations` WRITE;
/*!40000 ALTER TABLE `acl_role_relations` DISABLE KEYS */;
INSERT INTO `acl_role_relations` VALUES (NULL,0,'2024-01-31 17:33:48',NULL,1,2,1,2),(NULL,0,'2024-02-01 10:24:55',NULL,2,3,1,4);
/*!40000 ALTER TABLE `acl_role_relations` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `acl_roles`
--

DROP TABLE IF EXISTS `acl_roles`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `acl_roles` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `is_app_admin` tinyint(1) DEFAULT NULL,
  `app_id` int DEFAULT NULL,
  `uid` int DEFAULT NULL,
  `password` varchar(80) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `key` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `secret` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `app_id` (`app_id`),
  KEY `ix_acl_roles_deleted` (`deleted`),
  KEY `ix_acl_roles_name` (`name`),
  CONSTRAINT `acl_roles_ibfk_1` FOREIGN KEY (`app_id`) REFERENCES `acl_apps` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `acl_roles`
--

LOCK TABLES `acl_roles` WRITE;
/*!40000 ALTER TABLE `acl_roles` DISABLE KEYS */;
INSERT INTO `acl_roles` VALUES (NULL,0,'2024-01-31 17:33:48',NULL,1,'admin',0,NULL,1,NULL,'948a8e6eaa724ff9b4d7ccfbbae37b6e','~zKFxvUTXumVPM7A2c681kqLSY3bDGtR'),(NULL,0,'2024-01-31 17:33:48',NULL,2,'acl_admin',1,NULL,NULL,NULL,'7b845b56a0bb41bc9b1f96dc71dfb00d','pUzS%a8kHrPvCdI4XZux~ml@7n$3Ktqf'),(NULL,0,'2024-01-31 17:59:30',NULL,3,'oneterm_admin',1,4,NULL,NULL,'7c4404b8233a431b8be70ccf39a96e5d','jbfhN#30peycOw!ZBEimJXoqADIS4s8z');
/*!40000 ALTER TABLE `acl_roles` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `acl_triggers`
--

DROP TABLE IF EXISTS `acl_triggers`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `acl_triggers` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `wildcard` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `uid` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `resource_type_id` int DEFAULT NULL,
  `roles` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `permissions` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `enabled` tinyint(1) DEFAULT NULL,
  `app_id` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `resource_type_id` (`resource_type_id`),
  KEY `app_id` (`app_id`),
  KEY `ix_acl_triggers_deleted` (`deleted`),
  CONSTRAINT `acl_triggers_ibfk_1` FOREIGN KEY (`resource_type_id`) REFERENCES `acl_resource_types` (`id`),
  CONSTRAINT `acl_triggers_ibfk_2` FOREIGN KEY (`app_id`) REFERENCES `acl_apps` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `acl_triggers`
--

LOCK TABLES `acl_triggers` WRITE;
/*!40000 ALTER TABLE `acl_triggers` DISABLE KEYS */;
/*!40000 ALTER TABLE `acl_triggers` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `common_company_info_json`
--

DROP TABLE IF EXISTS `common_company_info_json`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `common_company_info_json` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `info` json DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_common_company_info_json_deleted` (`deleted`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `common_company_info_json`
--

LOCK TABLES `common_company_info_json` WRITE;
/*!40000 ALTER TABLE `common_company_info_json` DISABLE KEYS */;
/*!40000 ALTER TABLE `common_company_info_json` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `common_data`
--

DROP TABLE IF EXISTS `common_data`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `common_data` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `data_type` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `data` json DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_common_data_deleted` (`deleted`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `common_data`
--

LOCK TABLES `common_data` WRITE;
/*!40000 ALTER TABLE `common_data` DISABLE KEYS */;
/*!40000 ALTER TABLE `common_data` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `common_department`
--

DROP TABLE IF EXISTS `common_department`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `common_department` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `department_id` int NOT NULL AUTO_INCREMENT,
  `department_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `department_director_id` int DEFAULT NULL,
  `department_parent_id` int DEFAULT NULL,
  `sort_value` int DEFAULT NULL,
  `acl_rid` int DEFAULT NULL,
  PRIMARY KEY (`department_id`),
  KEY `ix_common_department_deleted` (`deleted`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `common_department`
--

LOCK TABLES `common_department` WRITE;
/*!40000 ALTER TABLE `common_department` DISABLE KEYS */;
INSERT INTO `common_department` VALUES (NULL,0,'2024-01-31 17:33:25','2024-01-31 17:33:25',0,'全公司',0,-1,0,0);
/*!40000 ALTER TABLE `common_department` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `common_employee`
--

DROP TABLE IF EXISTS `common_employee`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `common_employee` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `employee_id` int NOT NULL AUTO_INCREMENT,
  `email` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `username` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `nickname` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `sex` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `position_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `mobile` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `avatar` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `direct_supervisor_id` int DEFAULT NULL,
  `department_id` int DEFAULT NULL,
  `acl_uid` int DEFAULT NULL,
  `acl_rid` int DEFAULT NULL,
  `acl_virtual_rid` int DEFAULT NULL,
  `last_login` timestamp NULL DEFAULT NULL,
  `block` int DEFAULT NULL,
  `notice_info` json DEFAULT NULL,
  PRIMARY KEY (`employee_id`),
  KEY `department_id` (`department_id`),
  KEY `ix_common_employee_deleted` (`deleted`),
  CONSTRAINT `common_employee_ibfk_1` FOREIGN KEY (`department_id`) REFERENCES `common_department` (`department_id`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `common_employee`
--

LOCK TABLES `common_employee` WRITE;
/*!40000 ALTER TABLE `common_employee` DISABLE KEYS */;
INSERT INTO `common_employee` VALUES (NULL,0,'2024-01-31 17:33:48','2024-02-01 10:22:36',1,'amin@veops.cn','admin','admin','','','','',0,0,1,1,0,'2024-02-01 02:22:36',0,'{}');
/*!40000 ALTER TABLE `common_employee` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `common_employee_info`
--

DROP TABLE IF EXISTS `common_employee_info`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `common_employee_info` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `info` json DEFAULT NULL,
  `employee_id` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `employee_id` (`employee_id`),
  KEY `ix_common_employee_info_deleted` (`deleted`),
  CONSTRAINT `common_employee_info_ibfk_1` FOREIGN KEY (`employee_id`) REFERENCES `common_employee` (`employee_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `common_employee_info`
--

LOCK TABLES `common_employee_info` WRITE;
/*!40000 ALTER TABLE `common_employee_info` DISABLE KEYS */;
/*!40000 ALTER TABLE `common_employee_info` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `common_file`
--

DROP TABLE IF EXISTS `common_file`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `common_file` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `file_name` varchar(512) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `origin_name` varchar(512) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `binary` longblob NOT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_common_file_deleted` (`deleted`),
  KEY `ix_common_file_file_name` (`file_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `common_file`
--

LOCK TABLES `common_file` WRITE;
/*!40000 ALTER TABLE `common_file` DISABLE KEYS */;
/*!40000 ALTER TABLE `common_file` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `common_internal_message`
--

DROP TABLE IF EXISTS `common_internal_message`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `common_internal_message` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `title` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `content` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `path` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `is_read` tinyint(1) DEFAULT NULL,
  `app_name` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `category` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `message_data` json DEFAULT NULL,
  `employee_id` int DEFAULT NULL COMMENT 'ID',
  PRIMARY KEY (`id`),
  KEY `employee_id` (`employee_id`),
  KEY `ix_common_internal_message_deleted` (`deleted`),
  CONSTRAINT `common_internal_message_ibfk_1` FOREIGN KEY (`employee_id`) REFERENCES `common_employee` (`employee_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `common_internal_message`
--

LOCK TABLES `common_internal_message` WRITE;
/*!40000 ALTER TABLE `common_internal_message` DISABLE KEYS */;
/*!40000 ALTER TABLE `common_internal_message` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `common_notice_config`
--

DROP TABLE IF EXISTS `common_notice_config`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `common_notice_config` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT,
  `platform` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `info` json DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_common_notice_config_deleted` (`deleted`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `common_notice_config`
--

LOCK TABLES `common_notice_config` WRITE;
/*!40000 ALTER TABLE `common_notice_config` DISABLE KEYS */;
/*!40000 ALTER TABLE `common_notice_config` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `users`
--

DROP TABLE IF EXISTS `users`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `users` (
  `deleted_at` datetime DEFAULT NULL,
  `deleted` tinyint(1) DEFAULT NULL,
  `uid` int NOT NULL AUTO_INCREMENT,
  `username` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `nickname` varchar(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `department` varchar(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `catalog` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `email` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `mobile` varchar(14) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `password` varchar(80) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `key` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `secret` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `date_joined` datetime DEFAULT NULL,
  `last_login` datetime DEFAULT NULL,
  `block` tinyint(1) DEFAULT NULL,
  `has_logined` tinyint(1) DEFAULT NULL,
  `wx_id` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `employee_id` varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `avatar` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`uid`),
  UNIQUE KEY `email` (`email`),
  UNIQUE KEY `username` (`username`),
  UNIQUE KEY `mobile` (`mobile`),
  KEY `ix_users_employee_id` (`employee_id`),
  KEY `ix_users_deleted` (`deleted`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `users`
--

LOCK TABLES `users` WRITE;
/*!40000 ALTER TABLE `users` DISABLE KEYS */;
INSERT INTO `users` VALUES (NULL,0,1,'admin','admin',NULL,NULL,'amin@veops.cn',NULL,'e10adc3949ba59abbe56e057f20f883e','a5704726392648b7b5a15cc39091a166','P#Iunzvq7E^6mwMbftgW@KYG28x14*Dy','2024-01-31 09:33:48','2024-02-01 10:22:36',0,1,NULL,'0001',NULL);
/*!40000 ALTER TABLE `users` ENABLE KEYS */;
UNLOCK TABLES;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2024-02-01 10:25:41
