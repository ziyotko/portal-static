# 数据库模板管理与运维交接

Portal CMS 生产模板以 `template` 表为唯一权威来源。新版数据模型不含 `page` 表；`portal-static` 在每次生成操作开始时按明确选择器读取模板快照，仅执行只读查询。

## 1. 模板角色与绑定

`portal-static` 的 `home`、`about`、`list`、`article` 等 Key 只是内部角色，不是数据库 ID。每个角色按以下顺序选择一条模板：

1. 配置了 `paths.template_codes.<key>` 时，按唯一 `template.code` 选择；
2. 否则按适配器定义的唯一 `template.name` 选择；
3. 同时要求 `status=1`、`type` 与角色匹配、`source_code` 非空；
4. 找不到或匹配多条都立即失败。

`column.template_id` 表示栏目归属的主模板，用于数据隔离和父子栏目校验，不是栏目列表页渲染模板。列表页使用唯一绑定的 `type=column` 公共模板；详情页使用唯一绑定的 `type=detail` 公共模板。这与当前 `dia-platform` 的 `ColumnService.GetColumnPublishes` 和详情预览规则一致。`article_column_publish.template_id` 必须与关联栏目的 `template_id` 一致才视为有效发布关系。

## 2. 热加载和校验

每次生成操作都会重新读取 `id/name/code/type/route_path/status/source_code/layout`，执行以下校验后将源码写入受控临时文件：

- 选择器唯一、模板启用、类型正确；
- 源码非空且不超过 2 MiB；
- 源码是合法 Go `html/template`；
- `route_path` 能安全映射到授权输出目录。

操作内使用同一份快照，下一次操作会看到后台新版本。任何错误都不会回退到 preview 模板。

## 3. 只读检查

```sql
SELECT id, name, code, type, route_path, status,
       CHAR_LENGTH(source_code) AS source_length,
       layout
FROM template
ORDER BY type, id;

SELECT c.id, c.name, c.parent_id, c.template_id, c.route_path, c.status,
       t.name AS owning_template, t.type AS owning_type, t.status AS template_status
FROM `column` c
LEFT JOIN template t ON t.id = c.template_id
ORDER BY c.template_id, c.parent_id, c.sort, c.id;

SELECT acp.id, acp.article_id, acp.column_id, acp.template_id,
       c.template_id AS column_template_id
FROM article_column_publish acp
LEFT JOIN `column` c ON c.id = acp.column_id
WHERE c.id IS NULL OR c.status <> 1 OR acp.template_id <> c.template_id;
```

最后一个查询正常应返回零行。静态化账号建议仅授予 `SELECT`；本服务不会修复或删除生产数据。

## 4. 路由和专题

- 主页面：`template.route_path` 映射到物理 HTML；`/` 为 `index.html`，无扩展名的 `/about` 为 `about.html`，尾部 `/` 为目录内 `index.html`。
- 栏目页：使用与后台相同的 `JoinAccessPath(column.route_path, listTemplate.route_path)` 规则。
- 详情页：`detailTemplate.route_path/{article_id}.html`。
- 专题页：`type=special` 模板直接使用自身 `route_path`；专题 ID 就是 `template.id`。

为避免破坏已有站点，生成器保留原有主页面、`list/<column_id>/...` 和日期型详情文件，同时发布上述路由别名。重复路由、路径穿越、输出边界逃逸和符号链接逃逸会失败，不会静默覆盖。

## 5. 发布与回滚

1. 记录待修改模板的原始字段；
2. 在后台更新 `source_code`、状态或路由；
3. 运行只读检查并生成到 `dist_root` 下的隔离子目录；
4. 检查 HTML、资源、后台预览 URL 与生成数量；
5. 验收后生成正式目录。

回滚时恢复原模板字段并重新生成。不要手工修改生成后的 HTML，也不要为兼容旧代码恢复 `page` 表或已删除字段。

## 6. 常见错误

- `template ... not found`：选择器没有匹配模板；
- `selector is not unique`：选择器匹配多条；
- `is disabled`：模板未启用；
- `type is ..., want ...`：模板角色与类型不符；
- `has empty source_code`：模板正文为空；
- `parse database template`：模板语法无效；
- `resolve to the same static route`：两个内容对象会覆盖同一物理文件。
