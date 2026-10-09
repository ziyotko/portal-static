# Windows 日常运行（直接照着执行）

这份文档只解决一件事：环境已经安装好以后，如何启动 `portal-static`、管理后台和本地静态网站。

首次安装、数据库导入和生产部署不在这里说明。需要这些内容时再看 [Windows 完整操作手册](windows-quickstart.md)。

## 先看结论

- CAAM 静态化服务使用 `9142` 端口和 `caam_portal` 数据库。
- MIIC 静态化服务使用 `9143` 端口和 `miic_portal` 数据库。
- 当前 CAMIE 本机联调环境使用统一入口 `18080`、Portal API `18092`、Member API `18093`、静态化 API `19144`。
- CAMIE 的 Portal/静态化连接 `camie_portal`，Member 连接独立的 `camie_member`。
- CAMIE 手动运行时需要五个 PowerShell 窗口，分别运行 Redis、Portal、Member、portal-static 和本地同源网站。
- CAAM 和 MIIC 可以同时启动，但必须分别占用一个 PowerShell 窗口。
- 每个窗口中的命令都要在同一个窗口内连续执行。
- 命令中的 `你的MySQL密码` 要替换为实际密码，不要在 `@` 或 `_` 前面添加反斜杠 `\`。

## 一、手动启动 CAMIE 联调环境

以下命令按组件逐个执行。环境首次安装、数据库恢复和前端重新构建不属于日常启动；这里假设 `.local\camie-integration` 和 `.local\redis` 已经准备好。

五个窗口都要保持打开。不要把一个窗口中的环境变量设置好以后，换到另一个窗口启动程序；PowerShell 环境变量只对当前窗口及其子进程生效。

### CAMIE 窗口一：Redis

```powershell
Set-Location D:\WebstormProjects\portal-static\.local\redis\8.10.2\Redis-8.10.2-Windows-x64-msys2

.\redis-server.exe ..\..\camie-redis.conf
```

看到 Redis 已准备接受连接后保持窗口打开。若提示 `6379` 已被占用，先确认是否已有可用的 Redis，不要重复启动第二个实例。

### CAMIE 窗口二：Portal 后端

```powershell
Set-Location D:\WebstormProjects\portal-static\.local\camie-integration

$dbPassword = (Get-Content .\db-password -Raw).Trim()
$env:PORTAL_DB_PASSWORD = $dbPassword
$env:PORTAL_JWT_SECRET = (Get-Content .\portal-jwt-secret -Raw).Trim()
$env:PORTAL_MEMBER_JWT_SECRET = (Get-Content .\member-jwt-secret -Raw).Trim()
$env:CAMIE_STATIC_TOKEN = (Get-Content .\static-token -Raw).Trim()

Set-Location .\portal-runtime
.\portal-backend.exe
```

Portal API 监听 `127.0.0.1:18092`。这个进程连接 `camie_portal`，并通过 Redis DB `6/7/8` 工作；会员登出黑名单读取 Member 使用的 DB `4`。

### CAMIE 窗口三：Member 后端

```powershell
Set-Location D:\WebstormProjects\portal-static\.local\camie-integration

$dbPassword = (Get-Content .\db-password -Raw).Trim()
$env:MEMBER_DB_PASSWORD = $dbPassword
$env:MEMBER_JWT_SECRET = (Get-Content .\member-jwt-secret -Raw).Trim()

Set-Location .\member-runtime
.\member-backend.exe
```

Member API 监听 `127.0.0.1:18093`，只连接 `camie_member`，使用 Redis DB `4/5`。`MEMBER_JWT_SECRET` 与 Portal 窗口中的 `PORTAL_MEMBER_JWT_SECRET` 必须来自同一个文件、保持相同。

### CAMIE 窗口四：静态化服务

```powershell
Set-Location D:\WebstormProjects\portal-static\.local\camie-integration

$dbPassword = (Get-Content .\db-password -Raw).Trim()
$env:CAMIE_DB_DSN = "camie_local:$dbPassword@tcp(127.0.0.1:3306)/camie_portal?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai"
$env:CAMIE_STATIC_TOKEN = (Get-Content .\static-token -Raw).Trim()

.\portal-static.exe serve --config .\portal-static.yaml
```

看到静态化服务监听 `127.0.0.1:19144` 后保持窗口打开。

健康检查地址：[http://127.0.0.1:19144/healthz](http://127.0.0.1:19144/healthz)

### CAMIE 窗口五：本地同源网站

```powershell
Set-Location D:\WebstormProjects\portal-static\.local\camie-integration

