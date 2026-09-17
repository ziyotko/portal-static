# HTTP API 契约

## 1. 基本约定

- 默认基础地址：`http://<host>:<port>`。
- `GET /healthz` 不需要鉴权。
- 其他端点必须携带 `Authorization: Bearer <token>`。
- 请求参数使用 query string，不要求 JSON 请求体。
- 成功响应包含 `"ok": true`；失败包含 `"ok": false` 和 `error`。
- 不允许新增站点私有路由；站点差异通过适配器实现。

后台应代理调用这些端点，浏览器不应直接持有静态化 Token。

## 2. 端点总览

| 方法 | 路径 | 类型 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/healthz` | 同步 | 存活检查 |
| `POST` | `/api/static/site` | 异步 | 生成完整站点 |
| `POST` | `/api/static/pages` | 异步 | 生成全部主页面 |
| `POST` | `/api/static/lists` | 异步 | 生成全部栏目列表 |
| `POST` | `/api/static/articles` | 异步 | 生成全部文章详情 |
| `POST` | `/api/static/page` | 同步 | 生成一个主页面 |
| `POST` | `/api/static/list` | 同步 | 生成一个栏目列表 |
| `POST` | `/api/static/article` | 同步 | 生成一个文章详情，可关联刷新 |
| `DELETE` | `/api/static/article` | 同步 | 删除一个文章详情，可关联刷新 |
| `GET` | `/api/static/jobs/{id}` | 同步 | 查询批任务 |
| `DELETE` | `/api/static/jobs/{id}` | 同步 | 取消批任务 |

## 3. 公共参数

| 参数 | 适用端点 | 说明 |
| --- | --- | --- |
| `path` | 所有生成/删除操作 | 可选，绝对非根输出目录；缺省使用 `paths.dist_root` |
| `gray` | `site`、`pages`、`page` | 可选，字符串 `1` 开启、`2` 关闭；缺省为关闭 |
| `refresh` | `article` | 可选，`related` 或 `none`；缺省为 `related` |

`lists` 和 `articles` 批处理不接受 `gray=1`。

## 4. 批处理

示例：

```bash
curl -X POST \
  -H "Authorization: Bearer $STATIC_TOKEN" \
  'http://127.0.0.1:9142/api/static/site?path=/srv/portal-static/caam/site&gray=2'
```

成功返回 `202 Accepted`：

```json
{
  "ok": true,
  "job": {
    "id": "2bb82b6590c8c1af693efcf3b7931c76",
    "kind": "site",
    "status": "queued",
    "status_url": "/api/static/jobs/2bb82b6590c8c1af693efcf3b7931c76",
    "cancel_url": "/api/static/jobs/2bb82b6590c8c1af693efcf3b7931c76",
    "progress": {
      "stage": "等待执行",
      "processed": 0,
      "total": 0,
      "generated_files": 0
    },
    "created_at": "2026-09-18T02:46:33+08:00",
    "updated_at": "2026-09-18T02:46:33+08:00"
  }
}
```

同一时刻已有活动批任务时返回 `409 Conflict`，并在 `active_job` 中返回当前任务。

## 5. 查询和取消任务

```bash
curl -H "Authorization: Bearer $STATIC_TOKEN" \
  http://127.0.0.1:9142/api/static/jobs/<job-id>

curl -X DELETE \
  -H "Authorization: Bearer $STATIC_TOKEN" \
  http://127.0.0.1:9142/api/static/jobs/<job-id>
```

状态值：`queued`、`running`、`succeeded`、`failed`、`interrupted`。

取消活动任务返回 `202`；任务已处于终态时返回 `200`。取消是协作式的，客户端应继续轮询，直到状态变为 `interrupted` 或其他终态。

job 存储在进程内存中，终态保留 24 小时且最多保留 100 个。服务重启后旧 job ID 不可查询。

## 6. 单页面

```bash
curl -X POST \
  -H "Authorization: Bearer $STATIC_TOKEN" \
  'http://127.0.0.1:9142/api/static/page?name=首页&gray=2&path=/srv/portal-static/caam/site'
```

页面名由适配器规范化，允许站点定义中英文别名。CAAM 支持首页、协会概况、协会工作、统计数据、会员专区和党建专区及其英文别名；MIIC 支持资讯动态、核心业务、服务平台和关于我们及其英文别名。

## 7. 单栏目

栏目名称优先于栏目 ID：

```bash
curl -X POST \
  -H "Authorization: Bearer $STATIC_TOKEN" \
  --get \
  --data-urlencode 'column_name=首页通知公告' \
  --data-urlencode 'path=/srv/portal-static/caam/site' \
  http://127.0.0.1:9142/api/static/list
```

也可以使用正整数 `column_id`：

```text
POST /api/static/list?column_id=42
```

必须至少提供 `column_name` 或 `column_id`。

## 8. 单文章生成与删除

```bash
curl -X POST \
  -H "Authorization: Bearer $STATIC_TOKEN" \
  'http://127.0.0.1:9142/api/static/article?id=123&refresh=related&path=/srv/portal-static/caam/site'

curl -X DELETE \
  -H "Authorization: Bearer $STATIC_TOKEN" \
  'http://127.0.0.1:9142/api/static/article?id=123&refresh=related&path=/srv/portal-static/caam/site'
```

`refresh=related` 是默认值，用于同步刷新与文章关联的页面/栏目；`none` 只处理详情本身。删除仍处于可发布状态的文章通常返回 `409`，应先在 CMS 下线。

## 9. HTTP 状态码

| 状态码 | 含义 |
| --- | --- |
| `200` | 同步操作、查询或无需等待的取消成功 |
| `202` | 批任务已接收，或活动任务已收到取消信号 |
| `400` | 参数、页面名、灰度值或输出路径无效 |
| `401` | Token 缺失或不匹配 |
| `404` | 页面、栏目、文章或 job 不存在/不可发布 |
| `405` | HTTP 方法错误，响应包含 `Allow` |
| `409` | 批任务冲突、名称不唯一、文章仍处于发布状态等业务冲突 |
| `500` | 未分类的生成错误 |
| `503` | 适配器操作不可用；正常适配器会在启动阶段校验并避免此状态 |

## 10. dia-platform 对接约定

后台公开给前端的路由仍位于自身 API 前缀下，例如 `/business_portal/api/static/site`。后台负责：

- 用户鉴权、菜单权限、防重放和请求签名；
- 从设置中读取静态化地址、输出路径和令牌名；
- 从后台进程环境变量取得真实 Token；
- 调用本文件定义的 `/api/static/*`；
- 透传静态化状态码和响应，并记录操作日志。

因此管理前端不直接调用 9142/9143，也不保存静态化 Token。
