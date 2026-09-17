# MIIC 门户静态化 HTTP API

本文档面向后台管理系统、发布脚本和运维人员，说明如何启动静态化服务、触发生成任务、查询进度及处理错误。

## 1. 服务信息

- 默认监听地址：`http://127.0.0.1:9143`
- 默认配置文件：`backend/config.yaml`
- 响应格式：`application/json; charset=utf-8`
- 时间格式：RFC 3339，例如 `2026-08-27T09:30:00+08:00`
- 健康检查无需鉴权；其余接口均需要 Bearer Token。

服务默认只监听本机回环地址。需要由其他服务器调用时，建议通过受控反向代理开放，并限制来源 IP，不要直接暴露到公网。

### 1.1 必需环境变量

```powershell
$env:MIIC_DB_DSN = 'miic_static:密码@tcp(127.0.0.1:3306)/miic_portal?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai'
$env:MIIC_STATIC_TOKEN = '请替换为足够长的随机字符串'
```

真实 DSN、密码和 Token 不应写入仓库。数据库账号只需要 `miic_portal` 的读取权限。

### 1.2 启动服务

```powershell
cd D:\WebstormProjects\miic-portal\backend
go run ./cmd/miic-static serve --config config.yaml
```

服务启动时会连接数据库并解析配置的资讯页面。如果数据库不可用、页面不存在或配置无效，服务不会启动。

## 2. 鉴权

除 `GET /healthz` 外，请求头必须包含：

```http
Authorization: Bearer {MIIC_STATIC_TOKEN}
```

示例：

```bash
curl -H "Authorization: Bearer $MIIC_STATIC_TOKEN" \
  http://127.0.0.1:9143/api/static/jobs/{job_id}
```

注意：

- Token 区分大小写。
- 不要把 Token 放在 URL 查询参数中。
- Token 缺失、格式错误或不匹配时返回 `401`，同时响应头包含 `WWW-Authenticate: Bearer`。

## 3. 接口总览

### 3.1 后台批量任务

以下接口立即返回 `202 Accepted`，实际生成在后台执行：

| 方法 | 地址 | 任务类型 | 说明 |
| --- | --- | --- | --- |
| POST | `/api/static/site` | `site` | 生成文章、四个主页面和共享动态数据，校验后原子发布整站 |
| POST | `/api/static/pages` | `pages` | 生成四个主页面及 `generated-content.js` |
| POST | `/api/static/lists` | `lists` | 生成全部通用栏目分页 |
| POST | `/api/static/articles` | `articles` | 生成全部内部文章详情及共享动态数据 |

同一服务实例同时只允许一个后台批量任务。已有批量任务处于 `queued` 或 `running` 时，再次提交会返回 `409 Conflict`。

整站发布 `/site` 不产出资讯独立列表页；资讯栏目直接在 `news.html` 内筛选。只有显式调用 `/lists` 或 `/list` 时才生成 `list/{column_id}/{page}.html`。

### 3.2 同步操作

以下接口等待操作完成后返回结果：

| 方法 | 地址 | 说明 |
| --- | --- | --- |
| POST | `/api/static/page` | 生成一个主页面及共享动态数据 |
| POST | `/api/static/list` | 按栏目名称或栏目 ID 生成分页 |
| POST | `/api/static/article` | 生成文章详情，并默认联动栏目分页、相关主页面和共享动态数据 |
| DELETE | `/api/static/article` | 下架后刷新旧栏目与相关主页面，最后删除年月详情文件 |
| GET | `/api/static/jobs/{id}` | 查询后台任务状态和进度 |
| DELETE | `/api/static/jobs/{id}` | 请求取消后台任务 |

同步生成受配置中的 `server.request_timeout` 限制，默认 `10m`。

## 4. 通用查询参数

### 4.1 `path`

所有生成和删除接口都支持可选参数 `path`，用于覆盖本次操作的输出根目录。

```text
?path=D%3A%5Cpublish%5Cmiic-portal
```

规则：

