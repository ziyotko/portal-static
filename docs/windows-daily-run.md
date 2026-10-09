# Windows 日常运行（直接照着执行）

这份文档只解决一件事：环境已经安装好以后，如何启动 `portal-static`、管理后台和本地静态网站。

首次安装、数据库导入和生产部署不在这里说明。需要这些内容时再看 [Windows 完整操作手册](windows-quickstart.md)。

## 先看结论

- CAAM 静态化服务使用 `9142` 端口和 `caam_portal` 数据库。
- MIIC 静态化服务使用 `9143` 端口和 `miic_portal` 数据库。
- CAMIE 静态化服务使用 `9144` 端口和 `camie_portal` 数据库。
- 只测试 CAMIE 公开内容静态化时依次启动 Redis、Dia Portal 后端、Dia Portal 前端和 portal-static；查看生成站点时再开一个静态网站窗口。
- CAMIE 日常启动和 CAAM、MIIC 一样，直接从源码目录运行，不依赖 `.local` 中的程序或配置。
- CAAM 和 MIIC 可以同时启动，但必须分别占用一个 PowerShell 窗口。
- 每个窗口中的命令都要在同一个窗口内连续执行。
- 命令中的 `你的MySQL密码` 要替换为实际密码，不要在 `@` 或 `_` 前面添加反斜杠 `\`。

## 一、手动启动 CAMIE 静态化联调

以下命令与 CAAM、MIIC 一样，直接在源码目录执行。四个基础窗口都要保持打开；PowerShell 环境变量只对当前窗口及其子进程生效。

启动前先确认 `D:\WebstormProjects\dia-platform\business\portal\backend\config.yaml` 中的关键配置是：

```yaml
server:
  port: 8092

mysql:
  host: 127.0.0.1
  port: 3306
  user: root
  db_name: camie_portal

redis:
  addr: localhost:6379
  captcha_db: 6
  anti_replay_db: 7
  cache_db: 8
```

如果仍是 `db_name: caam_portal`，不要启动；先改成 `camie_portal`，否则后台会操作错数据库。

### CAMIE 窗口一：Redis

```powershell
Set-Location D:\Redis-8.0.0-Windows-x64-cygwin

.\redis-server.exe .\redis.conf
```

这就是 Dia 原来使用的 Redis：监听 `127.0.0.1:6379`，提供 16 个逻辑数据库，足够继续使用 Portal 的 DB `6/7/8`。看到 Redis 已准备接受连接后保持窗口打开。若提示 `6379` 已被占用，先确认是否已有可用的 Redis，不要重复启动第二个实例。

### CAMIE 窗口二：Portal 后端

```powershell
Set-Location D:\WebstormProjects\dia-platform\business\portal\backend

$env:PORTAL_DB_PASSWORD = '你的MySQL密码'
$env:PORTAL_JWT_SECRET = 'portal-local-jwt-secret-please-change-2026'
$env:CAMIE_STATIC_TOKEN = 'camie-portal-static-local-token'

go run .
```

Portal API 监听 `127.0.0.1:8092`。这个进程连接 `camie_portal`，并通过 Redis DB `6/7/8` 工作。

### CAMIE 窗口三：静态化服务

```powershell
Set-Location D:\WebstormProjects\portal-static

$env:CAMIE_DB_DSN = 'root:你的MySQL密码@tcp(127.0.0.1:3306)/camie_portal?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai'
$env:CAMIE_STATIC_TOKEN = 'camie-portal-static-local-token'

go run .\cmd\portal-static serve --config .\configs\camie.example.yaml
```

看到静态化服务监听 `127.0.0.1:9144` 后保持窗口打开。

健康检查地址：[http://127.0.0.1:9144/healthz](http://127.0.0.1:9144/healthz)

### CAMIE 窗口四：Dia Portal 前端

```powershell
Set-Location D:\WebstormProjects\dia-platform\business\portal\frontend

npm run dev
```

Dia Portal 前端默认监听 `3000`，并把 `/business_portal/api` 和公开上传请求代理到 `127.0.0.1:8092`。

Portal 后台：[http://127.0.0.1:3000/business_portal/](http://127.0.0.1:3000/business_portal/)

### CAMIE 后台静态化设置

登录 Dia 后台，进入“基础配置 → 静态化设置”，填写：

```text
静态化输出路径：D:/WebstormProjects/portal-static/dist/camie-portal
静态化程序访问地址：http://127.0.0.1:9144
静态化程序访问令牌名：CAMIE_STATIC_TOKEN
```

保存后进入“静态化管理”点击“生成全站”。Portal 后端与 portal-static 窗口中的 `CAMIE_STATIC_TOKEN` 必须完全相同。

### 可选窗口五：查看生成后的 CAMIE 网站

```powershell
Set-Location D:\WebstormProjects\portal-static

npx --yes http-server .\dist\camie-portal -p 8090 -c-1 -P http://127.0.0.1:8092
```

访问：[http://127.0.0.1:8090/](http://127.0.0.1:8090/)

只检查生成目录中的文件时不需要启动这个窗口。不要用 `file://` 双击 HTML。

### CAMIE 启动后检查

按顺序打开或执行：

```powershell
Invoke-RestMethod http://127.0.0.1:9144/healthz
Invoke-RestMethod http://127.0.0.1:8092/business_portal/api/site-info
```

两个请求都成功后，即可登录 Portal 后台进行公开内容维护和全站生成。测试这条公开静态化链路不需要启动 Member 后端。

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

本节命令用于 CAAM、MIIC。CAMIE 的 Portal 后端和前端已在第一节启动，不要再运行一份占用相同端口的后端。

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

### CAMIE

```text
静态化输出路径：D:/WebstormProjects/portal-static/dist/camie-portal
静态化程序访问地址：http://127.0.0.1:9144
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

CAMIE 输出目录：

```text
D:\WebstormProjects\portal-static\dist\camie-portal
```

CAMIE 生成后通过第一节的 `8090` 静态网站检查首页、列表、详情和搜索；不要双击生成的 HTML，也不要使用 `file://`。

## 六、在浏览器查看静态网站

下面命令中的 `8092` 是当前管理后台端口。如果你的后台使用其他端口，替换为实际端口。

### 查看 CAMIE

如果第一节的 CAMIE 静态网站窗口尚未启动，执行：

```powershell
Set-Location D:\WebstormProjects\portal-static
npx --yes http-server .\dist\camie-portal -p 8090 -c-1 -P http://127.0.0.1:8092
```

访问：[http://127.0.0.1:8090/](http://127.0.0.1:8090/)

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
静态化服务：CAMIE_STATIC_TOKEN=camie-portal-static-local-token
Portal 后端：CAMIE_STATIC_TOKEN=camie-portal-static-local-token
后台配置：  静态化程序访问令牌名=CAMIE_STATIC_TOKEN
```

修改环境变量后，静态化服务和管理后台都要停止并重新启动。

## 八、停止服务

在对应的 PowerShell 窗口中按：

```text
Ctrl + C
```

CAMIE 的四个基础窗口需要分别按 `Ctrl + C`。如果启动了查看站点的 `http-server`，也在该窗口按 `Ctrl + C`。建议先停止静态网站、Dia 前端和静态化服务，再停止 Portal 后端，最后停止 Redis。关闭 PowerShell 后，本窗口中设置的环境变量会自动失效，下次启动时重新执行本页命令即可。
