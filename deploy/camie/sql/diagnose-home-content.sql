-- CAMIE 上线前首页数据核对；只读，需在 portal-static 使用的数据库中执行。
-- 请分别保存各个 SELECT 的结果。无须提供数据库账号、密码或文章正文。

-- 1. 数据库时间与关键栏目状态。重复的 code、禁用栏目或模板均会影响生成。
SELECT DATABASE() AS database_name, NOW() AS database_now, @@session.time_zone AS session_time_zone;

SELECT c.id, c.name, c.code, c.status AS column_status,
       c.template_id, t.code AS template_code, t.status AS template_status
FROM `column` c LEFT JOIN template t ON t.id = c.template_id
WHERE c.code IN ('home-hero', 'home-expert-insights', 'about-experts', 'news-hot')
ORDER BY c.code, c.id;

-- 2. 轮播栏目当前勾选和发布记录的差异。含已下线/未审核文章，用来解释后台列表与公开站的差异。
WITH hero AS (
  SELECT id, template_id FROM `column` WHERE code = 'home-hero'
), related AS (
  SELECT ac.article_id, ac.column_id FROM article_column ac JOIN hero h ON h.id = ac.column_id
  UNION
  SELECT p.article_id, p.column_id FROM article_column_publish p JOIN hero h ON h.id = p.column_id
), publication AS (
  SELECT p.article_id, p.column_id, COUNT(*) AS publication_rows,
         SUM(p.template_id = h.template_id) AS matching_template_rows,
         MAX(p.id) AS newest_publication_id
  FROM article_column_publish p JOIN hero h ON h.id = p.column_id
  GROUP BY p.article_id, p.column_id
)
SELECT r.column_id, a.id AS article_id, a.title, a.status, a.audit_status, a.type,
       a.is_top, a.created_at, a.publish_time,
       CASE WHEN ac.article_id IS NULL THEN 0 ELSE 1 END AS currently_selected,
       COALESCE(p.publication_rows, 0) AS publication_rows,
       COALESCE(p.matching_template_rows, 0) AS matching_template_rows,
       p.newest_publication_id,
       CASE WHEN a.status = 1 AND a.audit_status = 2 AND a.type IN (1, 2)
                  AND a.publish_time <= NOW() THEN 1 ELSE 0 END AS publicly_eligible
FROM related r JOIN article a ON a.id = r.article_id
LEFT JOIN article_column ac ON ac.article_id = r.article_id AND ac.column_id = r.column_id
LEFT JOIN publication p ON p.article_id = r.article_id AND p.column_id = r.column_id
ORDER BY a.is_top DESC, a.publish_time DESC, a.id DESC;

-- 3. 使用修复后口径模拟首页轮播；首页最多取前三篇。
SELECT a.id AS article_id, a.title, a.is_top, a.publish_time, a.cover, a.url
FROM article a
JOIN article_column ac ON ac.article_id = a.id
JOIN `column` c ON c.id = ac.column_id AND c.code = 'home-hero' AND c.status = 1
JOIN template t ON t.id = c.template_id AND t.status = 1
WHERE a.status = 1 AND a.audit_status = 2 AND a.type IN (1, 2)
  AND a.publish_time <= NOW()
ORDER BY a.is_top DESC, a.publish_time DESC, a.id DESC
LIMIT 10;

-- 4. 首页专家视野与关于协会专家委员会的投放归属。
SELECT c.code, c.id AS column_id, a.id AS article_id, a.title,
       a.status, a.audit_status, a.publish_time,
       CASE WHEN ac.article_id IS NULL THEN 0 ELSE 1 END AS currently_selected,
       COUNT(p.id) AS publication_rows
FROM `column` c
JOIN article a ON a.id IN (
  SELECT article_id FROM article_column WHERE column_id = c.id
  UNION
  SELECT article_id FROM article_column_publish WHERE column_id = c.id
)
LEFT JOIN article_column ac ON ac.article_id = a.id AND ac.column_id = c.id
LEFT JOIN article_column_publish p ON p.article_id = a.id AND p.column_id = c.id
WHERE c.code IN ('home-expert-insights', 'about-experts')
GROUP BY c.code, c.id, a.id, a.title, a.status, a.audit_status, a.publish_time, ac.article_id
ORDER BY c.code, a.publish_time DESC, a.id DESC;

-- 5. 其他栏目关系漂移概览。确认修复仅作用轮播，保留旧父栏目聚合内容。
WITH relation_rows AS (
  SELECT article_id, column_id, 1 AS selected, 0 AS published FROM article_column
  UNION ALL
  SELECT p.article_id, p.column_id, 0, 1
  FROM article_column_publish p JOIN `column` c
    ON c.id = p.column_id AND c.template_id = p.template_id
), relations AS (
  SELECT article_id, column_id, MAX(selected) AS selected, MAX(published) AS published
  FROM relation_rows GROUP BY article_id, column_id
)
SELECT c.code, COUNT(*) AS related_articles,
       SUM(r.selected = 1 AND r.published = 0) AS selected_without_publication,
       SUM(r.selected = 0 AND r.published = 1) AS publication_without_selection
FROM relations r JOIN `column` c ON c.id = r.column_id
JOIN article a ON a.id = r.article_id
WHERE a.status = 1 AND a.audit_status = 2 AND a.type IN (1, 2)
GROUP BY c.code
HAVING selected_without_publication > 0 OR publication_without_selection > 0
ORDER BY c.code;

-- 6. 模板不匹配及重复发布记录；这两项会影响旧静态查询或排序。
SELECT p.article_id, p.column_id, c.code, p.template_id AS publication_template_id,
       c.template_id AS current_template_id
FROM article_column_publish p JOIN `column` c ON c.id = p.column_id
WHERE p.template_id <> c.template_id
ORDER BY c.code, p.article_id LIMIT 100;

SELECT p.article_id, p.column_id, c.code, p.template_id, COUNT(*) AS publication_rows
FROM article_column_publish p JOIN `column` c ON c.id = p.column_id
GROUP BY p.article_id, p.column_id, c.code, p.template_id
HAVING COUNT(*) > 1
ORDER BY publication_rows DESC, c.code LIMIT 100;
