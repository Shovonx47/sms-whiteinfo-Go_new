-- Schema & Settings Parity Migration (Reference Laravel migrations 2026_10_21 through 2026_10_25)
-- Safe and idempotent for MariaDB 10.4+ (sms_panel / sms_panel_scratch).

-- 1. 2026_10_21_000001_create_email_logs_table
CREATE TABLE IF NOT EXISTS `email_logs` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint(20) unsigned DEFAULT NULL,
  `recipient_email` varchar(255) NOT NULL,
  `recipient_name` varchar(255) DEFAULT NULL,
  `sender_email` varchar(255) DEFAULT NULL,
  `sender_name` varchar(255) DEFAULT NULL,
  `subject` varchar(255) NOT NULL,
  `type` varchar(50) NOT NULL DEFAULT 'general',
  `body_html` longtext DEFAULT NULL,
  `body_text` longtext DEFAULT NULL,
  `gateway_id` bigint(20) unsigned DEFAULT NULL,
  `gateway_name` varchar(255) DEFAULT NULL,
  `mailer` varchar(255) DEFAULT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'sent',
  `error_message` text DEFAULT NULL,
  `metadata` longtext CHARACTER SET utf8mb4 COLLATE utf8mb4_bin DEFAULT NULL CHECK (json_valid(`metadata`)),
  `sent_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `email_logs_user_id_foreign` (`user_id`),
  KEY `email_logs_gateway_id_foreign` (`gateway_id`),
  KEY `email_logs_recipient_email_index` (`recipient_email`),
  KEY `email_logs_subject_index` (`subject`),
  KEY `email_logs_type_index` (`type`),
  KEY `email_logs_status_index` (`status`),
  KEY `email_logs_sent_at_index` (`sent_at`),
  KEY `email_logs_type_status_created_at_index` (`type`,`status`,`created_at`),
  CONSTRAINT `email_logs_gateway_id_foreign` FOREIGN KEY (`gateway_id`) REFERENCES `email_gateways` (`id`) ON DELETE SET NULL,
  CONSTRAINT `email_logs_user_id_foreign` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 2. 2026_10_21_092000 & 2026_10_21_093000 backup settings + 2026_10_21_100000 notification settings
INSERT IGNORE INTO `settings` (`group`, `name`, `locked`, `payload`, `created_at`, `updated_at`) VALUES
  ('backup', 'auto_backup_enabled', 0, 'false', NOW(), NOW()),
  ('backup', 'auto_backup_frequency', 0, '"daily"', NOW(), NOW()),
  ('backup', 'auto_backup_time', 0, '"02:00"', NOW(), NOW()),
  ('backup', 'keep_local_backups', 0, '7', NOW(), NOW()),
  ('backup', 'ftp_enabled', 0, 'false', NOW(), NOW()),
  ('backup', 'ftp_host', 0, '""', NOW(), NOW()),
  ('backup', 'ftp_port', 0, '21', NOW(), NOW()),
  ('backup', 'ftp_username', 0, '""', NOW(), NOW()),
  ('backup', 'ftp_password', 0, '""', NOW(), NOW()),
  ('backup', 'ftp_directory', 0, '"/backups"', NOW(), NOW()),
  ('backup', 'ftp_ssl', 0, 'false', NOW(), NOW()),
  ('backup', 'ftp_passive', 0, 'true', NOW(), NOW()),
  ('backup', 'ftp_timeout', 0, '30', NOW(), NOW()),
  ('backup', 'gdrive_enabled', 0, 'false', NOW(), NOW()),
  ('backup', 'gdrive_client_id', 0, '""', NOW(), NOW()),
  ('backup', 'gdrive_client_secret', 0, '""', NOW(), NOW()),
  ('backup', 'gdrive_refresh_token', 0, '""', NOW(), NOW()),
  ('backup', 'gdrive_folder_id', 0, '""', NOW(), NOW()),
  ('backup', 'last_backup_at', 0, 'null', NOW(), NOW()),
  ('backup', 'last_backup_status', 0, 'null', NOW(), NOW()),
  ('backup', 'last_backup_error', 0, 'null', NOW(), NOW()),
  ('backup', 'last_backup_file', 0, 'null', NOW(), NOW()),
  ('backup', 'last_backup_size', 0, 'null', NOW(), NOW()),
  ('backup', 'delete_local_after_upload', 0, 'true', NOW(), NOW()),
  ('notification', 'telegram_enabled', 0, 'true', NOW(), NOW()),
  ('notification', 'telegram_bot_token', 0, '""', NOW(), NOW()),
  ('notification', 'telegram_chat_id', 0, '""', NOW(), NOW()),
  ('notification', 'low_balance_threshold', 0, '500', NOW(), NOW()),
  ('notification', 'alert_on_campaign_run', 0, 'true', NOW(), NOW()),
  ('notification', 'notify_on_new_customer', 0, 'true', NOW(), NOW()),
  ('notification', 'notify_on_new_deposit', 0, 'true', NOW(), NOW()),
  ('notification', 'notify_on_gateway_error', 0, 'true', NOW(), NOW()),
  ('notification', 'admin_alert_email', 0, '"admin@whiteinfobd.com"', NOW(), NOW()),
  ('notification', 'admin_alert_phone', 0, '"+8801700000000"', NOW(), NOW());

-- 3. 2026_10_25_120500_add_approval_columns_to_templates_table
ALTER TABLE `templates`
  ADD COLUMN IF NOT EXISTS `approval` varchar(255) NOT NULL DEFAULT 'approved' AFTER `variables`,
  ADD COLUMN IF NOT EXISTS `approved_by` bigint(20) unsigned DEFAULT NULL AFTER `approval`,
  ADD COLUMN IF NOT EXISTS `approved_at` timestamp NULL DEFAULT NULL AFTER `approved_by`,
  ADD COLUMN IF NOT EXISTS `rejection_reason` text DEFAULT NULL AFTER `approved_at`;

SET @fk_templates := (
  SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'templates' AND CONSTRAINT_NAME = 'templates_approved_by_foreign'
);
SET @sql := IF(@fk_templates = 0,
  'ALTER TABLE `templates` ADD KEY `templates_approved_by_foreign` (`approved_by`), ADD CONSTRAINT `templates_approved_by_foreign` FOREIGN KEY (`approved_by`) REFERENCES `users` (`id`) ON DELETE SET NULL',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

ALTER TABLE `batches`
  ADD COLUMN IF NOT EXISTS `template_id` bigint(20) unsigned DEFAULT NULL AFTER `sender_id`;

SET @fk_batches := (
  SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'batches' AND CONSTRAINT_NAME = 'batches_template_id_foreign'
);
SET @sql := IF(@fk_batches = 0,
  'ALTER TABLE `batches` ADD KEY `batches_template_id_foreign` (`template_id`), ADD CONSTRAINT `batches_template_id_foreign` FOREIGN KEY (`template_id`) REFERENCES `templates` (`id`) ON DELETE SET NULL',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 4. 2026_10_25_140000_add_sp_kyc_verification_to_users_table
ALTER TABLE `users`
  ADD COLUMN IF NOT EXISTS `created_by_sp` tinyint(1) NOT NULL DEFAULT 0 AFTER `created_by_admin`,
  ADD COLUMN IF NOT EXISTS `kyc_sp_verified_by` bigint(20) unsigned DEFAULT NULL AFTER `kyc_verified_at`,
  ADD COLUMN IF NOT EXISTS `kyc_sp_verified_at` timestamp NULL DEFAULT NULL AFTER `kyc_sp_verified_by`;

SET @idx_users_sp := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND INDEX_NAME = 'users_created_by_sp_index'
);
SET @sql := IF(@idx_users_sp = 0,
  'ALTER TABLE `users` ADD KEY `users_created_by_sp_index` (`created_by_sp`)',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @fk_users_sp := (
  SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND CONSTRAINT_NAME = 'users_kyc_sp_verified_by_foreign'
);
SET @sql := IF(@fk_users_sp = 0,
  'ALTER TABLE `users` ADD KEY `users_kyc_sp_verified_by_foreign` (`kyc_sp_verified_by`), ADD CONSTRAINT `users_kyc_sp_verified_by_foreign` FOREIGN KEY (`kyc_sp_verified_by`) REFERENCES `users` (`id`) ON DELETE SET NULL',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE `users`
SET `created_by_sp` = 1
WHERE `type` = 'customer'
  AND `sp_id` IS NOT NULL
  AND `created_by_admin` = 0
  AND `front_nid` IS NULL;

UPDATE `users`
SET `kyc_status` = 'pending'
WHERE `created_by_sp` = 1
  AND `front_nid` IS NULL
  AND `kyc_status` = 'under_review';

-- 5. Record applied migrations in migrations table
SET @next_batch := (SELECT COALESCE(MAX(`batch`), 0) + 1 FROM `migrations`);
INSERT INTO `migrations` (`migration`, `batch`)
SELECT m.name, @next_batch
FROM (
  SELECT '2026_10_21_000001_create_email_logs_table' AS name
  UNION ALL SELECT '2026_10_21_092000_create_backup_settings_data'
  UNION ALL SELECT '2026_10_21_093000_add_delete_local_after_upload_to_backup_settings'
  UNION ALL SELECT '2026_10_21_100000_create_notification_settings_data'
  UNION ALL SELECT '2026_10_25_120500_add_approval_columns_to_templates_table'
  UNION ALL SELECT '2026_10_25_140000_add_sp_kyc_verification_to_users_table'
) AS m
WHERE NOT EXISTS (SELECT 1 FROM `migrations` existing WHERE existing.`migration` = m.name);

-- 5. Base users table columns used by Filament profile / AppAuthentication 2FA.
ALTER TABLE `users`
  ADD COLUMN IF NOT EXISTS `app_authentication_secret` text DEFAULT NULL,
  ADD COLUMN IF NOT EXISTS `app_authentication_recovery_codes` longtext DEFAULT NULL;

-- Filament stores both MFA values encrypted (AppAuthentication), so the old
-- JSON check would reject a valid Laravel value. Normalise to plain LONGTEXT.
ALTER TABLE `users`
  MODIFY COLUMN `app_authentication_secret` text DEFAULT NULL,
  MODIFY COLUMN `app_authentication_recovery_codes` longtext DEFAULT NULL;

-- 6. Pending email-change verification tokens (Filament emailChangeVerification).
CREATE TABLE IF NOT EXISTS `email_change_requests` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `new_email` varchar(255) NOT NULL,
  `token_hash` varchar(64) NOT NULL,
  `expires_at` datetime NOT NULL,
  `used_at` datetime DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `email_change_requests_token_hash_unique` (`token_hash`),
  KEY `email_change_requests_user_id_index` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
