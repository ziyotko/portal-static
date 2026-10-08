SET NAMES utf8mb4;

SELECT 'BLOCKER' AS severity, 'mysql_major_below_8' AS check_name,
       IF(CAST(SUBSTRING_INDEX(VERSION(), '.', 1) AS UNSIGNED) < 8, 1, 0) AS issue_count
UNION ALL
SELECT 'BLOCKER', 'setting_row_count_not_one', IF((SELECT COUNT(*) FROM setting) = 1, 0, 1)
UNION ALL
SELECT 'BLOCKER', 'legacy_page_table', COUNT(*)
  FROM information_schema.TABLES
 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'page'
UNION ALL
SELECT 'BLOCKER', 'legacy_page_id_columns', COUNT(*)
  FROM information_schema.COLUMNS
 WHERE TABLE_SCHEMA = DATABASE() AND COLUMN_NAME = 'page_id'
UNION ALL
SELECT 'BLOCKER', 'legacy_member_content_video_url', COUNT(*)
  FROM information_schema.COLUMNS
 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'member_content' AND COLUMN_NAME = 'video_url'
UNION ALL
SELECT 'BLOCKER', 'legacy_member_column_code', COUNT(*)
  FROM information_schema.COLUMNS
 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'member_column' AND COLUMN_NAME = 'code'
UNION ALL
SELECT 'BLOCKER', 'duplicate_camie_template_codes', COUNT(*) FROM (
  SELECT code FROM template
   WHERE code IN ('camie-layout','camie-home','camie-party','camie-ministry','camie-news','camie-training','camie-standards','camie-list','camie-article','camie-about')
   GROUP BY code HAVING COUNT(*) > 1
) duplicate_templates
UNION ALL
SELECT 'BLOCKER', 'duplicate_column_codes', COUNT(*) FROM (
  SELECT code FROM `column` GROUP BY code HAVING COUNT(*) > 1
) duplicate_columns
UNION ALL
SELECT 'BLOCKER', 'missing_required_navigation_roots', 6 - COUNT(DISTINCT code)
  FROM `column`
 WHERE code IN ('party','ministry','news','training','standards','about');

SELECT 'INFO' AS severity, 'publication_template_mismatch' AS check_name, COUNT(*) AS issue_count
  FROM article_column_publish p
  JOIN `column` c ON c.id = p.column_id
 WHERE p.template_id <> c.template_id
UNION ALL
SELECT 'INFO', 'legacy_caamm_article_covers', COUNT(*) FROM article WHERE cover LIKE '/caamm/uploads/%'
UNION ALL
SELECT 'INFO', 'legacy_caamm_attachments', COUNT(*) FROM article_attachment WHERE url LIKE '/caamm/uploads/%';

SELECT id, static_path, static_program_addr, static_program_token_name FROM setting;

SELECT code, type, status, route_path, CHAR_LENGTH(source_code) AS source_chars
  FROM template
 WHERE code LIKE 'camie-%'
 ORDER BY code, id;

SELECT TABLE_NAME, INDEX_NAME, INDEX_TYPE, GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX) AS columns_in_index
  FROM information_schema.STATISTICS
 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'article' AND INDEX_TYPE = 'FULLTEXT'
 GROUP BY TABLE_NAME, INDEX_NAME, INDEX_TYPE
 ORDER BY INDEX_NAME;
