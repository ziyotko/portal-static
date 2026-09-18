# Windows 日常运行（直接照着执行）

这份文档只解决一件事：环境已经安装好以后，如何启动 `portal-static`、管理后台和本地静态网站。

首次安装、数据库导入和生产部署不在这里说明。需要这些内容时再看 [Windows 完整操作手册](windows-quickstart.md)。

## 先看结论

- CAAM 静态化服务使用 `9142` 端口和 `caam_portal` 数据库。
- MIIC 静态化服务使用 `9143` 端口和 `miic_portal` 数据库。
- CAAM 和 MIIC 可以同时启动，但必须分别占用一个 PowerShell 窗口。
- 每个窗口中的命令都要在同一个窗口内连续执行。
- 命令中的 `你的MySQL密码` 要替换为实际密码，不要在 `@` 或 `_` 前面添加反斜杠 `\`。

## 一、启动静态化服务

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

## 二、启动管理后台

管理后台必须在启动前拿到与静态化服务相同的 Token。

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

## 三、在管理后台填写静态化配置

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

注意：

- “令牌名”填写 `CAAM_STATIC_TOKEN` 或 `MIIC_STATIC_TOKEN`。
- 不要填写 `$env:`。
- 不要把实际令牌值 `caam-portal-static-local-token` 或 `miic-portal-static-local-token` 填到“令牌名”中。
- Windows 输出路径建议使用 `/`，不要只填写相对路径。

## 四、生成全站

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

## 五、在浏览器查看静态网站

下面命令中的 `8092` 是当前管理后台端口。如果你的后台使用其他端口，替换为实际端口。

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

## 六、出现 `unauthorized` 怎么办

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

修改环境变量后，静态化服务和管理后台都要停止并重新启动。

## 七、停止服务

在对应的 PowerShell 窗口中按：

```text
Ctrl + C
```

关闭 PowerShell 后，本窗口中设置的环境变量会自动失效，下次启动时重新执行本页命令即可。
