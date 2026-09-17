# portal-static

统一门户静态化服务。MIIC 与 CAAM 共用同一个二进制、HTTP API、任务队列、鉴权、超时、输出路径校验和媒体解析；每份配置只启动一个站点进程。站点的数据模型、仓储和生成规则分别位于 `internal/adapters/miic` 与 `internal/adapters/caam`。

原 `miic-portal`、`caam-portal` 工程只作为页面模板和静态资源脚手架，生成文件始终写入本工程的 `dist/`，不会回写原工程。

## 本地预览

预览使用内置演示数据，不连接数据库：

```bash
go run ./cmd/portal-static preview --config configs/miic.example.yaml
go run ./cmd/portal-static preview --config configs/caam.example.yaml
```

输出分别位于 `dist/miic-preview` 和 `dist/caam-preview`。两个站点都执行完整的演示站点生成流程：复制只读脚手架，并生成主页面、栏目列表和文章详情。

## 生产运行

复制对应示例配置，并设置数据库 DSN 与 API 令牌：

```bash
cp configs/miic.example.yaml configs/miic.yaml
export MIIC_DB_DSN='user:password@tcp(127.0.0.1:3306)/miic_portal?charset=utf8mb4&parseTime=true'
export MIIC_STATIC_TOKEN='replace-with-a-random-token'
go run ./cmd/portal-static serve --config configs/miic.yaml
```

```bash
cp configs/caam.example.yaml configs/caam.yaml
export CAAM_DB_DSN='user:password@tcp(127.0.0.1:3306)/caam_portal?charset=utf8mb4&parseTime=true'
export CAAM_STATIC_TOKEN='replace-with-a-random-token'
go run ./cmd/portal-static serve --config configs/caam.yaml
```

MIIC 默认监听 `127.0.0.1:9143`，CAAM 默认监听 `127.0.0.1:9142`。两个站点的 `generate` 都生成完整生产站点。

```bash
go run ./cmd/portal-static generate --config configs/miic.yaml
go run ./cmd/portal-static generate --config configs/caam.yaml
```

公开接口保持为：

- `POST /api/static/site|pages|lists|articles`
- `POST /api/static/page|list|article`
- `DELETE /api/static/article`
- `GET|DELETE /api/static/jobs/{id}`
- `GET /healthz`

除健康检查外，请求使用 `Authorization: Bearer <token>`。CAAM 页面参数同时接受中文名和兼容英文名，例如 `首页|home`、`协会概况|about`、`统计数据|stats`。

## 媒体路径

默认 `media.mode: same_origin`，受管媒体输出为站点根路径。CAAM 会把 `caamm/uploads` 和 `caam/uploads` 统一为 `/caam/uploads`，并将配置中的 `demo.miic.com.cn` 旧来源改写为当前站点路径。如需独立媒体域名，可切换为 `cdn` 并设置 `media.base_url`。

## 验证

```bash
go test ./...
go test -race ./...
go vet ./...
```

上线 CAAM 前，先通过 `preview` 或 API 的 `path` 参数生成到隔离目录，比较文件集合和关键 HTML，再切换 9142 端口上的旧静态化进程。

## 统一工作流约束

站点可以使用不同模板、栏目模型和数据仓储，但外部操作流程必须一致：`preview` 使用演示数据生成整站，`generate` 使用生产数据生成整站，`serve` 提供相同的 HTTP 路由、鉴权和任务管理。新增站点适配器不得在命令层引入站点专用语义。
