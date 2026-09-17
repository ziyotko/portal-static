# MIIC 门户静态化数据录入说明书（运维版）

本文档说明当前 `miic-static` 程序实际识别什么数据、如何录入，以及哪些数据会被过滤、跳过或导致生成失败。

适用数据库：`miic_portal`。表结构沿用 CAAM 门户的 `page`、`column`、`article`、`article_column_publish`、`article_attachment`。

> 本文以当前代码为准。后台字段名称可能与数据库字段名称不同，运维人员应以文中的数据库值核对最终保存结果。

## 1. 程序识别一条内容的完整条件

一条文章要被静态化程序读取，必须同时满足：

1. 所属页面：`page.status = 1` 且 `page.deleted_at IS NULL`。
2. 所属栏目：`column.status = 1` 且 `column.deleted_at IS NULL`。
3. 文章：`article.status = 1`、`article.audit_status = 2`、`article.deleted_at IS NULL`。
4. 文章类型：`article.type` 只能为 `1` 或 `2`。
5. 发布时间：`article.publish_time` 必须填写有效时间，不能为 `NULL`。
6. 发布关系：必须存在 `article_column_publish` 记录，且关系的 `deleted_at IS NULL`。

任何一项不满足，文章都不会进入对应栏目。

程序当前使用的固定值：

| 字段 | 程序认可的值 | 含义 |
| --- | --- | --- |
| `page.status` | `1` | 页面启用 |
| `column.status` | `1` | 栏目启用 |
| `article.status` | `1` | 文章已发布 |
| `article.audit_status` | `2` | 文章已审核通过 |
| `article.type` | `1` | 普通内容文章 |
| `article.type` | `2` | 视频内容文章 |

## 2. 页面数据

### 2.1 页面基本要求

页面通过中文 `page.name` 精确匹配。名称前后空格会被请求参数去掉，但数据库中的页面名称不应包含多余空格。

| 页面名称 | 当前用途 | 是否必须 |
| --- | --- | --- |
| `资讯动态` | 资讯栏目、中心重要动态、文章分类上下文 | 必须；缺失时程序无法正常启动或生成 |
| `核心业务` | 读取业务分类和服务详情 | 建议必须；缺失时业务详情为空 |
| `服务平台` | 当前平台主体内容仍是固定页面 | 建议保留；当前不读取其栏目内容 |
| `关于我们` | 读取招聘信息和信息公开 | 建议必须；缺失时两个动态列表为空 |

每个被程序使用的中文页面名必须只有一条有效记录。相同名称存在两条 `status=1`、`deleted_at IS NULL` 的页面时，程序会终止，避免读取错误页面。

`page.code`、`route_path` 当前不参与内容查询，但建议分别保持为：

| `page.name` | 建议 `page.code` | 建议 `route_path` |
| --- | --- | --- |
| 资讯动态 | `news` | `news.html` |
| 核心业务 | `business` | `business.html` |
| 服务平台 | `platforms` | `platforms.html` |
| 关于我们 | `about` | `about.html` |

## 3. 栏目数据通用规则

| 字段 | 录入要求 |
| --- | --- |
| `name` | 填写面向用户展示的中文栏目名；配置指定的栏目必须精确一致 |
| `code` | 使用小写英文、数字和连字符；核心业务子栏目必须填写 |
| `page_id` | 必须指向正确页面 |
| `parent_id` | 一级栏目填 `0`；子栏目填真实父栏目 ID |
| `sort` | 数字越小越靠前；相同值按栏目 ID 升序 |
| `status` | 必须为 `1` |
| `deleted_at` | 必须为 `NULL` |

同一个页面、同一个父栏目下面，不能出现重复的有效栏目名称或重复的非空 `code`。检测到重复时，整次生成会失败。

## 4. 文章数据通用规则

### 4.1 字段说明

