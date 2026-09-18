# macOS 本机傻瓜版操作手册

这份手册只讲“在当前这台 Mac 上怎么操作”。命令默认从本仓库根目录执行：

明天在单位 Windows 电脑从 Git 重新搭建时，请使用 [Windows 命令行操作手册（无 Docker）](windows-quickstart.md)。

```bash
cd /Users/zhitianbai/Project/portal-static
```

## 0. 只记住三件事

1. 先启动 Docker Desktop。
2. CAAM 日常环境只需要执行一条启动命令。
3. 预览静态网站必须用 `http://127.0.0.1:8088/`，不要双击 HTML，也不要用 `file://`。

## 1. CAAM：一条命令启动全部环境

启动：

```bash
./.local/caam-integration/control.sh start
```

查看是否都正常：

```bash
./.local/caam-integration/control.sh status
```

正常时应该看到：

- MySQL 容器：`portal-static-caam-local-mysql`
- Redis 容器：`portal-static-caam-local-redis`
- 门户预览容器：`portal-static-caam-local-site`
- `3000`、`8084`、`9142` 三个监听端口
- 门户预览容器监听 `8088`

如果 Docker Desktop 没启动，先启动 Docker Desktop，再重新执行 `start`。

## 2. CAAM：这些地址分别做什么

