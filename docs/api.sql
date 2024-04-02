-- Active: 1700721140603@@192.168.20.82@53306@oneterm

CREATE DATABASE IF NOT EXISTS oneterm;

CREATE TABLE
    IF NOT EXISTS oneterm.account(
        `id` INT NOT NULL AUTO_INCREMENT,
        `name` VARCHAR(64) NOT NULL DEFAULT '',
        `account_type` int NOT NULL DEFAULT 0,
        `account` VARCHAR(64) NOT NULL DEFAULT '',
        `password` TEXT NOT NULL,
        `pk` TEXT NOT NULL,
        `phrase` TEXT NOT NULL,
        `resource_id` INT NOT NULL DEFAULT 0,
        `creator_id` INT NOT NULL DEFAULT 0,
        `updater_id` INT NOT NULL DEFAULT 0,
        `created_at` TIMESTAMP NOT NULL,
        `updated_at` TIMESTAMP NOT NULL,
        `deleted_at` BIGINT NOT NULL DEFAULT 0,
        PRIMARY KEY (`id`),
        UNIQUE KEY `name_del` (`name`, `deleted_at`)
    ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE
    IF NOT EXISTS oneterm.asset(
        `id` INT NOT NULL AUTO_INCREMENT,
        `ci_id` INT NOT NULL DEFAULT 0,
        `name` VARCHAR(64) NOT NULL DEFAULT '',
        `comment` VARCHAR(64) NOT NULL DEFAULT '',
        `parent_id` INT NOT NULL DEFAULT 0,
        `ip` VARCHAR(64) NOT NULL DEFAULT '',
        `protocols` JSON NOT NULL,
        `gateway_id` INT NOT NULL DEFAULT 0,
        `authorization` JSON NOT NULL,
        `start` TIMESTAMP,
        `end` TIMESTAMP,
        `cmd_ids` JSON NOT NULL,
        `ranges` JSON NOT NULL,
        `allow` TINYINT(1) NOT NULL DEFAULT 0,
        `connectable` TINYINT(1) NOT NULL DEFAULT 0,
        `resource_id` INT NOT NULL DEFAULT 0,
        `creator_id` INT NOT NULL DEFAULT 0,
        `created_at` TIMESTAMP NOT NULL,
        `updater_id` INT NOT NULL DEFAULT 0,
        `updated_at` TIMESTAMP NOT NULL,
        `deleted_at` BIGINT NOT NULL DEFAULT 0,
        PRIMARY KEY (`id`),
        UNIQUE KEY `name_del` (`name`, `deleted_at`)
    ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE
    IF NOT EXISTS oneterm.command(
        `id` INT NOT NULL AUTO_INCREMENT,
        `name` VARCHAR(64) NOT NULL DEFAULT '',
        `cmds` JSON NOT NULL,
        `enable` TINYINT(1) NOT NULL DEFAULT 0,
        `resource_id` INT NOT NULL DEFAULT 0,
        `creator_id` INT NOT NULL DEFAULT 0,
        `updater_id` INT NOT NULL DEFAULT 0,
        `created_at` TIMESTAMP NOT NULL,
        `updated_at` TIMESTAMP NOT NULL,
        `deleted_at` BIGINT NOT NULL DEFAULT 0,
        PRIMARY KEY (`id`),
        UNIQUE KEY `name_del` (`name`, `deleted_at`)
    ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE
    IF NOT EXISTS oneterm.gateway(
        `id` INT NOT NULL AUTO_INCREMENT,
        `name` VARCHAR(64) NOT NULL DEFAULT '',
        `host` VARCHAR(64) NOT NULL DEFAULT '',
        `port` INT NOT NULL DEFAULT 0,
        `account_type` int NOT NULL DEFAULT 0,
        `account` VARCHAR(64) NOT NULL DEFAULT '',
        `password` TEXT NOT NULL,
        `pk` TEXT NOT NULL,
        `phrase` TEXT NOT NULL,
        `resource_id` INT NOT NULL DEFAULT 0,
        `creator_id` INT NOT NULL DEFAULT 0,
        `updater_id` INT NOT NULL DEFAULT 0,
        `created_at` TIMESTAMP NOT NULL,
        `updated_at` TIMESTAMP NOT NULL,
        `deleted_at` BIGINT NOT NULL DEFAULT 0,
        PRIMARY KEY (`id`),
        UNIQUE KEY `name_del` (`name`, `deleted_at`)
    ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE
    IF NOT EXISTS oneterm.node(
        `id` INT NOT NULL AUTO_INCREMENT,
        `name` VARCHAR(64) NOT NULL DEFAULT '',
        `comment` VARCHAR(64) NOT NULL DEFAULT '',
        `parent_id` INT NOT NULL DEFAULT 0,
        `ip` VARCHAR(64) NOT NULL DEFAULT '',
        `protocols` JSON NOT NULL,
        `gateway_id` INT NOT NULL DEFAULT 0,
        `authorization` JSON NOT NULL,
        `start` TIMESTAMP,
        `end` TIMESTAMP,
        `cmd_ids` JSON NOT NULL,
        `ranges` JSON NOT NULL,
        `allow` TINYINT(1) NOT NULL DEFAULT 0,
        `type_id` INT NOT NULL DEFAULT 0,
        `mapping` JSON NOT NULL,
        `filters` TEXT NOT NULL,
        `enable` TINYINT(1) NOT NULL DEFAULT 0,
        `frequency` DOUBLE NOT NULL DEFAULT 0,
        `creator_id` INT NOT NULL DEFAULT 0,
        `created_at` TIMESTAMP NOT NULL,
        `updater_id` INT NOT NULL DEFAULT 0,
        `updated_at` TIMESTAMP NOT NULL,
        `deleted_at` BIGINT NOT NULL DEFAULT 0,
        PRIMARY KEY (`id`)
    ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE
    IF NOT EXISTS oneterm.public_key(
        `id` INT NOT NULL AUTO_INCREMENT,
        `uid` INT NOT NULL DEFAULT 0,
        `username` VARCHAR(64) NOT NULL DEFAULT '',
        `name` VARCHAR(64) NOT NULL DEFAULT '',
        `mac` VARCHAR(64) NOT NULL DEFAULT '',
        `pk` TEXT NOT NULL,
        `creator_id` INT NOT NULL DEFAULT 0,
        `updater_id` INT NOT NULL DEFAULT 0,
        `created_at` TIMESTAMP NOT NULL,
        `updated_at` TIMESTAMP NOT NULL,
        `deleted_at` BIGINT NOT NULL DEFAULT 0,
        PRIMARY KEY (`id`),
        UNIQUE KEY `creator_id_name_del` (
            `creator_id`,
            `name`,
            `deleted_at`
        )
    ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE
    IF NOT EXISTS oneterm.history(
        `id` INT NOT NULL AUTO_INCREMENT,
        `remote_ip` VARCHAR(64) NOT NULL DEFAULT 0,
        `type` VARCHAR(64) NOT NULL DEFAULT 0,
        `target_id` INT NOT NULL DEFAULT 0,
        `old` JSON NOT NULL,
        `new` JSON NOT NULL,
        `action_type` INT NOT NULL DEFAULT 0,
        `creator_id` INT NOT NULL DEFAULT 0,
        `created_at` TIMESTAMP NOT NULL,
        PRIMARY KEY (`id`)
    ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE
    IF NOT EXISTS oneterm.session(
        `id` INT NOT NULL AUTO_INCREMENT,
        `session_type` INT NOT NULL DEFAULT 0,
        `session_id` VARCHAR(64) NOT NULL DEFAULT '',
        `uid` INT NOT NULL DEFAULT 0,
        `user_name` VARCHAR(64) NOT NULL DEFAULT '',
        `asset_id` INT NOT NULL DEFAULT 0,
        `asset_info` VARCHAR(64) NOT NULL DEFAULT '',
        `account_id` INT NOT NULL DEFAULT 0,
        `account_info` VARCHAR(64) NOT NULL DEFAULT '',
        `gateway_id` INT NOT NULL DEFAULT 0,
        `gateway_info` VARCHAR(64) NOT NULL DEFAULT '',
        `protocol` VARCHAR(64) NOT NULL DEFAULT '',
        `client_ip` VARCHAR(64) NOT NULL DEFAULT '',
        `status` INT NOT NULL DEFAULT 0,
        `created_at` TIMESTAMP NOT NULL,
        `updated_at` TIMESTAMP NOT NULL,
        `closed_at` TIMESTAMP,
        PRIMARY KEY(`id`),
        UNIQUE KEY `session_id` (`session_id`)
    ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE
    IF NOT EXISTS oneterm.session_cmd(
        `id` INT NOT NULL AUTO_INCREMENT,
        `session_id` VARCHAR(64) NOT NULL DEFAULT '',
        `cmd` TEXT NOT NULL,
        `result` TEXT NOT NULL,
        `level` INT NOT NULL DEFAULT 0,
        `created_at` TIMESTAMP NOT NULL,
        PRIMARY KEY(`id`)
    ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE
    IF NOT EXISTS oneterm.authorization(
        `id` INT NOT NULL AUTO_INCREMENT,
        `asset_id` INT NOT NULL DEFAULT 0,
        `account_id` INT NOT NULL DEFAULT 0,
        `resource_id` INT NOT NULL DEFAULT 0,
        `creator_id` INT NOT NULL DEFAULT 0,
        `created_at` TIMESTAMP NOT NULL,
        `updater_id` INT NOT NULL DEFAULT 0,
        `updated_at` TIMESTAMP NOT NULL,
        `deleted_at` BIGINT NOT NULL DEFAULT 0,
        PRIMARY KEY(`id`),
        UNIQUE KEY `asset_account_id_del` (
            `asset_id`,
            `account_id`,
            `deleted_at`
        )
    ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE
    IF NOT EXISTS oneterm.config(
        `id` INT NOT NULL AUTO_INCREMENT,
        `timeout` INT NOT NULL,
        `creator_id` INT NOT NULL DEFAULT 0,
        `created_at` TIMESTAMP NOT NULL,
        `updater_id` INT NOT NULL DEFAULT 0,
        `updated_at` TIMESTAMP NOT NULL,
        `deleted_at` BIGINT NOT NULL DEFAULT 0,
        PRIMARY KEY(`id`),
        UNIQUE KEY `deleted_at` (`deleted_at`)
    ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

INSERT INTO oneterm.config (timeout) VALUES (7200);


CREATE TABLE
    IF NOT EXISTS oneterm.file_history(
        `id` INT NOT NULL AUTO_INCREMENT,
        `uid` INT NOT NULL DEFAULT 0,
        `user_name` VARCHAR(64) NOT NULL DEFAULT '',
        `asset_id` INT NOT NULL DEFAULT 0,
        `account_id` INT NOT NULL DEFAULT 0,
        `client_ip` VARCHAR(64) NOT NULL DEFAULT '',
        `action` INT NOT NULL DEFAULT 0,
        `dir` VARCHAR(256) NOT NULL DEFAULT '',
        `filename` VARCHAR(256) NOT NULL DEFAULT '',
        `created_at` TIMESTAMP NOT NULL,
        `updated_at` TIMESTAMP NOT NULL,
        PRIMARY KEY(`id`)
    ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;