- 必须是绝对路径。
- 不能是 Windows 磁盘根目录，例如 `D:\`。
- 不能等于源码目录。
- 位于源码目录内部时，只允许使用源码根目录下的 `dist/{名称}`。
- 可以使用源码目录之外的独立发布目录。
- 省略时使用 YAML 中的 `site.dist_root`。

生产环境建议固定使用配置文件中的发布目录。调用方传入 `path` 时，应先进行自身的目录白名单校验。

### 4.2 `gray`

`gray` 仅适用于 `/site`、`/pages` 和 `/page`：

| 值 | 含义 |
| --- | --- |
| `1` | 开启整站置灰 |
| `2` | 恢复彩色，也是默认值 |

其他值返回 `400`。`/lists`、`/articles`、`/list` 和 `/article` 不支持 `gray=1`。

### 4.3 `refresh` 与自动关联栏目

`POST/DELETE /api/static/article` 默认执行联动刷新，不需要传 `refresh` 或 `column_id`。旧的 `refresh=related` 仍兼容；只有运维单独修复详情文件时才使用 `refresh=none`。文章接口不再读取 `column_id`；栏目列表接口的同名参数仍然有效。

- 后端根据文章 ID 查询全部关联栏目，支持一篇文章对应多个栏目；文章下架、删除或发布关系软删除后，仍可利用保留的关系记录。
- 文章换栏目或关系已物理删除时，从本次 `path` 输出目录的旧栏目分页和主页面中补查引用，合并去重后刷新。整站未生成独立列表、仅主页面保留引用时，会刷新该页面的全部有效栏目。
- 关联刷新期间将栏目记录保存在输出目录的 `.article-refresh/{文章ID}.json`，全部刷新成功后清除；失败重试仍能找回已经被替换列表中的旧关系。
- 下架或删除前必须先提交数据库状态；文章仍可发布时返回 `409`。没有剩余关系或详情文件时也可重复调用，不要求旧栏目 ID。

### 4.4 中文参数编码

页面名和栏目名包含中文时应使用 UTF-8 URL 编码。PowerShell 的 `Invoke-RestMethod` 可以直接使用中文；其他客户端建议通过标准 URL 编码方法构造查询字符串。

## 5. 健康检查

### `GET /healthz`

无需鉴权，仅表示 HTTP 服务正在运行。

响应 `200 OK`：

```json
{
  "ok": true
}
```

健康检查不会在每次请求时重新执行数据库查询。需要验证实际生成链路时，应另外执行一次受控的单页生成。

## 6. 批量生成

### 6.1 生成完整站点

```http
POST /api/static/site?gray=2
Authorization: Bearer {token}
```

生成过程：复制静态脚手架、生成内部文章详情、生成资讯主页和共享动态数据、校验必需文件与内部链接，最后原子替换发布目录。失败时保留上一版完整发布目录。

### 6.2 生成全部主页面

```http
POST /api/static/pages?gray=2
Authorization: Bearer {token}
```

生成 `news.html`，重新发布 `business.html`、`platforms.html`、`about.html`，并更新 `generated-content.js`。

### 6.3 生成全部栏目分页

```http
POST /api/static/lists
Authorization: Bearer {token}
```

输出格式：`list/{column_id}/{page}.html`。每页条数由 `site.page_size` 控制。

该接口只替换发布目录中的 `list/`；已发布的首页、其他主页面、`generated-content.js`、CSS、JavaScript 和图片资源不会被重写。若发布目录尚未初始化，只补齐缺失的基础文件。

### 6.4 生成全部文章详情

```http
POST /api/static/articles
Authorization: Bearer {token}
```

内部文章输出到 `article/YYYY/MM/{article_id}.html`。外部链接文章不生成本地详情文件。

### 6.5 批量任务受理响应

上述四个接口成功受理时均返回 `202 Accepted`：

```json
{
  "ok": true,
  "job": {
    "id": "13f3c8aa62fe4f6c9b1a77b68a0cdb18",
    "kind": "site",
    "status": "queued",
    "created_at": "2026-08-27T09:30:00+08:00",
    "updated_at": "2026-08-27T09:30:00+08:00",
    "progress": {
      "stage": "等待执行",
      "processed": 0,
      "total": 0,
      "generated_files": 0
    },
    "status_url": "/api/static/jobs/13f3c8aa62fe4f6c9b1a77b68a0cdb18",
    "cancel_url": "/api/static/jobs/13f3c8aa62fe4f6c9b1a77b68a0cdb18"
  }
}
```

`202` 只代表任务已受理，不代表生成成功。调用方必须继续轮询 `status_url`，直到任务进入终态。

除删除接口外，所有成功生成结果统一包含以下公共字段。批量接口通过任务对象的 `result` 返回，单项接口直接在同步响应的 `result` 中返回：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `generated_at` | string | 生成完成时间，RFC 3339 格式 |
| `duration_seconds` | number | 总耗时，单位为秒；成功时最小为 `0.001` |
| `generated_files` | integer | 本次实际生成的 HTML 文件总数 |
| `generated_details` | integer | 本次生成的详情页数量，没有则为 `0` |
| `generated_lists` | integer | 本次生成的栏目分页数量，没有则为 `0` |
| `output` | string | 本次操作的服务器输出路径 |

不同任务还会返回下列业务统计字段：

| 字段 | 说明 |
| --- | --- |
| `generated_pages` | 生成的主页面数量 |
| `generated_columns` | 生成的栏目数量 |
| `generated_articles` | 生成的内部文章详情数量 |
| `total_items` | 处理的内容总数 |
| `total_pages` | 生成的分页总数 |
| `gray` | 最终置灰状态，`1` 为置灰、`2` 为彩色 |

## 7. 单项生成与删除

### 7.1 生成指定主页面

```http
POST /api/static/page?name=资讯动态&gray=2
Authorization: Bearer {token}
```

`name` 支持以下中文名和英文代码：

| 中文名 | 英文代码 | 输出文件 |
| --- | --- | --- |
| 资讯动态 | `news` | `news.html` |
| 核心业务 | `business` | `business.html` |
| 服务平台 | `platforms` | `platforms.html` |
| 关于我们 | `about` | `about.html` |

成功响应 `200 OK`：

```json
{
  "ok": true,
  "result": {
    "generated_at": "2026-08-27T09:31:00+08:00",
    "duration_seconds": 0.18,
    "generated_files": 1,
    "generated_details": 0,
    "generated_lists": 0,
    "output": "D:\\publish\\miic-portal\\news.html",
    "generated_pages": 1,
    "gray": "2"
  }
}
```

### 7.2 生成指定栏目分页

按栏目名称：

```http
POST /api/static/list?column_name=招聘信息
Authorization: Bearer {token}
```

按栏目 ID：

```http
POST /api/static/list?column_id=401
Authorization: Bearer {token}
```

同时提供 `column_name` 和 `column_id` 时优先使用 `column_name`。全局存在同名栏目时返回 `409`，建议此时改用 `column_id`。

成功响应 `200 OK`：

```json
{
  "ok": true,
  "result": {
    "generated_at": "2026-08-27T09:32:00+08:00",
    "duration_seconds": 0.12,
    "generated_files": 1,
    "generated_details": 0,
    "generated_lists": 1,
    "column_id": 401,
    "total_items": 3,
    "total_pages": 1,
    "page_size": 10,
    "output": "D:\\publish\\miic-portal\\list\\401"
  }
}
```

### 7.3 生成指定文章详情

```http
POST /api/static/article?id=3001
Authorization: Bearer {token}
```

`id` 必须是正整数，且文章必须满足已发布、已审核、未删除、`type=1/2` 并存在有效栏目发布关系。默认执行顺序为：生成详情 → 刷新后端自动查出的全部当前及旧栏目分页 → 刷新相关主页面 → 重写 `generated-content.js`。外部链接文章不生成本地详情。

文章从栏目 `402` 移到 `401` 时：

```http
POST /api/static/article?id=3001
Authorization: Bearer {token}
```

仅运维修复详情文件、明确不希望联动时：

```http
POST /api/static/article?id=3001&refresh=none
Authorization: Bearer {token}
```

成功响应 `200 OK`：

```json
{
  "ok": true,
  "result": {
    "generated_at": "2026-08-27T09:33:00+08:00",
    "duration_seconds": 0.08,
    "generated_files": 4,
    "generated_details": 1,
    "generated_lists": 1,
    "article_id": 3001,
    "column_id": 401,
    "output": "D:\\publish\\miic-portal\\article\\2026\\08\\3001.html",
    "refreshed_column_ids": [401],
    "refreshed_pages": ["news"]
  }
}
```

### 7.4 删除指定文章详情

```http
DELETE /api/static/article?id=3001
Authorization: Bearer {token}
```

调用前必须先在数据库取消发布或删除文章，服务自行根据文章 ID 和旧静态列表查找关联栏目，无需传入 `column_id`。服务按“刷新旧栏目分页 → 刷新相关主页面及共享动态数据 → 删除详情”的顺序执行，避免列表保留 404 链接。接口只处理静态文件，不修改数据库；未找到详情文件时仍返回 `200`，其中 `deleted` 为 `false`。

只有清理孤立详情文件时才可显式调用 `DELETE /api/static/article?id=3001&refresh=none`，此模式不会刷新任何引用。

```json
{
  "ok": true,
  "result": {
    "deleted_at": "2026-08-27T09:34:00+08:00",
    "duration_seconds": 0.1,
    "generated_files": 3,
    "generated_details": 0,
    "generated_lists": 1,
    "article_id": 3001,
    "deleted": true,
    "deleted_paths": [
      "D:\\publish\\miic-portal\\article\\2026\\08\\3001.html"
    ],
    "refreshed_column_ids": [401],
    "refreshed_pages": ["news"]
  }
}
```

## 8. 任务查询与取消

### 8.1 查询任务

```http
GET /api/static/jobs/{job_id}
Authorization: Bearer {token}
```

任务状态：

| 状态 | 是否终态 | 说明 |
| --- | --- | --- |
| `queued` | 否 | 已受理，等待执行 |
| `running` | 否 | 正在生成 |
| `succeeded` | 是 | 生成成功，查看 `result` |
| `failed` | 是 | 生成失败，查看 `error` |
| `interrupted` | 是 | 被调用方、空闲超时或最大时长限制中断 |

运行中示例：

```json
{
  "ok": true,
  "job": {
    "id": "13f3c8aa62fe4f6c9b1a77b68a0cdb18",
    "kind": "site",
    "status": "running",
    "created_at": "2026-08-27T09:30:00+08:00",
    "started_at": "2026-08-27T09:30:00+08:00",
    "updated_at": "2026-08-27T09:30:05+08:00",
    "progress": {
      "stage": "生成文章详情",
      "processed": 18,
      "total": 40,
      "generated_files": 17
    },
    "status_url": "/api/static/jobs/13f3c8aa62fe4f6c9b1a77b68a0cdb18",
    "cancel_url": "/api/static/jobs/13f3c8aa62fe4f6c9b1a77b68a0cdb18"
  }
}
```

成功终态会增加 `finished_at` 和 `result`；失败或中断会增加 `finished_at` 和 `error`。`updated_at` 在任务开始、进度变化和终态写入时更新。任务成功后，`job.progress.generated_files` 与 `job.result.generated_files` 保持一致。

建议轮询间隔为 1–2 秒，避免高频请求。任务记录保存在当前服务进程内，服务重启后旧任务 ID 不再可查询。

### 8.2 取消任务

```http
DELETE /api/static/jobs/{job_id}
Authorization: Bearer {token}
```

取消是协作式的。活动任务收到取消信号时返回 `202 Accepted`；已经进入终态的任务返回 `200 OK`。任务可能短暂仍显示 `running`，调用方应继续查询，直到状态变为 `interrupted`。旧任务完全退出前仍占用批量任务槽，避免新旧任务同时写入发布目录。

后台任务还受以下配置控制：

- `server.batch_idle_timeout`：长时间无进度时取消，默认 `15m`。
- `server.batch_max_duration`：单个任务最大执行时间，默认 `6h`。

## 9. 错误响应与状态码

统一错误结构：

```json
{
  "ok": false,
  "error": "错误说明"
}
```

| 状态码 | 典型场景 | 调用方处理建议 |
| --- | --- | --- |
| `400 Bad Request` | 参数缺失、ID 非正整数、页面名无效、`gray`/`refresh` 无效、输出路径危险 | 修正请求后重试 |
| `401 Unauthorized` | Token 缺失或错误 | 检查 `Authorization` 请求头 |
| `404 Not Found` | 任务、栏目或已发布文章不存在 | 检查 ID、名称和发布状态 |
| `405 Method Not Allowed` | HTTP 方法错误 | 使用响应头 `Allow` 指定的方法 |
| `409 Conflict` | 已有批量任务运行、文件锁冲突、栏目名称不唯一、删除时文章仍可发布 | 等待任务结束、改用明确栏目 ID，或先提交数据库下架 |
| `500 Internal Server Error` | 数据库、模板、文件系统、链接校验或其他生成失败 | 查看服务端日志；上一版整站发布目录会保留 |

后台任务内部失败时，最初的受理请求仍是 `202`，最终错误体现在任务查询结果的 `status: "failed"` 和 `error` 字段中。

## 10. 调用示例

### 10.1 PowerShell：生成整站并轮询到结束

```powershell
$baseUrl = 'http://127.0.0.1:9143'
$headers = @{ Authorization = "Bearer $env:MIIC_STATIC_TOKEN" }

