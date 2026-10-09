# CAMIE 生产部署研发交接说明

## 1. 文档目的

本文档用于研发向运维说明 CAMIE 系统生产部署涉及的系统组成、构建要求、配置项、数据库变更、持久化数据及上线注意事项。

具体服务器环境、构建方式、部署目录、进程管理、Nginx、TLS、数据库备份、日志及监控等，由运维根据实际生产环境及现有运维规范确定。

---

## 2. 系统组成

CAMIE 当前主要包含：

```text
Portal
Member
portal-static
Portal 前端
Member 前端
门户静态站点
```

其中：

- **Portal**：门户后台管理服务
- **Member**：会员系统服务
- **portal-static**：门户静态页面生成服务
- **Portal 前端**：门户后台管理前端
- **Member 前端**：会员系统前端
- **门户静态站点**：由 portal-static 根据后台数据生成

系统关系大致如下：

```text
                     用户访问
                        │
                        ▼
                     Nginx
                        │
            ┌───────────┼───────────┐
            │           │           │
            ▼           ▼           ▼
         Portal       Member      静态站点
            │           │
            └─────┬─────┘
                  │
             MySQL / Redis

         Portal
            │
            ▼
      portal-static
            │
            ▼
       静态站点文件
```

具体端口、目录及反向代理方式由运维根据生产环境确定。

---

## 3. 构建说明

系统包含 Go 后端和前端工程。

构建涉及：

```text
Go
Node.js
npm
```

当前工程中已经提供统一构建脚本：

```text
scripts/build-camie-release.ps1
```

该脚本主要完成：

- Portal 后端构建
- Member 后端构建
- portal-static 构建
- Portal 前端构建
- Member 前端构建
- 数据库发布 SQL 生成
- 配置及部署资源整理
- 构建结果校验

当前脚本默认包含 Linux amd64 构建逻辑。

如实际生产环境不同，由运维根据实际环境调整构建方式。

---

## 4. 部署参考资源

工程中提供：

```text
deploy/camie/
```

主要包括：

```text
config/
nginx/
scripts/
sql/
systemd/
README.md
```

其中包含：

- 应用配置模板
- Nginx 参考配置
- systemd 参考配置
- 数据库升级相关脚本
- 配置检查脚本
- 健康检查及辅助脚本

以上内容作为部署参考，具体使用方式由运维根据生产环境确定。

---

## 5. 生产配置

当前主要包括以下配置：

```text
portal.yaml
portal.env

member.yaml
member.env

portal-static.yaml
portal-static.env
```

正式部署时需要根据生产环境填写实际配置。

主要包括以下内容。

### 数据库

需要配置：

```text
数据库地址
数据库端口
数据库名称
数据库用户
数据库密码
```

---

### Redis

需要配置：

```text
Redis 地址
Redis 端口
Redis 密码
Redis DB
```

---

### 正式访问地址

应用中的生产访问地址需要根据正式域名或实际生产地址配置。

---

## 6. 关键共享配置

以下配置存在跨服务一致性要求。

### JWT

Portal：

```text
PORTAL_MEMBER_JWT_SECRET
```

Member：

```text
MEMBER_JWT_SECRET
```

两个值必须完全一致。

---

### 静态化服务 Token

Portal 和 portal-static 使用：

```text
CAMIE_STATIC_TOKEN
```

两个服务必须配置相同的 Token。

生产环境密码、Token、JWT Secret 等敏感信息由运维生成和管理，不应使用开发或测试环境值。

---

## 7. Member 中文字体

Member 部分功能需要使用中文 TTF / OTF 字体。

配置项：

```text
MEMBER_CERT_FONT
```

生产环境需要填写服务器上实际可用的字体文件路径。

具体字体安装目录由运维确定。

---

## 8. 数据库变更

当前部署涉及数据库升级。

工程中提供：

```text
preflight.sql
migrate.sql
rollback.sql
```

### preflight.sql

用于正式升级前检查当前生产数据库是否满足迁移条件。

所有：

```text
BLOCKER
```

必须为 `0` 后才允许执行数据库升级。

---

### migrate.sql

用于执行当前版本涉及的：

- 数据库结构变更
- 模板变更
- 栏目及配置变更
- 静态化相关基础数据变更

数据库关联使用稳定业务 Code，不依赖开发环境中的数据库自增 ID。

