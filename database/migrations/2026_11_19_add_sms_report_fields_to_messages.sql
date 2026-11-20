-- SMS Reports parity (Reference Laravel migration 2026_09_19_000001)
-- Adds the per-message gateway snapshot columns the Superman SMS Reports table shows.
-- Safe and idempotent for MariaDB 10.4+ (sms_panel / sms_panel_scratch).

ALTER TABLE `messages`
  ADD COLUMN IF NOT EXISTS `bill_msisdn` varchar(32) DEFAULT NULL AFTER `source_ip`,
  ADD COLUMN IF NOT EXISTS `sms_type` varchar(32) DEFAULT NULL AFTER `bill_msisdn`,
  ADD COLUMN IF NOT EXISTS `channel` varchar(32) DEFAULT NULL AFTER `sms_type`,
  ADD COLUMN IF NOT EXISTS `request_id` char(36) DEFAULT NULL AFTER `tariff_name`,
  ADD COLUMN IF NOT EXISTS `server_txn_id` varchar(128) DEFAULT NULL AFTER `request_id`,
  ADD COLUMN IF NOT EXISTS `server_response_code` varchar(32) DEFAULT NULL AFTER `server_txn_id`,
  ADD COLUMN IF NOT EXISTS `mno_txn_id` varchar(128) DEFAULT NULL AFTER `server_response_code`,
  ADD COLUMN IF NOT EXISTS `mno_response_code` varchar(32) DEFAULT NULL AFTER `mno_txn_id`,
  ADD COLUMN IF NOT EXISTS `mno_response_message` text DEFAULT NULL AFTER `mno_response_code`,
  ADD COLUMN IF NOT EXISTS `provider_response` longtext DEFAULT NULL AFTER `mno_response_message`;

SET @idx_messages_request := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'messages' AND INDEX_NAME = 'messages_request_id_index'
);
SET @sql := IF(@idx_messages_request = 0,
  'ALTER TABLE `messages` ADD KEY `messages_request_id_index` (`request_id`)',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @idx_messages_server_txn := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'messages' AND INDEX_NAME = 'messages_server_txn_id_index'
);
SET @sql := IF(@idx_messages_server_txn = 0,
  'ALTER TABLE `messages` ADD KEY `messages_server_txn_id_index` (`server_txn_id`)',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @idx_messages_report_gateway := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'messages' AND INDEX_NAME = 'messages_report_gateway_status_date_index'
);
SET @sql := IF(@idx_messages_report_gateway = 0,
  'ALTER TABLE `messages` ADD KEY `messages_report_gateway_status_date_index` (`gateway_id`, `status`, `created_at`)',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @idx_messages_report_sender := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'messages' AND INDEX_NAME = 'messages_report_sender_date_index'
);
SET @sql := IF(@idx_messages_report_sender = 0,
  'ALTER TABLE `messages` ADD KEY `messages_report_sender_date_index` (`sender_id`, `created_at`)',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
