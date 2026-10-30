# MIIC 门户生产部署与运维交接

> 更新：2026-10-30。本文描述当前代码的部署方法，不表示 MIIC 已在生产环境发布。文中的域名、目录和账号均为示例，实施时以目标环境为准。本机 `miic_portal` 测试库、`dist/miic-live` 和本地令牌不能直接作为生产配置。

MIIC 使用 `portal-static` 的 `miic` 适配器。页面资源已纳入本仓库的 `internal/adapters/miic/site`，运行和构建都不依赖单独部署 `miic-portal` 工程。通用参数见[部署指南](deployment.md)和[配置参考](configuration.md)；内容录入位置见[MIIC 栏目取数对照表](column-mapping/miic.md)。Dia 后台自身的数据库迁移、前后端部署和安全配置按 `dia-platform/business/portal/DEPLOY.md` 执行。

## 1. 系统边界与上线前确认

| 组件 | 职责 |
| --- | --- |
| Dia Portal 后台与 API | 连接 MIIC 自己的 `miic_portal` 库，维护模板、栏目、文章、上传文件和静态化设置 |
| `portal-static` 的 MIIC 进程 | 只读 `miic_portal`，从本版本资源和数据库内容生成页面；默认监听 `127.0.0.1:9143` |
| 对外 Web 服务 | 托管生成目录，并把后台页面、API 和公开上传映射到同一站点域名 |

每站使用独立的 Portal 后台实例、数据库、Redis DB、静态化令牌、端口和输出目录。`portal-static` 不迁移数据库；数据库结构与版本由 Dia 发布流程负责。静态化服务建议使用只有 `SELECT` 权限的数据库账号。生产模板数据、栏目关系、公开上传和当前静态产物都应纳入备份。上线前固定 `portal-static` 与 Dia Portal 的代码版本和数据库结构，准备上一版本的二进制、资源、配置及产物回滚点。

MIIC 当前生产模式要求 `template` 表中六种模板均已启用、类型正确、`source_code` 非空且能唯一匹配：

| 角色 | 默认按名称匹配 | `template.type` |
| --- | --- | --- |
| `news` | 资讯动态 | `home` |
| `business` | 核心业务 | `home` |
| `platforms` | 服务平台 | `home` |
| `about` | 关于我们 | `home` |
| `list` | 栏目 | `column` |
| `article` | 详情 | `detail` |

如果正式库已确定稳定且唯一的 `template.code`，可在 `paths.template_codes` 中按上述角色显式绑定；否则使用表中的唯一名称。`template.route_path` 和栏目的 `route_path` 还会生成与 Dia 后台访问地址兼容的路径。启用栏目须绑定相应模板，文章须完成审核并发布到有效栏目。上线前按[数据库模板管理与运维交接](template-management.md)核对模板和路由，不要把本地测试模板直接覆盖正式库。

## 2. 构建与目录

在受控构建环境固定提交号，执行项目测试和构建；将同一提交的二进制、MIIC 页面资源和预览模板一起发布：

```bash
go test ./...
go vet ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o build/portal-static ./cmd/portal-static
```

目标主机可按下列布局准备目录，`<version>` 替换为本次版本号：

```text
/opt/portal-static/releases/<version>/
├── portal-static
└── share/
    ├── miic-site/                 # internal/adapters/miic/site 的内容直接放在此处
    │   ├── assets/
    │   ├── about.html
    │   ├── script.js
    │   └── styles.css
    └── miic-preview-templates/    # internal/adapters/miic/testdata/templates/*.tmpl
/opt/portal-static/current -> releases/<version>
/etc/portal-static/miic.yaml
/etc/portal-static/miic.env
/srv/portal-static/miic/site/       # 正式静态产物
/srv/portal-static/miic/preview/    # 演示数据预览；不对外发布
```

`source_root` 要指向直接包含 `about.html`、`styles.css` 和 `assets/` 的只读目录，不能再多套一层 `site`。`source_root` 与任一输出目录不能重叠。服务账号须能读取版本资源和配置，并能写入输出目录及其父目录，因为整站生成会在同级建立临时目录后替换正式目录。预览模板文件用于 `preview`；生产页面模板以数据库 `template.source_code` 为准，但 YAML 的模板路径仍须配置为存在的文件。

生产模式下，`news`、`business`、`platforms`、`about`、`list`、`article` 都读取本次操作的数据库模板快照。改动 `about.html` 等页面结构时，应把相同版本的内容发布到对应 `template.source_code` 并重建页面；仅替换 `miic-site` 中的 HTML 不会更新生产主页面。`styles.css`、`script.js`、图片等仍由 `miic-site` 脚手架复制。