python .\local-site.py
```

这个本地网站同时提供 CAMIE 静态文件、Portal/Member 前端，并把两个 API 和上传请求转发到对应后端；它支持后台提交审核需要的 `PATCH`。不要用普通的 `python -m http.server` 或只配置一个上游的 `http-server -P` 替代它。

统一入口：[http://127.0.0.1:18080/](http://127.0.0.1:18080/)

Portal 后台：[http://127.0.0.1:18080/business_portal/](http://127.0.0.1:18080/business_portal/)

Member 登录：[http://127.0.0.1:18080/business_member/login](http://127.0.0.1:18080/business_member/login)

### CAMIE 启动后检查

按顺序打开或执行：

```powershell
Invoke-RestMethod http://127.0.0.1:19144/healthz
Invoke-RestMethod http://127.0.0.1:18080/business_portal/api/site-info
Invoke-RestMethod http://127.0.0.1:18080/business_member/api/site-info
```

三个请求都成功后，再登录 Portal 后台进行内容维护和全站生成。

## 二、手动启动 CAAM 或 MIIC 静态化服务

只启动需要使用的站点；两个站点都需要时，分别打开两个 PowerShell 窗口。

### 窗口一：启动 CAAM

前提：MySQL 中已经存在 `caam_portal` 数据库。

```powershell
Set-Location D:\WebstormProjects\portal-static

$env:CAAM_DB_DSN = 'root:你的MySQL密码@tcp(127.0.0.1:3306)/caam_portal?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai'
$env:CAAM_STATIC_TOKEN = 'caam-portal-static-local-token'

go run .\cmd\portal-static serve --config .\configs\caam.example.yaml
```

看到下面的信息表示启动成功：

```text
"静态化服务已启动","addr":"127.0.0.1:9142","site":"caam"
```

这个窗口不要关闭。

健康检查地址：[http://127.0.0.1:9142/healthz](http://127.0.0.1:9142/healthz)

### 窗口二：启动 MIIC

前提：MySQL 中已经存在 `miic_portal` 数据库。

```powershell
Set-Location D:\WebstormProjects\portal-static

$env:MIIC_DB_DSN = 'root:你的MySQL密码@tcp(127.0.0.1:3306)/miic_portal?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai'
$env:MIIC_STATIC_TOKEN = 'miic-portal-static-local-token'

go run .\cmd\portal-static serve --config .\configs\miic.example.yaml
```

看到下面的信息表示启动成功：

```text
"静态化服务已启动","addr":"127.0.0.1:9143","site":"miic"
```

这个窗口不要关闭。

健康检查地址：[http://127.0.0.1:9143/healthz](http://127.0.0.1:9143/healthz)

## 三、启动 CAAM 或 MIIC 管理后台

管理后台必须在启动前拿到与静态化服务相同的 Token。

本节命令用于 CAAM、MIIC。CAMIE 的 Portal 和 Member 已在第一节分别启动，不要再运行一份占用相同端口的后端。

如果后台已经启动，先按 `Ctrl + C` 停止，再设置环境变量并重新启动。后台启动以后，在其他 PowerShell 窗口中设置环境变量不会生效。

### 使用命令行启动当前管理后台

```powershell
Set-Location D:\WebstormProjects\dia-platform\business\portal\backend

$env:PORTAL_DB_PASSWORD = '你的MySQL密码'
$env:PORTAL_JWT_SECRET = 'portal-local-jwt-secret-please-change-2026'
$env:CAAM_STATIC_TOKEN = 'caam-portal-static-local-token'
$env:MIIC_STATIC_TOKEN = 'miic-portal-static-local-token'

go run .
```

如果通过 WebStorm 或 GoLand 启动后台，把下面四项加入运行配置的环境变量，然后重启后台：

```text
PORTAL_DB_PASSWORD=你的MySQL密码
PORTAL_JWT_SECRET=portal-local-jwt-secret-please-change-2026
CAAM_STATIC_TOKEN=caam-portal-static-local-token
MIIC_STATIC_TOKEN=miic-portal-static-local-token
```

### 后台连接哪个站点

查看管理后台的 `backend/config.yaml`：

```yaml
database:
  dbname: caam_portal