| 字段 | 必填 | 当前程序行为 |
| --- | --- | --- |
| `title` | 是 | 页面标题和列表标题；不应为空 |
| `type` | 是 | 只能使用 `1` 或 `2`；其他类型不读取 |
| `summary` | 建议 | 资讯摘要、重要动态摘要、服务详情简介 |
| `content` | 站内详情建议 | 站内文章正文或服务详情正文 |
| `status` | 是 | 必须为 `1` |
| `audit_status` | 是 | 必须为 `2` |
| `deleted_at` | 是 | 必须为 `NULL` |
| `publish_time` | 是 | 必须是有效日期时间，不能为 `NULL` |
| `cover` | 否 | 为空时使用本地占位图 |
| `url` | 否 | 合法绝对 HTTP(S) 地址表示外链；站内详情必须留空 |
| `author` | 否 | 详情页作者；发布关系中的作者可覆盖它 |
| `source` | 否 | 详情页来源；发布关系中的来源可覆盖它 |
| `default_color` | 否 | 默认标题颜色；发布关系中的颜色可覆盖它 |

### 4.2 发布时间特别说明

虽然数据库字段允许 `publish_time = NULL`，但当前 Go 程序按非空时间读取。只要查询结果中包含一条空发布时间文章，就可能导致整批查询失败。因此运维发布文章前必须填写发布时间。

### 4.3 正文安全规则

正文生成时会进行白名单清洗：

- 删除 `<script>`。
- 删除 `onclick` 等事件属性。
- 删除 `javascript:` 等不安全链接。
- 保留常用段落、标题、列表、链接、图片和表格。
- 保留受控的 `<video>`、`<source>` 标签。

不要依赖脚本、内嵌事件或不安全协议实现正文功能，它们不会进入静态页面。

## 5. 文章与栏目的发布关系

文章是否出现在栏目中，由 `article_column_publish` 决定，不是只看文章表。

| 字段 | 录入要求 |
| --- | --- |
| `column_id` | 必须是目标栏目真实 ID |
| `article_id` | 必须是目标文章真实 ID |
| `page_id` | 应与该栏目的 `page_id` 一致 |
| `article_title` | 建议与文章标题一致，便于后台排查 |
| `is_top` | `1` 为栏目内置顶，`0` 为普通 |
| `author` | 非空时覆盖 `article.author` |
| `source` | 非空时覆盖 `article.source` |
| `is_bold` | 控制该发布关系下的加粗标记 |
| `color` | 非空时覆盖 `article.default_color` |
| `deleted_at` | 必须为 `NULL` |

同一文章可以发布到多个栏目。程序会让它出现在每个栏目中，但只生成一份站内详情文件。

栏目内排序规则固定为：

1. `article_column_publish.is_top` 降序。
2. `article.publish_time` 降序。
3. `article.id` 降序。

不要只修改 `article.is_top` 来控制栏目排序；栏目页面实际使用发布关系表的 `is_top`。

## 6. 资讯动态录入规则

### 6.1 栏目层级

资讯标签读取 `资讯动态` 页面下 `parent_id = 0` 的全部有效栏目，并按 `sort, id` 排序。

推荐结构：

| 栏目名称 | `code` | `parent_id` | 用途 |
| --- | --- | --- | --- |
| 中心重要动态 | `important` | `0` | 四个主页面共享轮播，不显示为资讯筛选标签 |
| 最新 | `latest` | `0` | 真实栏目，不是程序自动聚合 |
| 中心动态 | `center` | `0` | 资讯筛选标签 |
| 行业动态 | `industry` | `0` | 资讯筛选标签 |
| 行业资讯 | `information` | `0` | 资讯筛选标签 |
| 成果发布 | `results` | `0` | 资讯筛选标签 |

除 `中心重要动态` 外，新增的一级栏目会自动成为资讯页标签；删除或停用后不再显示。栏目 `code` 用作前端筛选键，建议填写且保持唯一、稳定。

### 6.2 “最新”不是自动栏目

程序不会自动把所有资讯汇总到“最新”。需要显示在“最新”的文章，必须单独建立文章与“最新”栏目的发布关系。

推荐发布方式：

- 一篇中心新闻同时发布到“最新”和“中心动态”。
- 一篇行业新闻同时发布到“最新”和“行业动态”。
- 同一文章可同时发布到“中心重要动态”，用于共享轮播。

为了让详情页显示准确的业务分类，文章除发布到“最新”外，还应发布到一个真实分类栏目。

### 6.3 重点卡片与加载更多

- 每个筛选栏目排序后的第一条文章显示为重点卡片。
- 初始显示前 5 条。
- 每次点击“加载更多”增加 2 条。
- 栏目没有文章时显示空状态，不会使用演示文章填充正式站点。

