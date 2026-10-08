# CAMIE Linux 生产部署交接

本目录随发布包交付。目标为 Linux x86_64、Nginx、Portal/Member/portal-static 同机部署；服务器不安装 Go、Node/npm，也不执行源码构建。

## 1. 上传与校验

构建机会生成两个文件：

```text
camie-release-<version>-linux-amd64.tar.gz
camie-release-<version>-linux-amd64.tar.gz.sha256
```

上传后先校验，失败即停止：

```bash
sha256sum -c camie-release-<version>-linux-amd64.tar.gz.sha256
sudo mkdir -p /opt/camie/releases
sudo tar -xzf camie-release-<version>-linux-amd64.tar.gz -C /opt/camie/releases
cd /opt/camie/releases/camie-release-<version>
sha256sum -c SHA256SUMS
file bin/portal bin/member bin/portal-static
```

三个二进制都必须显示 Linux ELF 64-bit x86-64。服务器不得使用 `.exe` 或 `.local` 目录。

## 2. 首次目录与配置

Windows 构建的 tar 不承诺保留 Linux 可执行位，因此必须以 root 通过 Bash 启动安装脚本：

```bash
sudo bash ops/scripts/install-layout.sh /opt/camie/releases/camie-release-<version>
```

脚本会立即将版本目录收归 `root:root`、移除组/其他用户写权限，并把三个二进制和运维脚本设为 `0755`；随后创建用户、目录、配置软链接和服务文件。它不会改数据库、切换当前版本或启动服务。然后：

1. 将 `config/*.yaml` 中的 `__CAMIE_ORIGIN__`、数据库和 Redis地址改成生产值，复制到 `/etc/camie/`。
2. 从 `config/*.env.example` 创建对应 `.env`，写入真实密码与随机密钥，权限设为 `0640 root:camie`。
3. 确认 `PORTAL_MEMBER_JWT_SECRET` 与 `MEMBER_JWT_SECRET` 完全相同；Portal 和 portal-static 的 `CAMIE_STATIC_TOKEN` 完全相同。
4. 安装 TTF/OTF 中文字体并填写 `MEMBER_CERT_FONT`。
5. 将 Nginx 文件中的 `__CAMIE_DOMAIN__` 替换为正式域名；证书由运维现有 TLS 流程配置。

生产目录：

```text
/opt/camie/releases/<version>       发布版本
/opt/camie/current                  当前版本软链接
/etc/camie                          配置与环境变量
/srv/camie/site                     公开静态站
/srv/camie/portal/uploads           Portal 公开上传
/srv/camie/portal/private_uploads   Portal 私有上传，禁止 Nginx alias
/srv/camie/member/uploads           Member 上传
/var/log/camie                      日志
```

## 3. 数据库

现有生产库不得导入本地测试 SQL。先用 MySQL `mysql_config_editor` 配置只供备份使用的 `camie-backup` login-path，再完整备份两库、三类上传目录、配置和 `/srv/camie/site`：

```bash
sudo MYSQL_LOGIN_PATH=camie-backup ops/scripts/backup-production.sh \
  /安全备份目录/camie-before-<version>
```

数据库模板包含在 `camie_portal` 全库备份中。备份校验通过后，再在生产库副本执行：

```bash
mysql --table -u <migration-user> -p camie_portal < db/preflight.sql
mysql --default-character-set=utf8mb4 -u <migration-user> -p camie_portal < db/migrate.sql
```

`preflight.sql` 的 `BLOCKER` 必须全部为 0。旧 `page/page_id`、会员专区旧列等问题按 dia-platform 的 DEPLOY 文档先处理；不要绕过迁移保护。`migrate.sql` 按稳定 code 更新，不使用本地自增 ID，并保存该发布版本的回滚快照。

数据库 `setting` 迁移后必须为：

```text
static_path=/srv/camie/site
static_program_addr=http://127.0.0.1:9144
static_program_token_name=CAMIE_STATIC_TOKEN
```

## 4. 启动与验收

```bash
sudo ops/scripts/verify-config.sh /opt/camie/releases/camie-release-<version>
sudo ln -sfn /opt/camie/releases/camie-release-<version> /opt/camie/current
sudo systemctl daemon-reload
sudo systemctl restart camie-portal camie-member camie-static
sudo nginx -t
sudo systemctl reload nginx
ops/scripts/smoke-test.sh https://<正式域名>
```

首次启动后从后台执行“生成全站”，确认输出是 `/srv/camie/site`。Portal 和 Member 的探活是各自 `/api/site-info`，不是 `/healthz`；只有 portal-static 使用 `/healthz`。

## 5. 回滚

暂停内容发布，保存故障日志，然后：

1. 执行当前包的 `db/rollback.sql`，恢复迁移前模板、栏目绑定和静态化设置；新增记录只会被禁用，不会删除业务数据。
2. 将 `/opt/camie/current` 指回上一版本。
3. 恢复上一版静态产物；必要时恢复部署前数据库备份。
4. 重启三个服务并 reload Nginx，再跑冒烟测试。

## 6. 已接受风险

当前会员页面只在浏览器侧确认 `status=active`，Portal 的 `member-zone` API 不实时复核会籍状态。非 active 用户持未过期 Token 时可能绕过页面读取会员内容。该风险已被接受上线，但不能在验收记录中描述为“服务端已校验会员状态”，应保留后续修复工单。
