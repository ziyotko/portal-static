# portal-static

`portal-static` 是所有门户项目共用的静态化生成平台，不是 MIIC 与 CAAM 的合并工程。MIIC、CAAM 只是首批适配器；后续门户继续接入同一个二进制、命令、HTTP API 和任务模型。

## 文档导航

- [完整文档目录](docs/README.md)
- [本机傻瓜版操作手册](docs/local-quickstart.md)
- [部署指南](docs/deployment.md)
- [管理后台使用指南](docs/admin-workflow.md)
- [总体设计](docs/architecture.md)
- [配置参考](docs/configuration.md)
- [HTTP API 契约](docs/api-contract.md)
- [新门户接入指南](docs/adapter-development.md)
- [运维与排障手册](docs/operations.md)

README 保留平台定位和快速入口；部署、使用、设计和接入细节以上述专题文档为准。

页面模板、栏目结构、数据来源和生成规则可以不同，外部操作流程必须一致：

- 一个程序：所有站点使用 `portal-static` 二进制。
- 一套命令：统一使用 `preview`、`generate`、`serve`。
- 一套接口：统一鉴权、任务队列、取消、超时、灰度、输出路径和错误响应。
- 一站一配置、一站一进程：不同门户使用独立配置、端口和输出目录。
- 完整预览：每个适配器的 `preview` 都必须生成主页面、栏目列表、文章详情和所需静态资源，不能只生成首页。
- 原工程只读：门户源工程只提供模板和资源；产物写入本工程的 `dist/` 或请求指定的隔离目录。

## 三层架构

### 公共运行内核

`internal/platform` 和 `internal/core` 提供适配器注册、统一运行时、配置、HTTP API、任务管理、鉴权、取消、超时、输出路径校验、进度和媒体重写。

命令层只读取 `driver` 并从注册表构建运行时，不包含任何站点分支。运行时启动前会校验完整的 `httpapi.Operations`；缺少任一标准生成操作都会立即失败。

### 可复用数据源

`internal/sources/portalcms` 是 `dia-platform/business/portal` 门户数据模型对应的共享 MySQL 数据源，负责：

- DSN、连接池、时区和连接生命周期；
- 安全校验并实际应用 `database.schema`；
- 阻止 DSN 数据库名与配置 schema 不一致；
- 为页面、栏目、文章和发布关系查询提供统一的 schema 限定执行环境。

数据库不是平台强制依赖。`preview` 不连接数据库，未来使用 HTTP、文件或其他数据库的适配器可以完全忽略 `database`；生产工厂自行选择数据源并负责关闭资源。

### 门户适配器

`internal/adapters/<driver>` 只保存该门户特有的配置映射、demo、模型、查询投影、模板渲染、页面别名和错误分类。当前注册入口位于 `internal/adapters/builtin`。

新增门户时只需要：

1. 实现 `platform.AdapterFactory` 的 `Driver` 和 `Build`。
2. 分别构建完整的 preview 与 production 运行时。
3. 返回通过校验的完整 `httpapi.Operations`。
4. 在 builtin 注册清单中注册工厂，不修改 CLI 或公共 HTTP 层。
5. 提供示例配置、自包含 demo 和共享契约测试。

## 标准工作流

### 演示预览

```bash
go run ./cmd/portal-static preview --config configs/<site>.example.yaml
```

预览使用适配器内置 demo，不需要 DSN，只能写入 `paths.preview_root`。

### 生产整站生成

```bash
export <SITE>_DB_DSN='user:password@tcp(127.0.0.1:3306)/site_portal?charset=utf8mb4&parseTime=true'
go run ./cmd/portal-static generate --config configs/<site>.yaml
```

使用共享 Portal CMS MySQL 数据源时，DSN 中的数据库名必须与 `database.schema` 相同；也可以省略 DSN 数据库名，由 schema 限定全部查询。

### 独立静态化服务

