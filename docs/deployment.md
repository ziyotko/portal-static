# 部署指南

## 1. 部署模型

每个门户使用同一个 `portal-static` 二进制，但必须使用独立的配置、监听端口、环境变量和产物目录。

```text
portal-static binary
├── CAAM process  -> configs/caam.yaml -> 127.0.0.1:9142 -> /srv/portal-static/caam/site
├── MIIC process  -> configs/miic.yaml -> 127.0.0.1:9143 -> /srv/portal-static/miic/site
└── future site   -> configs/site.yaml -> another port   -> another directory
```

静态化 HTTP 服务建议只监听回环地址或内网管理网，不直接暴露到公网。对外 Web 服务器只提供生成后的静态目录。

## 2. 前置条件

- Linux 生产主机；macOS 可用于本地开发。
- Go 版本满足 `go.mod` 要求。
- 生产模式所需的数据源可达。
- 门户模板/静态资源目录已部署，并对服务账号只读。
- 独立产物目录已规划，并对服务账号可写。
- 已生成足够随机的静态化访问令牌。

若使用 Portal CMS MySQL：

- 建议创建只读数据库账号；
- DSN 数据库名必须与 `database.schema` 相同，或在 DSN 中省略数据库名；
- 数据库时区和 `site.timezone` 应与业务发布时间语义一致。

## 3. 推荐目录

```text
/opt/portal-static/
├── releases/<version>/portal-static
└── current -> releases/<version>

/etc/portal-static/
├── caam.yaml
├── caam.env
├── miic.yaml
└── miic.env

/srv/portal-static/
├── caam/site/
├── miic/site/
└── preview/

/srv/portal-source/
├── caam/    # 只读模板和资源
└── miic/    # 只读模板和资源
```

配置文件建议使用绝对路径。`source_root` 与输出目录不能相同或互相包含。

## 4. 构建

在仓库根目录执行：

```bash
go test ./...
go test -race ./...
go vet ./...
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o build/portal-static ./cmd/portal-static
```

将二进制、目标站点配置和所需只读模板/资源部署到服务器。不要把数据库密码或 Token 写进 YAML 或提交到 Git。

## 5. 配置环境变量

以 CAAM 为例，`/etc/portal-static/caam.env`：

```bash
CAAM_DB_DSN=user:password@tcp(127.0.0.1:3306)/caam_portal?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai
CAAM_STATIC_TOKEN=请替换为随机长令牌
```

建议权限：

```bash
chown root:portal-static /etc/portal-static/caam.env
chmod 640 /etc/portal-static/caam.env
```

配置含义见[配置参考](configuration.md)。

## 6. 首次生成验收

先生成到隔离目录或配置的正式目录：

```bash
set -a
. /etc/portal-static/caam.env
set +a
/opt/portal-static/current/portal-static generate --config /etc/portal-static/caam.yaml
```

至少检查：

- 入口页和各业务主页面存在且非空；
- 栏目列表和已发布文章详情数量符合数据库；
- 草稿、未审核和已下线内容没有生成；
- 内部链接均能落到文件；
- HTML 中没有内网 IP、Windows 路径、旧域名或错误上传路径；
- 生成目录不在模板源工程内。

## 7. systemd 服务

每个站点创建一个 unit，例如 `/etc/systemd/system/portal-static-caam.service`：

```ini
[Unit]
Description=Portal Static Generator (CAAM)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=portal-static
Group=portal-static
WorkingDirectory=/opt/portal-static/current
EnvironmentFile=/etc/portal-static/caam.env
ExecStart=/opt/portal-static/current/portal-static serve --config /etc/portal-static/caam.yaml
Restart=on-failure
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/srv/portal-static/caam
ReadOnlyPaths=/srv/portal-source/caam
TimeoutStopSec=20

[Install]
WantedBy=multi-user.target
```

启动并检查：

```bash
systemctl daemon-reload
systemctl enable --now portal-static-caam
systemctl status portal-static-caam
curl --fail http://127.0.0.1:9142/healthz
journalctl -u portal-static-caam -n 100 --no-pager
```

MIIC 或后续门户复制 unit，修改服务名、配置文件、环境文件、端口和读写目录即可，不需要复制程序代码。

## 8. 对外静态站点

静态化服务只负责生成文件，不负责对公网提供这些文件。可由 Nginx、对象存储或现有发布系统托管。例如：

```nginx
server {
    listen 80;
    server_name portal.example.com;
    root /srv/portal-static/caam/site;
    index index.html;

    # 后台上传文件不属于静态化产物，必须单独映射或代理。
    # 路径要与后台 server.upload_dir_prefix 及生成 HTML 保持一致。
    location ^~ /business_portal/uploads/ {
        alias /srv/dia-platform/portal/uploads/;
    }

    location / {
        try_files $uri $uri/ =404;
    }
}
```

直接通过 `file://` 打开生成后的 HTML 无法正确解析 `/business_portal/uploads/...` 这类同源绝对路径。人工验收也应通过配置了上传目录映射的 HTTP 站点访问。若生产环境选择对象存储/CDN，则应把 `media` 配置为相应策略，并确保上传地址在生成页面所在域名下可达。

如果采用对象存储/CDN，应把本地产物同步作为发布步骤，并保证同步完成后再切换版本，不能让 CDN 读取正在生成的临时目录。

## 9. 接入 dia-platform 管理后台

在后台“基础配置 → 静态化设置”填写：

| 后台字段 | 示例 | 说明 |
| --- | --- | --- |
| 静态化输出路径 | `/srv/portal-static/caam/site` | 必须是静态化服务所在主机可写的绝对目录；其父目录也要允许创建发布临时目录 |
| 静态化程序访问地址 | `http://127.0.0.1:9142` | 从后台服务进程所在网络访问，不是浏览器访问地址 |
| 静态化程序访问令牌名 | `CAAM_STATIC_TOKEN` | 环境变量名称，不是 Token 值 |
| 首页整体变灰 | 按业务要求 | 只影响支持灰度的首页/整站生成 |

后台进程也必须配置同名环境变量，且值与静态化进程一致：

```bash
CAAM_STATIC_TOKEN=与静态化服务相同的随机长令牌
```

保存后在“静态化管理”确认服务在线，再执行一次“生成全站”。详细使用流程见[管理后台使用指南](admin-workflow.md)。

## 10. 上线与回滚

推荐升级顺序：

1. 对数据库和当前静态产物做快照或备份。
2. 部署新二进制到新的版本目录，不覆盖旧二进制。
3. 用正式配置在隔离输出目录执行 `generate` 验收。
4. 停止当前站点进程，切换 `current` 软链接并启动。
5. 检查 `/healthz`，再从后台提交一次小范围操作。
6. 最后执行全站生成并检查公网入口。

回滚时切回上一版本二进制和配置；若新版本已经发布了不兼容产物，同时恢复静态目录备份。数据库结构变更必须有独立回滚方案，不能只回滚二进制。

## 11. 本地开发环境

本地联调仍遵循“一站一进程”。可以使用 Docker 启动隔离 MySQL/Redis，用 `dia-platform` 的只读源码副本构建后台和前端；所有运行配置、日志和依赖应放在本仓库忽略的 `.local/`，不得写入三个原工程。

本地服务启动后通常检查：

```bash
curl http://127.0.0.1:9142/healthz
curl http://127.0.0.1:9143/healthz
```

具体端口以各站点 YAML 为准。
