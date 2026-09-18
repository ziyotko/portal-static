# Windows 命令行操作手册（无 Docker）

这份手册用于在单位 Windows 电脑上从 Git 重新搭建测试环境。它不依赖 Docker，也不修改 `dia-platform`、`caam-portal`、`miic-portal`；构建结果、后台上传文件和静态网站都写在 `portal-static` 的 `.local` 或 `dist` 中。

CAAM 可以使用 SQL 中的真实数据与管理后台联调。MIIC 可以直接生成 demo；真实 MIIC 联调还需要对应数据库。

## 1. 准备软件和服务

安装：

- Git for Windows；
- Go；
- Node.js LTS（带 npm）；
- PowerShell 7，命令为 `pwsh`；
- MySQL 8 客户端及一套可写 MySQL 8；
- 一套可用 Redis，至少能使用 DB 6、7、8。

MySQL 和 Redis 可以装在本机，也可以使用单位测试服务器。使用远程服务时，只需修改后文复制出来的后台配置和数据库 DSN。

打开 PowerShell 7，确认命令可用：

```powershell
git --version
go version
node --version
npm --version
mysql --version
redis-cli ping
```

最后一条应返回 `PONG`。后台启动时必须能同时连接 MySQL 和 Redis。

## 2. 克隆四个同级仓库

目录必须类似：

```text
D:\portal-work\
├── portal-static
├── dia-platform
├── caam-portal
└── miic-portal
```

执行：

```powershell
New-Item -ItemType Directory -Force D:\portal-work | Out-Null
Set-Location D:\portal-work

git clone https://github.com/ziyotko/portal-static.git
git clone https://miic.com.cn/zhangpeng/dia-platform
git clone https://miic.com.cn/zhangpeng/caam-portal.git
git clone https://github.com/ziyotko/miic-portal.git
```

单位 Git 服务如果要求登录、VPN 或证书，先完成单位网络认证。

## 3. 第一次导入 CAAM 数据库

把 SQL 放在仓库之外，例如：

```text
D:\portal-test-data\Dump20260917.sql
```

以下命令先从 dump 提取公共开头和 `caam_portal` 段，再只导入这个临时文件：

```powershell
$Dump = 'D:\portal-test-data\Dump20260917.sql'
$PortalDump = 'D:\portal-test-data\caam_portal.import.sql'
$Lines = Get-Content -LiteralPath $Dump -Encoding UTF8
$PreambleEnd = (Select-String -LiteralPath $Dump -SimpleMatch 'SQL_NOTES=0').LineNumber
$PortalStart = (Select-String -LiteralPath $Dump -SimpleMatch '-- Current Database: `caam_portal`').LineNumber

@(
  $Lines[0..($PreambleEnd - 1)]
  $Lines[($PortalStart - 1)..($Lines.Count - 1)]
) | Set-Content -LiteralPath $PortalDump -Encoding utf8NoBOM

cmd.exe /c 'mysql.exe -h 127.0.0.1 -P 3306 -u root -p < "D:\portal-test-data\caam_portal.import.sql"'
Remove-Item -LiteralPath $PortalDump
```

MySQL 会提示输入密码。导入成功后，临时提取文件会被删除，原始 SQL 不变。远程 MySQL 请替换主机和端口。

然后写入本机静态化参数。Windows 路径在 SQL 中使用 `/`：

```powershell
mysql.exe -h 127.0.0.1 -P 3306 -u root -p caam_portal -e "UPDATE setting SET captcha_enabled=0, static_path='D:/portal-work/portal-static/dist/caam-live', static_program_addr='http://127.0.0.1:9142', static_program_token_name='CAAM_STATIC_TOKEN' WHERE id=1;"
```

dump 内置的本地测试账号为：

```text
账号：admin
密码：1qaz@WSX
```

该账号只用于隔离测试环境。

## 4. 第一次构建运行文件

在 PowerShell 中执行：

```powershell
Set-Location D:\portal-work\portal-static
$Root = (Get-Location).Path

New-Item -ItemType Directory -Force `
  .\.local\windows\caam\bin, `
  .\.local\windows\caam\backend\logs, `
  .\.local\windows\caam\backend\uploads, `
  .\.local\windows\caam\frontend, `
  .\dist\caam-live | Out-Null

go build -o .\.local\windows\caam\bin\portal-static.exe .\cmd\portal-static

$env:GOWORK = 'off'
go -C ..\dia-platform\business\portal\backend build -mod=readonly `
  -o "$Root\.local\windows\caam\bin\portal-backend.exe" .
Remove-Item Env:GOWORK

Copy-Item .\configs\caam-admin.windows.example.yaml `
  .\.local\windows\caam\backend\config.yaml -Force

robocopy.exe ..\dia-platform\business\portal\frontend `
  .\.local\windows\caam\frontend /MIR /XD node_modules dist .git /XF .DS_Store

Set-Location .\.local\windows\caam\frontend
npm ci
Set-Location $Root
```

`robocopy` 返回码 0–7 都是正常结果。上面的命令只读取原工程，未在原工程内安装依赖或产生构建文件。

如果 MySQL 或 Redis 不在默认地址，编辑这份运行时副本：

```powershell
notepad .\.local\windows\caam\backend\config.yaml
```

修改 `database.host/port/username` 和 `redis.host/port/password`。不要修改 `dia-platform` 中的原始配置。

## 5. 每天启动 CAAM：开四个 PowerShell 窗口

先确认 MySQL、Redis 已启动。然后分别执行以下命令，每个窗口保持运行。

### 窗口一：静态化服务

```powershell
Set-Location D:\portal-work\portal-static
$env:CAAM_DB_DSN = 'root:你的MySQL密码@tcp(127.0.0.1:3306)/caam_portal?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai'
$env:CAAM_STATIC_TOKEN = 'portal-static-local-token'
.\.local\windows\caam\bin\portal-static.exe serve --config .\configs\caam.example.yaml
```