$started = Invoke-RestMethod `
  -Method Post `
  -Uri "$baseUrl/api/static/site?gray=2" `
  -Headers $headers

$jobUrl = $baseUrl + $started.job.status_url

do {
  Start-Sleep -Seconds 1
  $current = Invoke-RestMethod -Method Get -Uri $jobUrl -Headers $headers
  Write-Host $current.job.status $current.job.progress.stage
} while ($current.job.status -in @('queued', 'running'))

if ($current.job.status -ne 'succeeded') {
  throw "静态化失败：$($current.job.error)"
}

$current.job.result
```

### 10.2 curl：生成单篇文章

Windows PowerShell 中建议明确使用 `curl.exe`：

```powershell
curl.exe -X POST `
  -H "Authorization: Bearer $env:MIIC_STATIC_TOKEN" `
  "http://127.0.0.1:9143/api/static/article?id=3001"
```

### 10.3 curl：指定发布目录并开启置灰

```powershell
$encodedPath = [uri]::EscapeDataString('D:\publish\miic-portal')
curl.exe -X POST `
  -H "Authorization: Bearer $env:MIIC_STATIC_TOKEN" `
  "http://127.0.0.1:9143/api/static/site?path=$encodedPath&gray=1"
```

## 11. 推荐接入流程

1. 调用 `GET /healthz` 确认服务进程可访问。
2. 携带 Bearer Token 提交 `/api/static/site`。
3. 保存返回的任务 ID，并每 1–2 秒查询 `status_url`。
4. 仅在状态为 `succeeded` 时认定发布成功。
5. 状态为 `failed` 或 `interrupted` 时记录 `error`，不要覆盖调用方自己的上一版发布标记。
6. 新增或修改文章后调用 `POST /api/static/article?id={id}`；换栏目时后端自动补查旧关系，无需传 `column_id`。
7. 下架或删除文章时先提交数据库状态，再调用 `DELETE /api/static/article?id={id}`；接口会自动刷新引用，无需再调用 `/pages`。

生产发布优先使用 `/site`，因为它会在 staging 目录内完成生成与校验后再原子替换整站。`/page`、`/list` 和 `/article` 更适合内容变更后的增量更新。