| 地址 | 用途 | 是否直接打开 |
| --- | --- | --- |
| [http://127.0.0.1:3000/business_portal/](http://127.0.0.1:3000/business_portal/) | 门户管理后台 | 是 |
| [http://127.0.0.1:8088/](http://127.0.0.1:8088/) | 后台生成出来的 CAAM 静态网站 | 是 |
| [http://127.0.0.1:9142/healthz](http://127.0.0.1:9142/healthz) | CAAM 静态化服务健康检查 | 仅排障 |
| `http://127.0.0.1:8084` | 管理后台 API | 不需要直接打开 |
| `127.0.0.1:63306` | 本地 MySQL | 不用浏览器打开 |
| `127.0.0.1:63790` | 本地 Redis | 不用浏览器打开 |

当前本地测试后台账号：

```text
账号：admin
密码：1qaz@WSX
```

这是本机开发账号，不能作为生产环境密码。

## 3. CAAM：后台发布文章并查看结果

### 第一步：进入后台

打开：

[http://127.0.0.1:3000/business_portal/](http://127.0.0.1:3000/business_portal/)

使用本地测试账号登录。

### 第二步：维护内容

1. 进入“内容管理 → 图文管理”。
2. 新建或编辑文章。
3. 选择正确的发布栏目。
4. 填写标题、正文、封面和发布时间。
5. 保存并提交审核。
6. 完成全部栏目审核，直到文章状态变为“已发布”。

文章审核通过后，后台会尽力生成文章详情页；但首页和栏目列表不会因此全部自动刷新。

### 第三步：生成静态页面

进入“基础配置 → 静态化管理”。

根据变化范围选择：

| 你做了什么 | 点哪个按钮 |
| --- | --- |
| 新文章只需要进入某个栏目 | 先生成对应栏目页 |
| 新文章需要出现在首页 | 生成首页 |
| 不确定受影响范围 | 生成全站 |
| 更新了模板或大量文章 | 生成全站 |
| 只修一篇文章正文 | 在详情列表重新生成该文章 |

第一次操作或准备验收时，直接点“生成全站”最省心。

### 第四步：等待任务完成

任务状态变成“执行成功”后再预览。如果显示失败或中断，先查看错误，不要连续重复点击。

### 第五步：预览门户

打开：

[http://127.0.0.1:8088/](http://127.0.0.1:8088/)

如果浏览器还显示旧内容，执行强制刷新：

```text
macOS：Command + Shift + R
```

不要使用下面这种地址：

```text
file:///Users/zhitianbai/Project/portal-static/dist/caam-live/index.html
```

因为 HTML 中的 `/business_portal/uploads/...` 是网站绝对路径，只有通过 8088 的 Web 服务才能映射到后台上传目录。

## 4. CAAM：停止、重启和恢复

停止全部本地环境：

```bash
./.local/caam-integration/control.sh stop
```

重新启动：

```bash
./.local/caam-integration/control.sh start
```

一起重启：

```bash
./.local/caam-integration/control.sh restart
```

停止不会删除 MySQL 数据。除非明确要重建测试库，否则不要删除 Docker volume，也不要删除 `.local/caam-integration/backend/uploads`。

## 5. CAAM：改过 portal-static 代码后怎么办

代码发生变化后，重新编译静态化程序，再重启：

```bash
go build -o .local/caam-integration/bin/portal-static ./cmd/portal-static
./.local/caam-integration/control.sh restart
```

然后在后台重新执行一次“生成全站”，再打开 8088 验收。

## 6. MIIC：先做无数据库完整预览

如果只是检查 MIIC 模板和完整生成流程，不需要启动管理后台，也不需要数据库。

生成 MIIC demo 站点：

```bash
go run ./cmd/portal-static preview --config configs/miic.example.yaml
```

正常结果会显示 `mode=preview`、`driver=miic` 和“整站生成完成”。当前 demo 会生成 4 个主页面和 6 个文章详情。

生成目录：

```text
/Users/zhitianbai/Project/portal-static/dist/miic-preview
```

再启动一个临时 HTTP 预览服务：

```bash
python3 -m http.server 8089 \
  --bind 127.0.0.1 \
  --directory /Users/zhitianbai/Project/portal-static/dist/miic-preview
```

这个命令会占用当前终端，保持它运行，然后打开：

[http://127.0.0.1:8089/](http://127.0.0.1:8089/)

结束预览时回到该终端按 `Control + C`。

MIIC preview 使用内置 demo，验证的是完整页面、栏目、详情和资源流程，不代表生产数据库内容。

## 7. MIIC：连接真实数据库运行静态化服务

需要真实 MIIC 数据时，先准备 MIIC 数据库和两个环境变量：

```bash
export MIIC_DB_DSN='数据库用户:数据库密码@tcp(数据库地址:3306)/miic_portal?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai'
export MIIC_STATIC_TOKEN='请替换为随机本地令牌'
```

先直接生成一次真实站点：

```bash
go run ./cmd/portal-static generate --config configs/miic.example.yaml
```

默认输出目录：

```text
/Users/zhitianbai/Project/portal-static/dist/miic-portal
```

启动 MIIC 静态化服务：

```bash
go run ./cmd/portal-static serve --config configs/miic.example.yaml
```

保持该终端运行。MIIC 默认地址：

```text
http://127.0.0.1:9143
```

健康检查：

```bash
curl http://127.0.0.1:9143/healthz
```

如果只想直接测试整站 API：

```bash
curl -X POST \
  -H "Authorization: Bearer $MIIC_STATIC_TOKEN" \
  http://127.0.0.1:9143/api/static/site
```

接口返回 job 后，使用响应里的 `status_url` 继续查询，直到状态为 `succeeded`。

## 8. MIIC：也通过管理后台维护数据

要像 CAAM 一样通过后台维护 MIIC，必须为 MIIC 单独启动一套后台实例，连接 MIIC 自己的数据库。不要让 MIIC 和 CAAM 共用同一个后台进程、数据库 schema、Redis DB 或静态输出目录。

推荐端口：

| 组件 | CAAM | MIIC |
| --- | --- | --- |
| 管理前端 | 3000 | 3001 |
| 管理后台 API | 8084 | 8085 |
| 静态化 API | 9142 | 9143 |
| 门户预览 | 8088 | 8089 |

MIIC 后台“静态化设置”填写：

```text
静态化输出路径：/Users/zhitianbai/Project/portal-static/dist/miic-live
静态化程序访问地址：http://127.0.0.1:9143
静态化程序访问令牌名：MIIC_STATIC_TOKEN
```

后台进程和 MIIC 静态化进程必须同时具有同值的 `MIIC_STATIC_TOKEN`。MIIC 上传目录也必须通过预览 Web 服务映射，不能用 `file://` 验收。

当前本机只完成了 CAAM 管理后台的一键环境；MIIC 的真实后台环境需要在拿到 MIIC 数据库后按上述端口单独配置。无数据库 MIIC preview 已经可以随时运行。

## 9. 以后测试第三个或更多新门户

### 情况 A：适配器已经开发完成

假设新门户 driver 是 `example`，配置文件是 `configs/example.example.yaml`。

第一步，生成无数据库完整预览：

```bash
go run ./cmd/portal-static preview --config configs/example.example.yaml
```

第二步，用 HTTP 服务查看 `paths.preview_root`：

```bash
python3 -m http.server 8090 \
  --bind 127.0.0.1 \
  --directory /绝对路径/到/preview_root
```

第三步，按适配器要求准备真实数据源环境变量和 Token。以下以 Portal CMS MySQL 为例：

```bash
export EXAMPLE_DB_DSN='...'
export EXAMPLE_STATIC_TOKEN='...'
```

第四步，直接生成真实站点：

```bash
go run ./cmd/portal-static generate --config configs/example.example.yaml
```

第五步，启动独立静态化进程：

```bash
go run ./cmd/portal-static serve --config configs/example.example.yaml
```

第六步，为它分配独立管理后台、数据库、Redis DB、静态输出目录和门户预览端口。

### 情况 B：只有模板或旧门户工程，还没有适配器

仅新增 YAML 并修改 `driver` 不能接入新门户。必须先：

1. 在 `internal/adapters/<driver>` 实现完整适配器。
2. 提供自包含 demo 和完整 preview。
3. 注册到 `internal/adapters/builtin`。
4. 通过公共 HTTP 契约测试。
5. 用真实数据在隔离目录验收。

具体开发步骤见[新门户接入指南](adapter-development.md)。

## 10. 新门户固定端口登记表

接入新门户时先登记，避免端口和目录冲突：

| 项目 | 填写内容 |
| --- | --- |
| driver | 例如 `example` |
| 配置文件 | `configs/example.example.yaml` |
| 数据库/schema | 例如 `example_portal` |
| DSN 环境变量 | `EXAMPLE_DB_DSN` |
| Token 环境变量 | `EXAMPLE_STATIC_TOKEN` |
| 管理前端端口 | 不与现有端口重复 |
| 管理后台端口 | 不与现有端口重复 |
| 静态化 API 端口 | 不与现有端口重复 |
| 门户预览端口 | 不与现有端口重复 |
| 模板源目录 | 只读绝对路径 |
| 正式输出目录 | 独立可写绝对路径 |
| 预览输出目录 | 独立可写绝对路径 |
| 上传目录映射 | URL 前缀 → 实际上传目录或对象存储 |

## 11. 出问题时先做这四步

```bash
cd /Users/zhitianbai/Project/portal-static
./.local/caam-integration/control.sh status
curl http://127.0.0.1:9142/healthz
curl -I http://127.0.0.1:8088/
```

然后检查：

1. 后台静态化任务是否真的 `succeeded`。
2. 浏览器打开的是 `http://127.0.0.1:8088/`，不是 `file://`。
3. 图片 URL 通过 8088 是否返回 `200`。
4. 是否需要 `Command + Shift + R` 强制刷新。

更详细的故障清单见[运维与排障手册](operations.md)。