### 6.4 中心重要动态

配置默认读取栏目名 `中心重要动态`，最多读取 3 条。

这同一份数据供以下页面共用：

- 资讯动态
- 核心业务
- 服务平台
- 关于我们

该栏目不存在时，页面仍可生成，但共享轮播为空；同名有效栏目超过一个时生成失败。

## 7. 核心业务详情录入规则

### 7.1 栏目层级

核心业务使用两级栏目：

```text
核心业务页面
├─ 一级业务分类（parent_id = 0）
│  └─ 服务子栏目（parent_id = 一级栏目ID）
```

一级栏目名称会显示为服务详情的分类名称。核心业务落地页本身仍是固定内容，因此一级栏目和服务子栏目的名称应与页面现有内容保持一致。

### 7.2 服务子栏目 `code`

服务详情地址为 `service-detail.html?id={code}`。子栏目的 `column.code` 必须与地址中的 `id` 完全一致。

当前固定页面使用以下 14 个代码：

| 一级分类 | 服务名称 | 必须使用的 `column.code` |
| --- | --- | --- |
| 信息技术服务 | 软件产品及定制开发服务 | `software` |
| 信息技术服务 | 系统集成服务 | `integration` |
| 信息技术服务 | 云资源服务 | `cloud` |
| 信息技术服务 | 信息系统运维服务 | `operations` |
| 信息技术服务 | 安全合规服务 | `security` |
| 数智化创新赋能服务 | 数据要素服务 | `data-elements` |
| 数智化创新赋能服务 | 数智化评价咨询服务 | `digital-evaluation` |
| 数智化创新赋能服务 | 数智化能力培育服务 | `digital-training` |
| 行业智库咨询服务 | 战略规划咨询服务 | `strategy` |
| 行业智库咨询服务 | 产业研究与决策咨询服务 | `industry-research` |
| 行业智库咨询服务 | 项目评估与论证咨询服务 | `project-evaluation` |
| 行业公共服务 | 公共信息服务 | `public-information` |
| 行业公共服务 | 行业交流服务 | `industry-exchange` |
| 行业公共服务 | 普惠赋能服务 | `inclusive-empowerment` |

子栏目 `code` 为空时，程序会记录警告并跳过该服务映射。代码填写错误时，对应详情地址会显示“暂无服务详情”。

### 7.3 服务详情文章

每个服务子栏目可以发布多篇文章，但当前详情页只使用排序后的第一篇：

- `title`：服务详情页标题。
- `summary`：顶部简介。
- `cover`：详情页主图。
- `content`：服务详情正文。

建议每个服务子栏目只保留一篇置顶的当前有效详情文章；历史版本应取消栏目发布关系或设为非发布状态。

## 8. 服务平台录入规则

当前服务平台的五个平台、平台介绍和查询参数均为固定页面内容，暂时不读取数据库平台栏目。

服务平台页面唯一读取的数据库内容是 `中心重要动态` 共享轮播。因此暂时不要为了平台主体内容自行新增数据库结构，后续有明确数据模型后再迁移。

## 9. 关于我们录入规则

关于我们页面的中心介绍和联系方式为固定内容。当前读取两个动态栏目：

| 栏目名称 | 推荐 `code` | 用途 |
| --- | --- | --- |
| 招聘信息 | `recruitment` | 招聘列表和招聘详情 |
| 信息公开 | `disclosure` | 预算、决算等公开内容 |

栏目名必须与 YAML 配置一致，默认分别为 `招聘信息`、`信息公开`。

### 9.1 招聘信息

推荐录入方式：

- `title`：岗位名称。
- `summary`：岗位简要说明，可为空。
- `content`：岗位职责、任职要求、应聘方式。
- `url`：留空，生成本站详情。
- 发布到“招聘信息”栏目。

如果填写了合法 HTTP(S) `url`，点击岗位会直接跳转外部地址，不生成本站详情。

### 9.2 信息公开

点击优先级固定为：

1. 文章 `url` 中的合法 HTTP(S) 地址。
2. `article_attachment` 中按 ID 排序的第一个附件。
3. 如果前两项都没有，进入本站文章详情。

因此一条信息公开内容应选择一种主要方式：

