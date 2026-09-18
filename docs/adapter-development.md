# 新门户接入指南

本指南用于接入第三个及后续门户。目标是新增适配器，而不是复制一套 CLI、HTTP 服务或任务系统。

## 1. 接入前确认

先收集以下信息：

- 门户稳定 `driver` 和 `site.id`；
- 模板、资源和原门户源码位置；
- 首页、业务页、栏目页、详情页的输出路径规则；
- 数据来源和已发布内容判定规则；
- 页面、栏目、文章的唯一标识；
- 发布、下架、删除和关联刷新的业务语义；
- 历史媒体域名、上传目录别名和目标地址策略；
- 允许的中英文页面名称；
- 预览 demo 所需的最小完整内容。
- 面向运维人员的“静态化栏目取数对照表”所需信息：后台页面名、栏目名、层级、内容类型、取数数量、排序和必填字段。

如果业务方不能明确“什么数据算已发布”，不要开始生成器开发。静态化不能自行推断或绕过 CMS 审核规则。

## 2. 目录建议

```text
internal/adapters/example/
├── factory.go
├── runtime.go
├── config.go
├── model/
├── repository/        # 仅站点专属查询；公共 Portal CMS 能力放 sources
├── generator/
├── demo/
├── testdata/
├── http_contract_test.go
└── preview_test.go

configs/example.example.yaml
```

不要复制 MIIC 或 CAAM 数据层后只改包名。先判断数据模型是否真的相同；公共能力应提取到 `internal/sources`，不同模型保留在适配器内。

## 3. 实现工厂

工厂必须实现：

```go
type AdapterFactory interface {
    Driver() string
    Build(context.Context, Mode, config.Snapshot, *slog.Logger) (*Runtime, error)
}
```

约束：

- `Driver()` 返回非空、稳定、唯一的配置名称。
- `Build(Preview, ...)` 使用自包含 demo，不连接生产数据源。
- `Build(Production, ...)` 自行创建数据源，并通过 runtime close 函数关闭资源。
- 初始化中途失败时立即清理已创建的连接或临时资源。
- 返回的 `Operations` 必须通过完整性校验。

## 4. 实现统一 Operations

每个适配器必须提供：

```text
GenerateSite
GeneratePages
GenerateAllLists
GenerateAllArticles
GeneratePage
GenerateList
GenerateListByName
GenerateArticle
DeleteArticle
GenerateArticleRelated
DeleteArticleRelated
NormalizePageName
ValidateOutputPath
ClassifyError
PageNameError
```

即使门户当前不使用某个后台按钮，也要定义一致、可测试的业务行为；不能在 HTTP 层增加站点判断，也不能运行后才返回“该门户不支持”。

## 5. Preview 要求

preview 不是首页截图，而是完整运行演练。它必须：

- 使用生产同款模板和生成器；
- 生成适配器定义的全部主页面；
- 至少生成一个真实结构的栏目列表与分页；
- 至少生成一个文章详情；
- 包含页面依赖的 CSS、JavaScript、图片等资源；
- 产生可独立浏览和检查的目录；
- 不依赖数据库、内网服务或原工程运行进程。

demo 数据和模板测试脚手架放在适配器 `testdata`，不要保留临时 seed 模块或复制生产 SQL。

## 6. Production 数据源

### 使用 Portal CMS MySQL

优先复用 `internal/sources/portalcms` 的连接、schema 和公共查询环境。适配器只实现站点所需投影和规则。

必须满足：

- `database.schema` 是查询的权威 schema；
- DSN/schema 冲突时启动失败；
- SQL 标识符经过安全校验；
- 查询接受 context，并受请求或任务超时控制；
- 发布关系、草稿和未审核排除有测试覆盖。

### 使用其他来源

适配器可以直接选择其他数据库、HTTP API 或文件，不应为了通过公共校验伪造 MySQL 配置。连接和关闭生命周期仍由工厂负责。

## 7. 输出与原子发布

- 源模板目录只读，输出目录必须独立。
- 完整站点先在目标同级临时目录生成并校验，再替换目标。
- 单栏目和单文章使用锁与临时文件/目录，避免半写入文件。
- 输出路径校验必须拒绝相对路径、根目录和源目录重叠。
- `path` 请求覆盖必须贯穿所有嵌套生成器；内部 staging 生成时要屏蔽原请求 path，禁止写回正式目录。
- 完整站点发布前至少校验主页面非空、内部链接落盘和关键资源存在。

## 8. 页面名称与错误

适配器负责把中文业务名称和英文稳定别名规范化为内部页面键。例如：

```text
首页 / home -> home
协会概况 / about -> about
```

错误分类至少覆盖：

- 忙/锁冲突 → `409`；
- 页面、栏目、文章不存在或文章未发布 → `404`；
- 页面/栏目不唯一 → `409`；
- 输出路径或页面名无效 → `400`；
- 未知内部错误 → `500`，对外不泄露数据库密码或内部路径细节。

## 9. 媒体迁移

在示例配置中列出：

- 历史上传路径别名；
- 需要剥离的旧 origin；
- 当前同源或 CDN 输出策略。

测试正文 HTML、封面、视频 poster、附件链接、查询参数和 fragment。上线验收扫描旧域名、内网 IP、Windows 路径和拼写错误的上传目录。

## 10. 注册

在 `internal/adapters/builtin` 的注册清单中加入工厂。除此之外，不修改：

- `cmd/portal-static` 的命令分支；
- `internal/core/httpapi` 的路由表；
- 管理后台的静态化操作流程。

新增 driver 后，未知/重复 driver、preview/production 选择和初始化清理仍由平台注册表测试保障。

## 11. 必需测试

- 配置解析、路径解析和字段校验；
- preview 完整站点测试；
- production repository/数据源测试；
- 所有页面、栏目、文章生成和删除；
- 请求输出路径和原子发布；
- 发布关系、草稿、未审核和下线排除；
- 媒体重写和内部链接检查；
- 页面中英文别名和错误分类；
- 共享 HTTP 契约测试；
- 初始化失败资源关闭。

提交前执行：

```bash
go test ./...
go test -race ./...
go vet ./...
```

若有可用脱敏 SQL dump，再增加显式 Docker 集成验收；常规单元测试不能依赖 Docker 或生产网络。

## 12. 栏目取数对照表

每个适配器必须提供独立的运维对照表：

```text
docs/column-mapping/<driver>.md
```

可从 [新站点静态化栏目取数对照表模板](column-mapping-template.md) 开始编写。对照表必须：

- 使用管理后台中能看到的页面名称、栏目名称和内容类型；
- 逐项对应“页面区域 → 后台页面/栏目 → 类型 → 数量 → 排序 → 必填内容 → 输出位置”；
- 对动态栏目明确页面归属、父子层级、排除规则和 `code` 要求；
- 对统计数据、附件、外链、封面和视频等特殊格式提供可复制示例；
- 说明审核、发布和栏目关联条件；
- 说明录入后应该生成文章、栏目、主页面还是全站；
- 与当前示例配置和生成器实现保持同步。

没有栏目取数对照表，视为适配器未完成，不能交付运维上线。

## 13. 接入完成定义

- 同一二进制可以通过新配置运行新门户；
- CLI 和 HTTP 层没有新增站点分支；
- preview 生成完整站点；
- 管理后台无需新增私有接口即可操作；
- 已提供并核对 `docs/column-mapping/<driver>.md`；
- 生产模板源工程保持只读；
- 全量测试和真实数据验收通过；
- 部署、配置、运维和回滚信息已补充到文档。