```

- `dbname: caam_portal`：当前后台管理 CAAM。
- `dbname: miic_portal`：当前后台管理 MIIC。

一个后台进程只连接一个数据库。即使 CAAM 和 MIIC 两个静态化服务都已经启动，当前后台也只会管理其 `dbname` 指定的站点。

## 四、在管理后台填写静态化配置

进入“基础配置 → 静态化设置”。根据后台当前连接的数据库，只填写对应站点的一组配置。

### CAAM

```text
静态化输出路径：D:/WebstormProjects/portal-static/dist/caam-live
静态化程序访问地址：http://127.0.0.1:9142
静态化程序访问令牌名：CAAM_STATIC_TOKEN
```

### MIIC

```text
静态化输出路径：D:/WebstormProjects/portal-static/dist/miic-live
静态化程序访问地址：http://127.0.0.1:9143
静态化程序访问令牌名：MIIC_STATIC_TOKEN
```

### CAMIE 当前本机联调环境

```text
静态化输出路径：D:/WebstormProjects/portal-static/.local/camie-integration/site
静态化程序访问地址：http://127.0.0.1:19144
静态化程序访问令牌名：CAMIE_STATIC_TOKEN
```

注意：

- “令牌名”填写对应站点的 `CAAM_STATIC_TOKEN`、`MIIC_STATIC_TOKEN` 或 `CAMIE_STATIC_TOKEN`。
- 不要填写 `$env:`。
- 不要把实际令牌值 `caam-portal-static-local-token` 或 `miic-portal-static-local-token` 填到“令牌名”中。
- Windows 输出路径建议使用 `/`，不要只填写相对路径。

## 五、生成全站

按下面顺序操作：

1. 打开对应站点的健康检查地址，确认页面正常返回。
2. 确认管理后台已经设置了对应的 Token，并在设置后重新启动过。
3. 在管理后台保存静态化配置。
4. 进入“静态化管理”，点击“生成全站”。
5. 等待任务显示成功。
6. 检查输出目录中是否已经生成 `index.html`。

CAAM 输出目录：

```text
D:\WebstormProjects\portal-static\dist\caam-live
```

MIIC 输出目录：

```text
D:\WebstormProjects\portal-static\dist\miic-live
```

CAMIE 当前本机输出目录：

```text
D:\WebstormProjects\portal-static\.local\camie-integration\site
```

CAMIE 生成后通过统一入口检查首页、列表、详情和搜索；不要双击生成的 HTML，也不要使用 `file://`。

## 六、在浏览器查看静态网站

下面命令中的 `8092` 是当前管理后台端口。如果你的后台使用其他端口，替换为实际端口。

### 查看 CAMIE

CAMIE 不需要再启动 `http-server`。第一节的本地同源网站窗口保持运行时，直接访问：

[http://127.0.0.1:18080/](http://127.0.0.1:18080/)

### 查看 CAAM

再打开一个 PowerShell 窗口：

```powershell
Set-Location D:\WebstormProjects\portal-static
npx --yes http-server .\dist\caam-live -p 8088 -c-1 -P http://127.0.0.1:8092
```

访问：[http://127.0.0.1:8088/](http://127.0.0.1:8088/)

### 查看 MIIC

再打开一个 PowerShell 窗口：

```powershell
Set-Location D:\WebstormProjects\portal-static
npx --yes http-server .\dist\miic-live -p 8089 -c-1 -P http://127.0.0.1:8092
```

访问：[http://127.0.0.1:8089/](http://127.0.0.1:8089/)

## 七、出现 `unauthorized` 怎么办

`unauthorized` 表示管理后台发送的 Token 与静态化服务要求的 Token 不一致。只检查下面三项：

### CAAM

```text
静态化服务：CAAM_STATIC_TOKEN=caam-portal-static-local-token
管理后台：  CAAM_STATIC_TOKEN=caam-portal-static-local-token
后台配置：  静态化程序访问令牌名=CAAM_STATIC_TOKEN
```

### MIIC

```text
静态化服务：MIIC_STATIC_TOKEN=miic-portal-static-local-token
管理后台：  MIIC_STATIC_TOKEN=miic-portal-static-local-token
后台配置：  静态化程序访问令牌名=MIIC_STATIC_TOKEN
```

### CAMIE

```text
静态化服务：CAMIE_STATIC_TOKEN 读取 .local/camie-integration/static-token
Portal 后端：CAMIE_STATIC_TOKEN 读取同一个 static-token
后台配置：  静态化程序访问令牌名=CAMIE_STATIC_TOKEN
```

修改环境变量后，静态化服务和管理后台都要停止并重新启动。

## 八、停止服务

在对应的 PowerShell 窗口中按：

```text
Ctrl + C
```

CAMIE 的五个窗口需要分别按 `Ctrl + C`。先停止本地同源网站和静态化服务，再停止 Portal、Member，最后停止 Redis。关闭 PowerShell 后，本窗口中设置的环境变量会自动失效，下次启动时重新执行本页命令即可。
