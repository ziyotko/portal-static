# portal-static

面向所有静态化门户项目的统一生成平台。每个门户可以拥有不同的页面模板、栏目模型、数据库结构和生成规则，但必须通过同一个程序、配置格式、命令流程、HTTP API 和任务模型运行。

MIIC 与 CAAM 是当前首批接入的站点适配器，不是平台的范围边界。后续所有门户静态化项目都应接入本工程，不再为单个门户复制一套独立的静态化服务。

## 平台原则

- 一个程序：所有门户使用 `portal-static` 二进制。
- 一套流程：统一使用 `preview`、`generate`、`serve`。
- 一套接口：共享鉴权、任务队列、取消、超时、进度、输出路径校验和错误响应。
- 一站一配置、一站一进程：不同门户使用不同配置、端口和输出目录，不在同一进程中混合租户。
- 适配器隔离：门户专属的 model、repository、generator、demo 和页面名映射位于 `internal/adapters/<driver>`。
- 原工程只读：现有门户工程只作为模板和静态资源脚手架，所有生成产物写入本工程的 `dist/` 或请求指定的隔离目录。
- 模板可以不同，外部操作语义不得不同；站点专属逻辑不能进入公共 HTTP 和命令层。

## 标准工作流

### 1. 演示预览

`preview` 使用适配器内置演示数据生成可独立浏览的完整站点，不连接生产数据库：

```bash
go run ./cmd/portal-static preview --config configs/<site>.example.yaml
```

预览必须包含脚手架资源、主页面、栏目列表和文章详情，且只能写入配置中的 `paths.preview_root`。

### 2. 生产生成

`generate` 连接站点数据库并生成完整生产站点：

```bash
export <SITE>_DB_DSN='user:password@tcp(127.0.0.1:3306)/database?charset=utf8mb4&parseTime=true'
go run ./cmd/portal-static generate --config configs/<site>.yaml
```

### 3. HTTP 服务

`serve` 启动该站点的独立静态化进程：

```bash
export <SITE>_STATIC_TOKEN='replace-with-a-random-token'
go run ./cmd/portal-static serve --config configs/<site>.yaml
```

除健康检查外，请求使用 `Authorization: Bearer <token>`。所有适配器共享以下公开接口：

- `POST /api/static/site|pages|lists|articles`
- `POST /api/static/page|list|article`
- `DELETE /api/static/article`
- `GET|DELETE /api/static/jobs/{id}`
- `GET /healthz`

站点可以定义自己的中英文页面别名和错误分类，但不得增加只对单个站点可见的命令流程或私有 HTTP 路由。

## 适配器结构

```text
internal/adapters/<driver>/
├── config/       # 站点配置映射与校验
├── demo/         # 可生成完整预览站点的演示数据源
├── model/        # 站点专属数据结构
├── repository/   # 生产数据访问
├── generator/    # 模板渲染与产物规则
├── testdata/     # 自包含模板和测试数据
└── runtime.go    # 接入统一 Operations
```

当前已有：

- `driver: miic`，示例配置 `configs/miic.example.yaml`，默认端口 9143。
- `driver: caam`，示例配置 `configs/caam.example.yaml`，默认端口 9142。

## 新门户接入要求

1. 在 `internal/adapters/<driver>` 实现独立适配器，不修改其他门户适配器。
2. 提供完整示例配置和无需数据库的完整 demo 数据源。
3. 将页面别名、业务错误和结果计数映射为统一 `httpapi.Operations`。
4. 保证 `preview`、`generate`、`serve` 与现有站点语义一致。
5. 添加 HTTP 契约、输出路径、媒体重写、完整预览和原产物兼容测试。
6. 确认源门户工作树未被修改，产物中没有旧域名、内网 IP 或错误上传路径。

## 通用媒体处理

默认 `media.mode: same_origin`，受管媒体输出为当前站点根路径；如需独立媒体域名，可使用 `cdn` 并设置 `media.base_url`。历史目录别名和旧来源域名通过每个站点配置中的 `media.aliases`、`media.rewrite_origins` 声明，不在公共代码中硬编码。

## 验证与上线

```bash
go test ./...
go test -race ./...
go vet ./...
```

上线任何门户前，先生成到隔离目录，对比旧、新文件集合和关键 HTML；确认允许差异后，再切换该站点的独立进程。不得直接向原门户工程写入或发布产物。

## 下一阶段

将当前命令层中的站点 `switch` 替换为统一适配器注册表，并提供新适配器脚手架与自动契约测试，使新增门户只需注册适配器和配置，不再修改命令层或公共 HTTP 层。
