CREATE DATABASE IF NOT EXISTS `miic_portal`
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_0900_ai_ci;

USE `miic_portal`;

CREATE TABLE IF NOT EXISTS `page` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  `name` VARCHAR(100) NOT NULL,
  `code` VARCHAR(100) NOT NULL,
  `page_type` VARCHAR(20) NOT NULL DEFAULT 'page',
  `route_path` VARCHAR(200) NOT NULL DEFAULT '',
  `template_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `template` VARCHAR(100) NOT NULL DEFAULT '',
  `description` VARCHAR(500) NOT NULL DEFAULT '',
  `status` BIGINT NOT NULL DEFAULT 1,
  PRIMARY KEY (`id`),
  KEY `idx_page_deleted_at` (`deleted_at`),
  KEY `idx_page_name` (`name`),
  KEY `idx_page_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `column` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  `name` VARCHAR(100) NOT NULL,
  `code` VARCHAR(100) NOT NULL,
  `page_id` BIGINT UNSIGNED NOT NULL,
  `parent_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `route_path` VARCHAR(200) NOT NULL DEFAULT '',
  `template` VARCHAR(100) NOT NULL DEFAULT '',
  `description` VARCHAR(500) NOT NULL DEFAULT '',
  `sort` BIGINT NOT NULL DEFAULT 0,
  `status` BIGINT NOT NULL DEFAULT 1,
  `display_type` BIGINT NOT NULL DEFAULT 1,
  `workflow_id` BIGINT UNSIGNED NULL,
  PRIMARY KEY (`id`),
  KEY `idx_column_deleted_at` (`deleted_at`),
  KEY `idx_column_page_id` (`page_id`),
  KEY `idx_column_parent_id` (`parent_id`),
  KEY `idx_column_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `category` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  `name` VARCHAR(100) NOT NULL,
  `code` VARCHAR(100) NOT NULL,
  `description` VARCHAR(500) NOT NULL DEFAULT '',
  `sort` BIGINT NOT NULL DEFAULT 0,
  `status` BIGINT NOT NULL DEFAULT 1,
  PRIMARY KEY (`id`),
  KEY `idx_category_deleted_at` (`deleted_at`),
  KEY `idx_category_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `article` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  `title` VARCHAR(200) NOT NULL,
  `type` BIGINT NOT NULL DEFAULT 1,
  `summary` VARCHAR(500) NOT NULL DEFAULT '',
  `content` LONGTEXT NULL,
  `status` BIGINT NOT NULL DEFAULT 0,
  `audit_status` BIGINT NOT NULL DEFAULT 0,
  `is_top` BIGINT NOT NULL DEFAULT 0,
  `is_bold` BIGINT NOT NULL DEFAULT 0,
  `default_color` VARCHAR(20) NOT NULL DEFAULT '',
  `cover` VARCHAR(500) NOT NULL DEFAULT '',
  `author` VARCHAR(100) NOT NULL DEFAULT '',
  `author_code` VARCHAR(100) NOT NULL DEFAULT '',
  `source` VARCHAR(200) NOT NULL DEFAULT '',
  `url` VARCHAR(500) NOT NULL DEFAULT '',
  `publish_time` DATETIME(3) NULL,
  `column_count` BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_article_deleted_at` (`deleted_at`),
  KEY `idx_article_publish` (`status`,`audit_status`,`publish_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `article_column_publish` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  `page_id` BIGINT UNSIGNED NOT NULL,
  `column_id` BIGINT UNSIGNED NOT NULL,
  `article_id` BIGINT UNSIGNED NOT NULL,
  `article_title` VARCHAR(200) NOT NULL DEFAULT '',
  `author` VARCHAR(100) NOT NULL DEFAULT '',
  `source` VARCHAR(200) NOT NULL DEFAULT '',
  `is_top` BIGINT NOT NULL DEFAULT 0,
  `is_bold` BIGINT NOT NULL DEFAULT 0,
  `color` VARCHAR(20) NOT NULL DEFAULT '',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_article_column_publish` (`column_id`,`article_id`),
  KEY `idx_acp_page_id` (`page_id`),
  KEY `idx_acp_article_id` (`article_id`),
  KEY `idx_acp_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `article_attachment` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `article_id` BIGINT UNSIGNED NOT NULL,
  `name` VARCHAR(255) NOT NULL,
  `url` VARCHAR(500) NOT NULL,
  `size` BIGINT NOT NULL DEFAULT 0,
  `created_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_article_attachment_article_id` (`article_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