```bash
export <SITE>_STATIC_TOKEN='replace-with-a-random-token'
go run ./cmd/portal-static serve --config configs/<site>.yaml
```

当前站点：

- `driver: miic`：示例配置 `configs/miic.example.yaml`，默认监听 `127.0.0.1:9143`。
- `driver: caam`：示例配置 `configs/caam.example.yaml`，默认监听 `127.0.0.1:9142`。

## 公共 HTTP 契约

除 `GET /healthz` 外，请求都必须携带 `Authorization: Bearer <token>`。

```text
POST   /api/static/site
POST   /api/static/pages
POST   /api/static/lists
POST   /api/static/articles
POST   /api/static/page
POST   /api/static/list
POST   /api/static/article
DELETE /api/static/article
GET    /api/static/jobs/{id}
DELETE /api/static/jobs/{id}
```

批处理统一返回 `202` 和包含 `id`、`kind`、`status`、`status_url`、`cancel_url`、时间及进度的 job；同步操作统一返回 `200` 和生成计数。适配器可以定义自己的中英文页面别名和业务错误映射，但不得增加站点私有路由或特殊命令流程。

`dia-platform/business/portal` 当前使用的 `site`、`pages`、`lists`、`articles`、`page`、`list`、`article` 和 `jobs` 调用可以直接使用该契约。后台工程不需要修改，也不是本工程的编译依赖。

## 配置职责

所有站点共享以下配置外壳：

- `driver`、`site`：适配器和站点身份。
- `server`：监听地址、令牌环境变量和任务超时。
- `paths`：只读源脚手架、生产输出、预览输出和模板。
- `media`：同源/CDN 模式、历史目录别名和旧 origin 重写。
- `database`：可选的 Portal CMS MySQL 连接配置。
- `adapter`：站点专属页面、栏目和生成规则。

公共配置只校验平台字段；数据源和适配器分别校验自己的配置。配置中的 `source_root` 与任何输出目录都不得重叠，原门户工程不能作为输出目标。

## CAAM 真实数据库验收

验收命令只读取指定 SQL 文件，只提取 `caam_portal` 段，并在临时 MySQL 8 容器中运行。它不会导入其他数据库、连接 dump 中的内网地址或把 SQL 加入 Git。

先启动 Docker Desktop，然后在仓库根目录执行：

```bash
export CAAM_SQL_DUMP='/absolute/path/to/Dump.sql'
go run ./cmd/caam-db-verify
```

如需查看生成过程日志：

```bash
CAAM_VERIFY_VERBOSE=1 go run ./cmd/caam-db-verify
```

默认生成目录随验收临时目录一起清理。如需保留页面供人工检查，指定一个尚不存在的目录：

```bash
CAAM_VERIFY_OUTPUT="$PWD/dist/caam-db-preview" go run ./cmd/caam-db-verify
```

工具会验证真实页面、栏目、发布关系、草稿排除、首页与六类页面、栏目列表、详情数量和媒体重写，并扫描以下禁止内容：

- dump 中的内网 IP；
- Windows 部署路径；
- `demo.miic.com.cn` 等旧 origin；
- `caamm/uploads` 错误路径。

容器、凭据和提取 SQL 在成功或失败后都会删除；未设置 `CAAM_VERIFY_OUTPUT` 时生成目录也会删除。Docker 未启动时只返回明确错误，常规测试不依赖 Docker。

## 测试与上线

```bash
go test ./...
go test -race ./...
go vet ./...
```

`internal/platform/contracttest` 是所有适配器共用的 HTTP 契约测试，覆盖鉴权、批任务、取消、超时、灰度、输出路径、页面别名和错误分类。新增适配器必须加入该矩阵。

上线前先生成到隔离目录，对比文件集合和关键 HTML，确认产物不存在旧域名、内网 IP 或错误上传路径，再切换该站点的独立进程。不得向 `miic-portal`、`caam-portal` 或 `dia-platform` 写入代码和生成产物。