- 外部文件或网页：填写文章 `url`。
- 上传附件：文章 `url` 留空，确保正确附件是该文章附件列表中的第一条。
- 本站正文：文章 `url` 留空且不上传附件，在 `content` 中填写正文。

附件表当前没有发布状态和删除状态过滤，程序直接取该文章附件中 ID 最小的一条。废弃附件应从关系中清理，避免被误选。

## 10. URL、封面和附件路径

### 10.1 文章 `url`

只有同时满足下列条件才被识别为外链：

- 使用 `http://` 或 `https://`。
- 包含有效主机名。

站内文章请将 `url` 留空。不要填写 `/article/xxx`、`www.example.com` 或其他不完整地址；它们不会被识别为外链，程序会把文章当作站内详情处理。

### 10.2 封面 `cover`

支持：

- 完整 HTTP(S) 图片地址。
- 站点相对路径，例如 `assets/news-cover.jpg`。
- 上传目录相对路径。

封面为空时使用配置中的 `assets/miic-placeholder.png`。程序不会下载远程媒体文件；相对路径对应的文件必须能够随站点部署访问。

### 10.3 附件 `url`

附件地址必须是合法 HTTP(S) 地址或受控的站点相对路径。严禁录入 `javascript:`、`data:`、本机磁盘路径或无法访问的临时地址。

## 11. 哪些问题会导致什么结果

### 11.1 会直接导致启动或生成失败

- `资讯动态` 页面不存在、停用、软删除或存在有效重名页面。
- 同一页面、同一父栏目下存在重复有效栏目名或重复非空 `code`。
- `中心重要动态`、`招聘信息`、`信息公开`存在有效同名栏目。
- 被查询到的文章 `publish_time` 为 `NULL`。
- 数据库字段类型或表结构与当前程序不一致。
- 数据库不可连接、模板不可读取、发布目录不可写或内部链接校验失败。

### 11.2 不报错，但内容不会出现

- 文章 `status` 不是 `1`。
- 文章 `audit_status` 不是 `2`。
- 文章或发布关系已经软删除。
- 文章类型不是 `1` 或 `2`。
- 没有 `article_column_publish` 发布关系。
- 页面或栏目被停用、软删除。
- 核心业务子栏目没有文章或 `code` 为空。

### 11.3 会自动降级

- 封面为空：使用占位图。
- `中心重要动态`不存在：共享轮播为空。
- `核心业务`页面不存在：固定业务页仍发布，但数据库服务详情为空。
- `关于我们`页面或两个动态栏目不存在：招聘和信息公开显示空状态。
- 外链文章：保留外链，不生成本地详情文件。

## 12. 录入后的 SQL 自检

以下查询均为只读，可在 MySQL Workbench 中执行。

### 12.1 检查页面是否唯一有效

```sql
SELECT name, COUNT(*) AS valid_count
FROM page
WHERE status = 1
  AND deleted_at IS NULL
  AND name IN ('资讯动态', '核心业务', '服务平台', '关于我们')
GROUP BY name;
```

`资讯动态`必须返回且 `valid_count = 1`；其他已启用动态功能的页面也应为 `1`。

### 12.2 检查同级栏目重名

```sql
SELECT page_id, parent_id, TRIM(name) AS column_name, COUNT(*) AS duplicate_count
FROM `column`
WHERE status = 1 AND deleted_at IS NULL
GROUP BY page_id, parent_id, TRIM(name)
HAVING COUNT(*) > 1;
```

应返回 0 行。

### 12.3 检查同级栏目 `code` 重复

```sql
SELECT page_id, parent_id, TRIM(code) AS column_code, COUNT(*) AS duplicate_count
FROM `column`
WHERE status = 1
  AND deleted_at IS NULL
  AND TRIM(code) <> ''
GROUP BY page_id, parent_id, TRIM(code)
HAVING COUNT(*) > 1;
```

应返回 0 行。

### 12.4 检查可发布文章的空发布时间

```sql
SELECT id, title, status, audit_status, type
FROM article
WHERE status = 1
  AND audit_status = 2
  AND deleted_at IS NULL
  AND type IN (1, 2)
  AND publish_time IS NULL;
```

应返回 0 行。

### 12.5 检查无效发布关系

