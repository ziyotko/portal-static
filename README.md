# portal-static

统一门户静态化服务。当前已接入 MIIC 适配器，通用配置、HTTP API、任务管理、数据库访问和媒体 URL 解析均位于本工程；`miic-portal` 只作为只读的页面脚手架。

## 快速验证

```bash
go test ./...
go run ./cmd/portal-static preview --config configs/miic.example.yaml
```

预览输出到 `dist/miic-preview`。该命令使用内置演示数据，不连接数据库，也不会写入 `miic-portal`。

## 生产运行

复制配置后设置数据库和 API 令牌环境变量：

```bash
cp configs/miic.example.yaml configs/miic.yaml
export MIIC_DB_DSN='user:password@tcp(127.0.0.1:3306)/miic_portal?charset=utf8mb4&parseTime=true'
export MIIC_STATIC_TOKEN='replace-with-a-random-token'
go run ./cmd/portal-static serve --config configs/miic.yaml
```

整站一次性生成使用：

```bash
go run ./cmd/portal-static generate --config configs/miic.yaml
```

`media.mode: same_origin` 会将受管媒体输出为站点根路径，切换域名时无需改静态文件；如需独立媒体域名，可改为 `cdn` 并设置 `media.base_url`。