## 3. MIIC 专属配置与启动

复制[MIIC 示例 YAML](../configs/miic.example.yaml)为 `/etc/portal-static/miic.yaml`，保留其中 `driver`、`site`、`database`、`server`、`media` 和 `adapter` 的 MIIC 取值，并把所有 `paths` 改成目标主机绝对路径：

```yaml
paths:
  source_root: /opt/portal-static/current/share/miic-site
  dist_root: /srv/portal-static/miic/site
  allowed_output_root: /srv/portal-static/miic
  preview_root: /srv/portal-static/miic/preview
  templates:
    news: /opt/portal-static/current/share/miic-preview-templates/news.html.tmpl
    business: /opt/portal-static/current/share/miic-site/business.html
    platforms: /opt/portal-static/current/share/miic-site/platforms.html
    about: /opt/portal-static/current/share/miic-site/about.html
    list: /opt/portal-static/current/share/miic-preview-templates/list.html.tmpl
    article: /opt/portal-static/current/share/miic-preview-templates/article.html.tmpl
```

| 配置 | 核对口径 |
| --- | --- |
| `driver` / `site.id` | `miic` / `miic` |
| `database.dsn_env` / `database.schema` | `MIIC_DB_DSN` / `miic_portal`，DSN 中的数据库名与 schema 一致 |
| `server.addr` / `server.token_env` | 同机示例为 `127.0.0.1:9143` / `MIIC_STATIC_TOKEN`；跨主机时改为后台可达的内网地址 |
| `paths.dist_root` / `allowed_output_root` | Dia 指定的输出路径必须位于安全根内；正式路径与其他站点隔离 |
| `media` | 按正式上传路径调整历史别名和旧域名重写；生成后逐一验证图片、附件和视频 |
| `adapter.news` / `adapter.about` | 栏目名须与正式库一致，尤其是中心重要动态、中心介绍、招聘信息和信息公开 |

在受保护的 `/etc/portal-static/miic.env` 注入 `MIIC_DB_DSN` 和 `MIIC_STATIC_TOKEN`，不要把密码或令牌值写入 YAML、Git 或验收截图。DSN 示例仅说明格式：

```text
MIIC_DB_DSN=portal_reader:<密码>@tcp(<数据库地址>:3306)/miic_portal?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai
MIIC_STATIC_TOKEN=<随机长令牌>
```

环境文件建议由 `root:portal-static` 持有，权限 `640`。Dia Portal 后端进程也须注入同名、同值的 `MIIC_STATIC_TOKEN`；后台连接的数据库必须是 `miic_portal`。修改 YAML 或服务进程环境变量后，重启对应进程。