---

### rollback.sql

用于回滚当前版本对应的数据库变更。

正式执行数据库升级前，应按照生产环境现有规范完成数据库备份。

具体备份方式、备份位置及恢复流程由运维确定。

---

## 9. 历史数据库兼容

如果现有生产数据库仍存在旧版本结构，例如：

```text
page
page_id
```

或会员专区旧字段等历史结构，需要先完成对应数据库升级。

不得绕过 `preflight.sql` 检查直接执行数据库迁移。

---

## 10. 静态化服务

Portal 会调用 `portal-static` 生成门户静态页面。

主要涉及以下配置：

```text
static_path
static_program_addr
static_program_token_name
```

### static_path

静态站点生成目录。

### static_program_addr

Portal 调用 portal-static 的内部服务地址。

### static_program_token_name

静态化服务调用所使用的 Token 配置名称，对应：

```text
CAMIE_STATIC_TOKEN
```

具体路径、地址及端口由运维根据生产环境配置。

portal-static 的 `portal-static.yaml` 同时包含：

```text
paths.dist_root
paths.allowed_output_root
```

`paths.dist_root` 是 Portal 未传 `static_path` 时的默认生成目录；Portal 配置了 `static_path` 时优先使用该路径，但它必须等于 `paths.allowed_output_root` 或位于其下级。生产环境若只允许写入一个正式目录，可将两者配置为同一路径。

---

## 11. 持久化数据

以下内容属于生产业务数据，程序升级过程中不能直接覆盖或删除：

```text
Portal 上传文件
Portal 私有上传文件
Member 上传文件
门户静态站点文件
生产配置文件
数据库
```

其中：

**Portal 私有上传文件禁止直接通过 Nginx 静态目录对外暴露。**

具体服务器存储目录由运维根据生产环境确定。

---

## 12. 健康检查

当前服务提供以下基本检查入口。

Portal：

```text
/api/site-info
```

Member：

```text
/api/site-info
```

portal-static：

```text
/healthz
```

Portal 和 Member 当前不使用 `/healthz`。

---

## 13. 上线验收

部署完成后建议至少确认以下内容：

- Portal 服务正常
- Member 服务正常
- portal-static 服务正常
- Portal 管理后台可正常访问
- Member 页面可正常访问
- 门户首页可正常访问
- 栏目页可正常访问
- 详情页可正常访问
- Portal / Member 数据库连接正常
- Redis 连接正常
- 上传文件功能正常
- Portal 私有文件无法通过静态地址直接访问
- 后台“生成全站”功能正常
- portal-static 能够正常生成静态页面

---

## 14. 回滚

当前版本提供数据库：

```text
rollback.sql
```

用于数据库相关变更回滚。

程序版本回滚、数据库恢复、静态站点恢复及生产发布回滚流程，由运维按照现有生产规范执行。

研发负责保证当前版本提供的数据库迁移和回滚逻辑与程序版本对应。

---

## 15. 已知问题

当前会员页面会在浏览器侧判断：

```text
status=active
```

但 Portal 的 `member-zone` API 当前不会实时再次校验会员状态。

因此：

非 active 用户如果仍持有未过期 Token，在特定情况下可能继续读取会员内容。

该问题属于当前版本已知问题，需要保留后续研发修复任务。

生产验收时不能将当前实现描述为：

```text
服务端已完成会员状态实时校验
```

---

## 16. 研发交接内容

研发主要向运维说明以下内容：

- 系统包含哪些服务
- 各服务之间的关系
- 构建入口及构建注意事项
- 应用需要哪些生产配置
- 哪些配置必须在多个服务之间保持一致
- 数据库是否存在升级
- 数据库升级及回滚脚本
- 哪些目录属于持久化数据
- 哪些数据不能在升级过程中删除
- 服务健康检查方式
- 上线后的基本验收点
- 当前版本已知问题

生产服务器环境、构建环境、部署方式、目录规划、网络、安全、TLS、服务托管、日志、监控、备份及发布流程由运维根据实际生产环境确定。

---

## 17. 本次上线信息

每次上线前研发补充以下内容即可：

```text
版本名称：

本次是否存在数据库变更：是 / 否

本次主要变更：

本次特殊注意事项：

数据库升级脚本：
- preflight.sql
- migrate.sql
- rollback.sql

已知问题：

研发联系人：
```
