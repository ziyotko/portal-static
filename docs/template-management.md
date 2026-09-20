# 数据库模板管理与运维交接

Portal CMS 站点的生产模板以数据库为唯一权威来源。管理后台维护 `template.source_code`，页面通过 `page.template_id` 绑定模板；`portal-static` 在每次生成操作开始时读取一份完整模板快照。

本机制只读取数据，不要求修改表结构。静态化生产账号建议仅授予 `SELECT`。

## 1. 生效链路

```text
后台“模板管理”保存 source_code
  → 后台“栏目管理”中的页面绑定 template_id
  → portal-static 开始一次生成操作
  → 校验页面、模板状态、类型、唯一性和模板语法
  → 使用本次快照生成 HTML
```

保存模板不会自动改写已经生成的 HTML，也不需要重启静态化服务。保存后执行对应页面、全部栏目、全部详情或全站生成即可。

## 2. 数据要求

有效生产模板必须同时满足：

- 页面未删除且已启用：`page.deleted_at IS NULL AND page.status=1`；
- 页面已绑定模板：`page.template_id` 指向有效记录；
- 模板未删除且已启用：`template.deleted_at IS NULL AND template.status=1`；
- `template.source_code` 非空且是合法 Go `html/template`；
- 主页面的页面/模板类型都是 `home`；
- 通用栏目页面/模板类型都是 `column`；
- 通用详情页面/模板类型都是 `detail`；
- 同一个适配器角色只匹配一条有效页面记录。

CAAM 主页面名称为：首页、协会概况、协会工作、统计数据、会员专区、党建专区。MIIC 主页面名称为：资讯动态、核心业务、服务平台、关于我们。两个站点都另有唯一的通用栏目页和通用详情页。

## 3. 运维只读检查

在目标 schema 中执行以下查询，核对页面与模板绑定。查询不会修改数据：

```sql
SELECT
  p.id AS page_id,
  p.name AS page_name,
  p.page_type,
  p.status AS page_status,
  t.id AS template_id,
  t.name AS template_name,
  t.type AS template_type,
  t.status AS template_status,
  CHAR_LENGTH(t.source_code) AS source_length
FROM page AS p
LEFT JOIN template AS t ON t.id = p.template_id
WHERE p.deleted_at IS NULL
ORDER BY p.id;
```

重点检查：`template_id` 不为空、两边状态为 `1`、页面类型与模板类型一致、`source_length` 大于零。

## 4. 数据变更规范

模板初始化或修复只允许使用数据操作：

- `INSERT` 新增模板或缺失的通用页面；
- `UPDATE template SET source_code=...` 更新模板正文；
- `UPDATE page SET template_id=...` 修复页面绑定；
- 必要时更新已有记录的 `status`、`page_type` 或 `type`，但必须先核对业务含义。

禁止为本功能执行 `ALTER TABLE`、`CREATE TABLE`、`DROP TABLE` 或 `TRUNCATE TABLE`。执行前导出目标表数据，使用事务，在提交前运行只读检查查询并保留变更清单。不要把 SQL dump、密码或生产模板正文提交到 Git。

## 5. 发布与回滚

模板发布建议按以下顺序：

1. 备份 `page`、`template` 的数据或记录待修改行的原值。
2. 在事务中更新 `source_code` 或绑定。
3. 运行只读检查，确认状态、类型、绑定和源码长度正确。
4. 提交事务，在隔离输出目录执行一次生成并抽查。
5. 验收通过后生成正式目录。

回滚时恢复原 `source_code` 或原 `template_id`，然后重新生成受影响范围。不要手工修改生成后的 HTML 作为回滚；下次生成会覆盖它。

## 6. 常见错误

- `has empty source_code`：模板正文为空。
- `type is ..., want ...`：页面绑定了错误类型的模板。
- `is ambiguous`：同一通用类型或同名主页面存在多条启用记录。
- `is unavailable`：页面/模板缺失、未绑定、已禁用或已删除。
- `parse database template`：模板语法无效。

这些错误都不会回退到本地旧模板，避免后台显示一个版本而实际生成另一个版本。