```sql
SELECT
  acp.id AS publish_id,
  acp.article_id,
  acp.column_id,
  a.id AS valid_article,
  c.id AS valid_column,
  p.id AS valid_page
FROM article_column_publish acp
LEFT JOIN article a
  ON a.id = acp.article_id
 AND a.status = 1
 AND a.audit_status = 2
 AND a.deleted_at IS NULL
 AND a.type IN (1, 2)
LEFT JOIN `column` c
  ON c.id = acp.column_id
 AND c.status = 1
 AND c.deleted_at IS NULL
LEFT JOIN page p
  ON p.id = c.page_id
 AND p.status = 1
 AND p.deleted_at IS NULL
WHERE acp.deleted_at IS NULL
  AND (a.id IS NULL OR c.id IS NULL OR p.id IS NULL);
```

应返回 0 行。

### 12.6 检查核心业务代码

```sql
SELECT parent.name AS category_name, child.id, child.name, child.code
FROM `column` child
JOIN `column` parent ON parent.id = child.parent_id
JOIN page p ON p.id = child.page_id
WHERE p.name = '核心业务'
  AND p.status = 1 AND p.deleted_at IS NULL
  AND parent.status = 1 AND parent.deleted_at IS NULL
  AND child.status = 1 AND child.deleted_at IS NULL
ORDER BY parent.sort, parent.id, child.sort, child.id;
```

逐项核对第 7.2 节列出的 14 个 `code`。

### 12.7 检查文章实际发布位置

```sql
SELECT
  a.id AS article_id,
  a.title,
  p.name AS page_name,
  c.name AS column_name,
  c.code AS column_code,
  acp.is_top,
  a.publish_time
FROM article a
JOIN article_column_publish acp
  ON acp.article_id = a.id
 AND acp.deleted_at IS NULL
JOIN `column` c
  ON c.id = acp.column_id
 AND c.status = 1
 AND c.deleted_at IS NULL
JOIN page p
  ON p.id = c.page_id
 AND p.status = 1
 AND p.deleted_at IS NULL
WHERE a.status = 1
  AND a.audit_status = 2
  AND a.deleted_at IS NULL
  AND a.type IN (1, 2)
ORDER BY p.name, c.sort, acp.is_top DESC, a.publish_time DESC, a.id DESC;
```

## 13. 推荐录入流程

1. 先确认页面和栏目层级正确。
2. 新建或编辑文章，填写标题、类型、发布时间和业务内容。
3. 将文章状态设为已发布，对应数据库 `status = 1`。
4. 完成审核，对应数据库 `audit_status = 2`。
5. 建立文章与目标栏目的发布关系。
6. 需要栏目置顶时，在发布关系上设置 `is_top = 1`。
7. 执行第 12 节 SQL 自检。
8. 生成正式站点并查看返回的 `generated_files`、`generated_details` 和错误信息。

正式生成命令：

```powershell
cd D:\WebstormProjects\miic-portal\backend
$env:MIIC_DB_DSN='用户名:密码@tcp(127.0.0.1:3306)/miic_portal?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai'
go run ./cmd/miic-static generate --config config.yaml
```

演示预览命令（不读取数据库）：

```powershell
cd D:\WebstormProjects\miic-portal\backend
go run ./cmd/miic-static preview --config config.example.yaml
```

## 14. 发布前快速检查表

- [ ] `资讯动态`页面唯一、启用、未删除。
- [ ] 同级栏目名称和 `code` 没有重复。
- [ ] 所有待发布文章的 `status = 1`。
- [ ] 所有待发布文章的 `audit_status = 2`。
- [ ] 文章类型为 `1` 或 `2`。
- [ ] 发布时间不为空。
- [ ] 文章已发布到正确栏目。
- [ ] 栏目置顶设置在 `article_column_publish.is_top`。
- [ ] 核心业务子栏目 `code` 与详情 URL 参数一致。
- [ ] 站内文章 `url` 留空，外链使用完整 HTTP(S) 地址。
- [ ] 信息公开附件顺序正确，第一条是实际要打开的文件。
- [ ] 封面或媒体路径在部署环境中可访问。
- [ ] SQL 自检没有返回异常记录。

本地示例结构和示例数据可参考 [db/seed.sql](db/seed.sql)，但生产内容应通过内容管理后台录入，不应直接复制示例文章。