为 MIIC 单独创建 `portal-static-miic.service`，以[通用 systemd 示例](deployment.md#7-systemd-服务)为基础设置 `EnvironmentFile=/etc/portal-static/miic.env`、`ExecStart=/opt/portal-static/current/portal-static serve --config /etc/portal-static/miic.yaml`、`ReadWritePaths=/srv/portal-static/miic`，并允许读取版本资源。启动后检查：

```bash
systemctl daemon-reload
systemctl enable --now portal-static-miic
systemctl status portal-static-miic
curl --fail http://127.0.0.1:9143/healthz
journalctl -u portal-static-miic -n 100 --no-pager
```

`/healthz` 只说明进程可响应；数据库、模板和输出权限仍须用真实生成任务验证。

## 4. 首次生成与 Dia 后台配置

先把正式 YAML 复制为隔离验收配置，将 `paths.dist_root` 改到 `/srv/portal-static/miic/staging-<version>`；它应位于 `allowed_output_root` 内，并与正式目录不同。通过受保护环境文件注入变量后，用真实数据库生成，核对结果，再启动正式服务：

```bash
set -a
. /etc/portal-static/miic.env
set +a
/opt/portal-static/current/portal-static generate --config /etc/portal-static/miic-staging.yaml
```

`preview` 使用内置演示数据，只能检查资源和结构，不能代替正式库验收。当前 MIIC 的整站生成会复制脚手架、生成文章详情和四个主页面、写入 `generated-content.js`；**栏目列表需另执行后台的“生成栏目页”批量操作**。全站任务后检查 `list/<栏目ID>/1.html`，若不存在，通过该操作或调用 `POST /api/static/lists` 补齐并确认任务成功。后续再次整站生成会替换目标目录，仍需重新执行栏目页批量生成。

Dia Portal 后台“基础配置 → 静态化设置”填入：

| 字段 | MIIC 示例 | 要点 |
| --- | --- | --- |
| 静态化输出路径 | `/srv/portal-static/miic/site` | 静态化进程可写的绝对路径，位于 `allowed_output_root` 内 |
| 静态化程序访问地址 | `http://127.0.0.1:9143` | 从后台进程所在主机访问；不同主机不能使用此回环地址 |
| 静态化程序访问令牌名 | `MIIC_STATIC_TOKEN` | 填环境变量名称，不填令牌值 |

保存后在“静态化管理”确认服务在线，执行“生成全站”，等待任务显示成功，再执行“生成栏目页”（批量生成全部栏目）。批任务返回 `202` 表示已排队，仍须查询任务直到成功；接口与状态说明见[HTTP API 契约](api-contract.md)。如果需要直接调用静态化服务，可由受控运维终端执行：

```bash
curl --fail -X POST -H "Authorization: Bearer ${MIIC_STATIC_TOKEN}" \
  http://127.0.0.1:9143/api/static/lists
```

Dia 的静态化输出路径优先于 YAML 的默认 `dist_root`，所以任务的 `result.output` 必须与对外 Web 服务读取的目录一致。后台内容发布触发的局部生成属于尽力而为；发布成功后仍应核对静态化任务结果。

## 5. 对外路由与验收

对外 Web 服务应按以下路径配置同源访问，先于静态文件通配规则处理后台与上传路径：

| 路径 | 目标 |
| --- | --- |
| `/`、`/news.html`、`/business.html`、`/platforms.html`、`/about.html`、`/article/`、`/list/`、`/assets/`、`/styles.css`、`/script.js`、`/generated-content.js` | MIIC 静态产物目录 |
| `/business_portal/` | Dia Portal 管理前端 |
| `/business_portal/api/` | Dia Portal 后端 API |
| `/business_portal/uploads/` | Dia Portal 公开上传目录或对应后端 |
| `/miic/uploads/` | 如果生成结果中存在此历史别名路径，映射到实际存储位置 |

上传路径以目标环境和生成 HTML 的实测结果为准。不要把 Portal 的 `private_uploads` 目录作为公开静态路径。浏览器验收应走 HTTP(S)，不要用 `file://` 打开页面，因为同源绝对路径无法正确解析。部署 Dia 时还要按其说明核对 `server.api_prefix`、`upload_dir_prefix`、可信代理和 Redis DB 配置。

上线前至少记录以下结果：

1. 真实库模板、栏目、文章状态与[栏目对照表](column-mapping/miic.md)一致；资讯分类、核心业务二级栏目和关于我们三个栏目均能取到预期内容。
2. `news.html`、`business.html`、`platforms.html`、`about.html`、`generated-content.js`、文章详情和所需栏目列表存在且非空；草稿、未审核及下线文章没有公开详情。
3. `/about.html` 的中心介绍取栏目首篇，信息公开与招聘信息分别显示对应栏目；新闻详情“相关资讯”属于当前栏目。
4. 图片、附件、视频、后台登录和 API 均可从对外域名访问；抽查桌面与手机布局、浏览器控制台及服务日志。
5. 扫描生成目录，确认没有测试域名、内网 IP、Windows 本地路径和错误上传路径；内部链接没有 404。验收记录保留版本号、任务 ID、输出路径和文件数量，不含口令或令牌。

## 6. 日常运行与回滚

内容发布、下线和模板修改后核对后台静态化日志及页面实际更新时间。生产数据库模板 `source_code` 修改后不必重启 MIIC 静态化进程，但必须重新生成受影响页面；MIIC 页面脚手架、CSS、JS 或二进制更新则须发布同版本资源并重新生成全站及全部栏目列表。页面 HTML 变更还须同步对应数据库模板。若任务失败，先处理数据库、模板、路径或上传配置问题，再重新生成；不要只凭 `/healthz` 判定站点已恢复。

升级前备份 `miic_portal`、公开上传、模板数据和当前静态产物。回滚时恢复上一版二进制、页面资源及 YAML，并按需要恢复对应数据库模板和静态产物；如 Dia 同步升级了数据库结构，按 Dia 的迁移回滚方案处理。恢复后检查 `/healthz`、主页面、栏目列表、详情和媒体链接。日常故障定位见[运维与排障手册](operations.md)。
