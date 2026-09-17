# 配置参考

## 1. 配置加载规则

配置格式为 YAML，当前只支持 `version: 1`。相对路径按配置文件所在目录解析，不按启动命令的当前目录解析。生产部署建议全部使用绝对路径，避免移动配置文件后路径含义变化。

当前运行时在启动时读取一次配置。修改 `driver`、`site`、`database`、`server`、`paths`、`media` 或 `adapter` 后都必须完成测试并重启对应站点进程；不要依赖运行中热加载。

## 2. 完整公共外壳

```yaml
version: 1
driver: example

site:
  id: example
  timezone: Asia/Shanghai

database:
  dsn_env: EXAMPLE_DB_DSN
  schema: example_portal
  max_open_conns: 10
  max_idle_conns: 5
  conn_max_lifetime: 5m

server:
  addr: 127.0.0.1:9150
  token_env: EXAMPLE_STATIC_TOKEN
  request_timeout: 10m
  batch_idle_timeout: 15m
  batch_max_duration: 6h

paths:
  source_root: /srv/portal-source/example
  dist_root: /srv/portal-static/example/site
  preview_root: /srv/portal-static/preview/example
  templates:
    home: templates/home.html.tmpl

media:
  mode: same_origin
  aliases:
    old/uploads: /example/uploads
  rewrite_origins:
    - https://old.example.com

adapter: {}
```

## 3. 顶层字段

| 字段 | 必填 | 含义 |
| --- | --- | --- |
| `version` | 是 | 配置协议版本，当前为 `1` |
| `driver` | 是 | 适配器注册名，如 `miic`、`caam` |
| `site` | 是 | 站点身份和时区 |
| `database` | 取决于适配器 | Portal CMS MySQL 适配器生产模式必填；preview 和非 MySQL 适配器可不使用 |
| `server` | 是 | HTTP 监听、Token 环境变量和超时 |
| `paths` | 是 | 只读源、输出、预览和模板路径 |
| `media` | 是 | 媒体路径规范化策略 |
| `adapter` | 是 | 站点专属页面、栏目和渲染配置 |

## 4. `site`

| 字段 | 说明 |
| --- | --- |
| `id` | 站点稳定标识，用于日志和运行身份 |
| `timezone` | IANA 时区名，如 `Asia/Shanghai`；用于发布时间和生成时间 |

## 5. `database`

| 字段 | 说明 |
| --- | --- |
| `dsn_env` | 保存 DSN 的环境变量名；YAML 中不写密码 |
| `schema` | 查询的权威 schema，仅允许字母、数字和下划线 |
| `max_open_conns` | 最大打开连接数，必须大于 0 |
| `max_idle_conns` | 最大空闲连接数，范围为 0 到 `max_open_conns` |
| `conn_max_lifetime` | 连接最长生命周期，如 `5m` |

示例 DSN：

```bash
EXAMPLE_DB_DSN='portal_reader:password@tcp(127.0.0.1:3306)/example_portal?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai'
```

安全规则：

- DSN 显式指定的数据库必须与 `schema` 一致；不一致会启动失败。
- 所有共享数据源查询都限定到配置 schema。
- 生产账号优先只授予 `SELECT`；静态化服务不负责迁移或修改 CMS 数据。
- 不要使用 SQL dump 中记录的旧主机或内网地址作为默认连接目标。

## 6. `server`

| 字段 | 说明 |
| --- | --- |
| `addr` | HTTP 监听地址；一站一端口，生产推荐回环或内网地址 |
| `token_env` | Bearer Token 的环境变量名 |
| `request_timeout` | 单页、单栏目、单文章同步操作超时 |
| `batch_idle_timeout` | 批任务多长时间没有进度即中断 |
| `batch_max_duration` | 单个批任务最长运行时间 |

`token_env` 必须有非空值，否则 `serve` 拒绝启动。Token 应由密码管理系统生成和注入，不写入配置、日志或 Git。

## 7. `paths`

| 字段 | 说明 |
| --- | --- |
| `source_root` | 原门户模板/资源脚手架，只读 |
| `dist_root` | `generate` 和默认生产操作输出目录 |
| `preview_root` | `preview` 输出目录 |
| `templates` | 适配器需要的模板名到路径映射；相对路径以 `source_root` 为基准 |
| `assets` | 可选的附加资源路径 |

约束：

- `source_root` 与任何输出目录不能相同或重叠。
- HTTP 请求中的 `path` 必须是绝对非根目录。
- 不允许把 `miic-portal`、`caam-portal` 或其他源工程作为输出目录。
- 运行账号需要读取源目录、写入输出目录和创建同级临时目录的权限。

## 8. `media`

`mode: same_origin` 是默认推荐方式：数据库中的完整旧 URL 会去掉匹配的 origin，最终输出当前站点路径。

`aliases` 左侧是历史路径，右侧是规范路径。例如：

```yaml
aliases:
  caamm/uploads: /caam/uploads
  caam/uploads: /caam/uploads
```

`rewrite_origins` 列出需要移除的旧 origin。上线验收仍应扫描产物，确认没有遗漏旧域名、内部 IP 或错误上传目录。

## 9. `adapter`

`adapter` 完全由门户适配器解释。它通常包含：

- 页面输出文件名；
- 页面对应的 CMS 页面名和栏目名；
- 栏目展示标题、数量限制和允许文章类型；
- 分页大小、锁过期时间和占位图；
- 站点专属容错策略。

不要把适配器字段上移到公共配置，除非所有门户都需要相同语义和相同生命周期。

## 10. 当前示例

- CAAM：[configs/caam.example.yaml](../configs/caam.example.yaml)
- MIIC：[configs/miic.example.yaml](../configs/miic.example.yaml)

示例用于说明结构，不应直接携带生产密码、生产 Token 或不可移植的临时路径。