远程 MySQL 时同步替换 DSN 的主机和端口。

### 窗口二：管理后台

```powershell
Set-Location D:\portal-work\portal-static\.local\windows\caam\backend
$env:PORTAL_DB_PASSWORD = '你的MySQL密码'
$env:PORTAL_JWT_SECRET = 'portal-static-windows-test-jwt-secret-please-change'
$env:CAAM_STATIC_TOKEN = 'portal-static-local-token'
..\bin\portal-backend.exe
```

### 窗口三：管理前端

```powershell
Set-Location D:\portal-work\portal-static\.local\windows\caam\frontend
npm run dev -- --host 127.0.0.1
```

### 窗口四：静态网站预览

```powershell
Set-Location D:\portal-work\portal-static
npx --yes http-server .\dist\caam-live -p 8088 -c-1 `
  -P http://127.0.0.1:8084
```

这里的代理很重要：静态文件由 8088 提供，找不到的 `/business_portal/uploads/...` 图片会转发到管理后台 8084。不要用 `file:///.../index.html` 预览。

## 6. 在后台发布并查看静态网站

打开：

- 管理后台：[http://127.0.0.1:3000/business_portal/](http://127.0.0.1:3000/business_portal/)
- 静态网站：[http://127.0.0.1:8088/](http://127.0.0.1:8088/)
- 静态化健康检查：[http://127.0.0.1:9142/healthz](http://127.0.0.1:9142/healthz)

操作顺序：

1. 使用 `admin / 1qaz@WSX` 登录。
2. 在“内容管理 → 图文管理”维护文章并上传图片。
3. 完成审核和发布。
4. 进入“基础配置 → 静态化管理”，第一次直接执行“生成全站”。
5. 等任务成功后打开 8088；页面未更新时按 `Ctrl + F5`。

SQL dump 只包含图片路径，不包含图片文件。导入数据后，旧文章图片缺失是正常的；通过当前后台重新上传的图片会保存在 `portal-static\.local\windows\caam\backend\uploads`，并能经 8088 显示。

## 7. 停止、更新和重建

停止时，在四个运行窗口分别按 `Ctrl + C`。MySQL 和 Redis 是否停止由单位环境决定。

更新代码：

```powershell
Set-Location D:\portal-work\portal-static
git pull --ff-only
```

如果 Go 代码、管理后台源码或前端依赖变化，重新执行“第 4 节”的构建命令。数据库和上传文件不会因此删除。

## 8. 测试 MIIC

MIIC demo 不需要管理后台、MySQL 或 Redis：

```powershell
Set-Location D:\portal-work\portal-static
go run .\cmd\portal-static preview --config .\configs\miic.example.yaml
npx --yes http-server .\dist\miic-preview -p 8089 -c-1
```

打开 [http://127.0.0.1:8089/](http://127.0.0.1:8089/)。正常时会生成完整主页面和详情页。按 `Ctrl + C` 停止预览。

真实 MIIC 数据使用同样流程，只是要提供 `MIIC_DB_DSN` 和 `MIIC_STATIC_TOKEN`，并以 production 模式运行：

```powershell
$env:MIIC_DB_DSN = '用户:密码@tcp(数据库地址:3306)/miic_portal?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai'
$env:MIIC_STATIC_TOKEN = '单独的测试令牌'
go run .\cmd\portal-static generate --config .\configs\miic.example.yaml
go run .\cmd\portal-static serve --config .\configs\miic.example.yaml
```

CAAM 和 MIIC 必须使用各自的端口、数据库、Token 和输出目录，一站一进程。

## 9. 测试以后接入的新门户

操作流程始终相同：

```text
准备同级模板工程和配置
→ preview 生成 demo
→ 用 HTTP 服务验收完整网站
→ 配置真实数据源
→ generate 或由管理后台调用 serve
```

命令形式固定为：

```powershell
go run .\cmd\portal-static preview --config .\configs\<site>.example.yaml
go run .\cmd\portal-static generate --config .\configs\<site>.example.yaml
go run .\cmd\portal-static serve --config .\configs\<site>.example.yaml
```

只有模板和 YAML 还不能运行新门户；必须先按[新门户接入指南](adapter-development.md)实现并注册适配器。

## 10. 常见问题

### 静态网站打不开

先在后台执行一次“生成全站”。`dist\caam-live` 尚未生成时，8088 返回 404 是正常的。

### 图片不显示

确认：

1. 访问的是 `http://127.0.0.1:8088/`，不是 `file://`；
2. 预览命令带有 `-P http://127.0.0.1:8084`；
3. 管理后台 8084 正在运行；
4. 图片文件确实存在于 `.local\windows\caam\backend\uploads`；
5. dump 中的旧图片文件需要另行取得，SQL 本身不包含图片。

### 后台启动后立即退出

查看终端报错。最常见原因是 MySQL/Redis 未启动、密码错误，或运行时 `config.yaml` 地址不正确。

### 端口占用

```powershell
Get-NetTCPConnection -State Listen |
  Where-Object LocalPort -In 3000,8084,8088,9142,9143 |
  Format-Table LocalAddress,LocalPort,OwningProcess
```

先停止自己启动的旧测试进程，不要随意结束不认识的单位软件进程。

## 11. 明天最短清单

1. 克隆四个同级仓库。
2. 确认 MySQL 8 和 Redis 可用。
3. 按第 3 节导入 `caam_portal`。
4. 按第 4 节构建一次。
5. 开四个 PowerShell 窗口，逐一运行第 5 节命令。
6. 登录 3000，生成全站，在 8088 查看。
