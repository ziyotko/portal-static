# MIIC 门户静态化服务

运维人员录入页面、栏目、文章和发布关系前，请先阅读 [DATA_ENTRY_GUIDE.md](DATA_ENTRY_GUIDE.md)。该文档说明当前程序实际认可的状态值、栏目层级、业务代码、排序规则和 SQL 自检方法。

该服务从 MIIC 内容数据库读取已发布、已审核且未删除的内容并生成完整部署目录。资讯栏目、资讯列表与详情、四个主页面共用的“中心重要动态”、核心业务服务详情、招聘信息和信息公开由数据库驱动；核心业务首页主体、服务平台主体与详情、关于我们介绍和全站公共信息保持固定。

## 配置

复制 `config.example.yaml` 为 `config.yaml`，按生产内容后台调整页面名、栏目名和输出路径。数据库和接口令牌只通过环境变量传入：

```powershell
$env:MIIC_DB_DSN='user:password@tcp(127.0.0.1:3306)/miic_portal?charset=utf8mb4&parseTime=true'
$env:MIIC_STATIC_TOKEN='replace-with-a-long-random-token'
```

数据库沿用 CAAM 的 `page`、`column`、`article`、`article_column_publish`、`article_attachment` 表；SQL 使用 DSN 选中的数据库，不写死 schema 名称。栏目列表按 `column.sort, column.id` 排序，文章按栏目发布关系的 `article_column_publish.is_top`、发布时间和文章 ID 排序。

## 本地数据库

本地开发库固定为 `127.0.0.1:3306/miic_portal`。如果没有从 CAAM 导入表结构，可依次执行 `db/schema.sql` 和 `db/seed.sql`；如果已经导入 CAAM 表结构，只执行 `db/seed.sql` 写入最小联调数据。

核心业务按“一级业务栏目 → 服务子栏目”组织，子栏目的 `column.code` 必须与现有查询参数一致，例如 `software` 对应 `service-detail.html?id=software`。子栏目内置顶或最新的一篇文章作为当前服务详情。

## 运行

```powershell
# 不连接数据库，生成演示站点到 dist/miic-preview
go run ./cmd/miic-static preview --config config.example.yaml

# 连接数据库，一次性生成完整站点到 dist/miic-portal
go run ./cmd/miic-static generate --config config.yaml

# 启动默认监听 127.0.0.1:9143 的静态化服务
go run ./cmd/miic-static serve --config config.yaml
```

完整调用方式见 [STATIC_API.md](STATIC_API.md)。

## 输出约定

- 主页面保持 `news.html`、`business.html`、`platforms.html`、`about.html`。
- 文章详情为 `article/YYYY/MM/{id}.html`。
- 通用栏目分页接口仍可输出 `list/{column_id}/{page}.html`；资讯主页本身按后台直接子栏目筛选，不依赖独立列表页。
- `detail.html?id={数字文章ID}` 通过 `generated-content.js` 跳转到对应年月详情。
- 上传媒体不会被下载；相对路径保持原样，或按 `site.media_base_url` 拼接。
- 整站先在同级临时目录完成生成与链接校验，再原子替换发布目录。
- 原子发布前自动将目录统一为 `0755`、文件统一为 `0644`，Nginx 无需依赖 `777` 权限读取站点。
- 单篇文章接口根据文章 ID 自动查询全部关联栏目，并从本次输出目录的旧列表和主页面补查已移除关系，联动刷新栏目分页、相关主页面和 `generated-content.js`，无需传 `column_id`；只有显式 `refresh=none` 才仅处理详情。
- 关联刷新失败时保留 `.article-refresh/{文章ID}.json`，重试完成全部刷新后自动清理，避免丢失旧栏目引用。

## 检查

```powershell
go test ./...
go vet ./...
go build ./cmd/miic-static
```
