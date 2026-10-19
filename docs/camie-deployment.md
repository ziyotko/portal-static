# CAMIE 门户部署与运维交接

> 更新：2026-10-19。本文是交接准备，不表示已经部署。实施时使用目标环境的域名、目录、账号和密钥；不要把本机测试库、测试账号或 `.local/` 配置原样搬到生产环境。页面和栏目逐项对照见 [CAMIE 页面区块与栏目维护对照表](column-mapping/camie-site-areas.md)。

生产服务器不承担构建。统一在受控的 Windows 构建机运行 `scripts/build-camie-release.ps1 -Version <版本>`，生成 Linux amd64 的版本化 `.tar.gz` 和独立 SHA256 文件；服务器只校验、解压、配置、迁移和启动。构建脚本要求两个仓库的当前 commit 都带同名 `camie-release-<版本>` tag，并拒绝未提交的发布源码。发布包模板、systemd、Nginx、SQL 与操作说明位于 `deploy/camie/`。

## 1. 系统边界和上线前置项

| 组件 | 数据库 | 作用 |
| --- | --- | --- |
| `dia-platform/business/portal` 后台和 API | `camie_portal` | 公开栏目、文章、模板、会员专区的 `member_column` / `member_content`；公开上传和私有文件 |
| `dia-platform/business/member` 会员系统 | `camie_member` | 注册、登录、会员资料、会籍状态；签发 `member-token` |
| `portal-static` 的 CAMIE 进程 | **只读** `camie_portal` | 根据公开内容和所配置的模板来源生成静态文件；不连接 `camie_member` |
| 对外 Web 服务 | 无 | 同一域名下托管静态站、两套前端和两套 API |

两库应分别备份和恢复；`camie_portal` 的会员专区内容表属于门户库，不能误移到 `camie_member`。门户后台与静态化程序共用门户库，但应使用各自最小权限账号。会员系统使用独立账号连接会员库。数据库之外，还须备份门户公开上传、会员专区私有上传、会员系统上传、数据库模板源码、运行配置和静态产物当前版本；密钥单独安全保存。

**已接受的上线风险：**当前 CAMIE 页面先请求 `/business_member/api/member/profile`，只让 `status=active` 的账号继续加载会员内容；但现有门户会员专区 API 的服务端中间件只验证会员 JWT 和登出黑名单，没有实时复核 `active`。持有效 Token 的非正式会员仍可能直接调用列表、详情和文件签名接口。此次上线允许带着该风险交付，但验收记录必须如实保留，且不能描述为“服务端已校验会员状态”；后续应由后端或网关补上实时状态校验。

## 2. 版本、数据和目录准备

1. 固定同一轮联调验收的 `portal-static`、Portal、Member 代码版本和数据库结构；按 `dia-platform/business/portal/DEPLOY.md` 与 `dia-platform/business/member/DEPLOY.md` 完成数据库迁移与服务配置。旧 SQL 如仍含 `page_id` / `page`，先按门户部署说明核对并迁移，不能直接与当前按 `column.template_id` 取数的结构混用。
2. 在 `camie_portal` 核对七条顶部页面模板 `camie-home`、`camie-party`、`camie-ministry`、`camie-news`、`camie-training`、`camie-standards`、`camie-about`，以及 `camie-list`、`camie-article`、`camie-layout`。模板须启用、有正确 `type` 和 `code`；栏目绑定到正确模板，稳定 `column.code` 不应因改名而改变。当前缺省 `file` 模式使用发布包内 `.tmpl` 渲染，数据库仍提供栏目、文章和模板归属；切到 `database` 时还需确认这些模板的 `source_code` 非空且正确。
3. 规划只读资源目录、独立静态产物目录、预览目录和回滚版本目录。`paths.source_root` 指向随本版本发布的 CAMIE `site` 资源；`paths.dist_root` 是默认产物目录，`paths.allowed_output_root` 是 Dia 可选路径的安全边界，`paths.preview_root` 是预览目录。所有输出目录均不得与源码目录重叠。服务账号可读资源与配置，可写产物目录及其父目录中的临时发布目录。
4. 先备份两库、上述上传目录、当前线上静态产物和数据库模板；校验备份可恢复。不要在生成目录中手工改 HTML、JS、CSS；重建会覆盖这些改动。

发布包应完整解压，不能只更新可执行文件。以 `/opt/camie/current` 为当前版本目录时，静态资源和文件模板应为以下结构：

```text
/opt/camie/current/share/
├── camie-site/
│   ├── assets/
│   ├── css/
│   ├── js/
│   └── column-structure.json
└── camie-preview-templates/
    ├── layout.html.tmpl
    ├── home.html.tmpl
    ├── list.html.tmpl
    ├── article.html.tmpl
    └── about.html.tmpl
```

`paths.source_root: /opt/camie/current/share/camie-site` 表示该目录下直接是 `assets`、`css`、`js` 等内容，不要再嵌套一层 `site` 或 `camie-site`。完整的正确、错误示例和上线前检查命令见 [CAMIE 生产部署研发交接说明](../deploy/camie/README.md#41-静态资源和模板放置位置)。

发布流水线应在切换版本前自动调用发布包内 `ops/scripts/apply-release-db.sh <发布目录> <本次备份目录>`。脚本会完成备份、`preflight.sql` 检查和 `migrate.sql` 执行；任何 `BLOCKER` 或 SQL 错误都会让该步骤失败并阻止继续发布。不要让运维分别手动运行两份 SQL。生产流水线由运维维护，接入命令和所需的 MySQL login path 见 [发布说明](../deploy/camie/README.md)。

## 3. 静态化配置与启动

以 [CAMIE 示例配置](../configs/camie.example.yaml) 为起点，另存为环境专用 YAML，至少核对：

| 配置 | 生产口径 |
| --- | --- |
| `driver` / `site.id` | `camie` / `camie` |
| `database.dsn_env` / `database.schema` | 例如 `CAMIE_DB_DSN` / `camie_portal`；DSN 仅通过受保护环境变量注入 |
| `server.addr` / `server.token_env` | 内网或回环监听；例如 `127.0.0.1:9144` / `CAMIE_STATIC_TOKEN` |
| `paths.source_root` | 与构建版本匹配的只读 CAMIE 资源目录 |
| `paths.dist_root` / `paths.allowed_output_root` / `paths.preview_root` | 默认正式目录、Dia 可选路径安全根、预览目录；生产若只允许一个目录，可让前两者相同 |
| `paths.template_codes` | 与上述十条数据库模板的稳定 code 一致 |
| `adapter.site.template_source` | 缺省为 `file`，现有配置无需加该行；从同版本发布包的 `share/camie-preview-templates/*.tmpl` 渲染。今后由运维维护数据库模板时显式改为 `database`，从 `template.source_code` 渲染，并重启静态化进程、生成全站 |
| `adapter.site.page_name` / `hero_column` | `环保机械协会` / `home-hero`；轮播只读取“首页 → 轮播图”的独立投放栏目，旧值 `news-hot` 会导致启动校验失败。修改后需重新生成全站并核对首页轮播链接的 `from=home-hero` |
| `adapter.site.member_login_path` / `member_register_path` | 同源根路径 `/business_member/login`、`/business_member/register`；修改后重建全站 |
| `media` | 上传路径按实际反代或对象存储地址配置；生成后检查封面、附件、公开视频均可访问 |

构建前在仓库根目录执行 `go test ./...`、`go vet ./...`，再构建二进制。`portal-static preview --config <CAMIE配置>` 使用**内置演示数据**，可检查页面结构与资源，但不能证明目标库的栏目和文章正确；目标库验收应将 `dist_root` 指向隔离目录后执行 `portal-static generate --config <隔离配置>`。核对完产物，再用正式配置启动一站一进程的静态化服务。例如：

`file` 模式下只改 `.tmpl` 无须先更新数据库 `source_code` 才能看到页面变化，但仍须发布同版本程序和模板资源并重建页面。发布包的 `db/migrate.sql` 目前仍会写入模板源码及栏目归属等数据；有数据库变更的版本必须按发布流水线执行。切到 `database` 模式前，应核对数据库模板已是目标版本；切换后发布脚本仍可能覆盖运维在后台改过的 `source_code`，届时要调整迁移策略。

**分支机构验收：**后台保留“关于协会 `about` → 分支机构 `about-branches` → 九个 `branch-*`”结构。每个旧 `branch-*` 栏目首篇已发布图文是分会简介，页面沿用“协会简介”的正文模板，只显示首篇正文；“关于协会”中的“分支机构”默认选中水分会。首页九张卡片的外观和顺序不变，但链接进入 82 `home-fenzhijigou` 下各 `main-branch-*` 的专用文章列表，首页“分支机构”标题默认进入水分会列表。直接打开 `list/<about-branches 栏目 ID>/1.html` 显示水分会简介，不显示九宫格。发布时一起更新生成器、`camie-home`、`camie-about` 和 `camie-layout` 模板；若使用 `database` 模式，须先将对应数据库模板源码更新到同版并在隔离目录生成核对。检查九张卡片、九个列表页、九个简介页、默认选中状态、面包屑及链接校验。

**多栏目文章详情验收：**同一文章若同时投放到“关于协会”和首页分会列表或专家视野，详情 URL 的 `?from=<栏目 code>` 决定侧栏、面包屑及上一篇/下一篇的栏目上下文。发布包须同时包含新版生成器、`camie-article` 模板和 `js/main.js`，随后执行**全站生成**；只替换其中一项或只更新列表页不会修复已生成的详情 HTML。用首页分会列表的 `?from=main-branch-*`、专家视野的 `?from=about-expert-insights` 分别打开详情，核对侧栏属于对应栏目，并确认页面的 `data-generated-at` 已更新。旧 `branch-*` 属于“关于协会”的简介栏目，不用作首页专用列表。

2026-10-19 的栏目导出中，82 `home-fenzhijigou` 位于 65 `home` 下，84、85、87–93 `main-branch-*` 位于 82 下。旧生成器加载子栏目却跳过 82 列表，造成 `list/82/1.html` 断链；本版生成 82 及子栏目的列表页，并在导航中将它们视为独立的分支机构栏目。运维保留这套新栏目，在 `main-branch-*` 发布需要展示在首页入口后的动态文章；旧 `branch-*` 只维护简介，不必将同一篇简介重复投放。不要直接删除或修改生产库。其他 `main-*` 栏目须按其自身用途单独核对。

**会议活动及信息公开：**在“交流培训 → 会议活动”下新增 `training-meetings-conference`（环保装备高质量发展大会）和 `training-meetings-other`（其他活动）；在“关于协会”下新增 `about-disclosure`（信息公开）。三个新栏目都使用普通文章列表。二、三级栏目只显示直接发布到自身的文章：父级“会议活动”已有文章继续留在父级，两个子栏目各自维护文章，不自动上卷或合并；需要跨栏目出现的文章应在后台分别投放。首页“会议活动”资讯标签继续读取父栏目，环保装备高质量发展大会横幅进入同名子栏目。修改栏目或文章后重新生成全站，核对三个新列表及父子文章互不混入。

### 三个新栏目的运维数据配置

正式库须由运维在 Dia“栏目管理”创建；本地测试库中的 ID 99、100、101 仅供联调，**不要在正式库固定使用这些 ID，也不要导入 `.local/` 中的测试文章**。本次发布包的 `migrate.sql` 会按栏目树统一校正模板归属，但不会新建这三条栏目。后台如果不能设置稳定 `code` 或模板归属，应由运维在备份、审核和回滚准备完成后执行等价的受控数据迁移；静态化服务本身只读数据库。

| 新栏目名称 | 稳定 `column.code` | 父栏目 `code` | 模板 `code` | 同级排序 | 展示方式 |
| --- | --- | --- | --- | ---: | --- |
| 环保装备高质量发展大会 | `training-meetings-conference` | `training-meetings` | `camie-training` | 0 | 普通文章列表 |
| 其他活动 | `training-meetings-other` | `training-meetings` | `camie-training` | 1 | 普通文章列表 |
| 信息公开 | `about-disclosure` | `about` | `camie-about` | 放在分支机构后 | 普通文章列表 |

三个栏目均须启用（`status=1`），本地库的普通列表 `display_type=1`。父栏目 `training-meetings`、`about` 的编码和现有文章不变。运维创建后按 `code`、`parent_id`、`template_id`、排序及启用状态复核，再把文章直接投放到目标栏目并完成审核发布；只调整栏目层级不会迁移文章的 `article_column` / `article_column_publish` 关系。先发布匹配的生成器与模板、准备栏目数据，随后生成全站；核对首页大会横幅、首页“会议活动”资讯、会议活动父列表、两个子列表、“关于协会 → 信息公开”侧栏和列表，以及文章详情、面包屑。`list/<栏目ID>/1.html` 的 ID 以正式库实测为准。

运维可在目标库执行以下**只读**查询，将结果附在上线验收记录中；三行都应返回，并与上表的父级、模板、排序和状态一致：

```sql
SELECT c.id, c.name, c.code, p.code AS parent_code,
       t.code AS template_code, c.sort, c.status, c.display_type
FROM `column` AS c
JOIN `column` AS p ON p.id = c.parent_id
JOIN template AS t ON t.id = c.template_id
WHERE c.code IN ('training-meetings-conference',
                 'training-meetings-other', 'about-disclosure')
ORDER BY p.code, c.sort;
```

运维可在目标库执行以下**只读**查询，将结果附在上线验收记录中；三行都应返回，并与上表的父级、模板、排序和状态一致：

```sql
SELECT c.id, c.name, c.code, p.code AS parent_code,
       t.code AS template_code, c.sort, c.status, c.display_type
FROM `column` AS c
JOIN `column` AS p ON p.id = c.parent_id
JOIN template AS t ON t.id = c.template_id
WHERE c.code IN ('training-meetings-conference',
                 'training-meetings-other', 'about-disclosure')
ORDER BY p.code, c.sort;
```

```bash
/opt/portal-static/current/portal-static generate --config /etc/portal-static/camie-staging.yaml
/opt/portal-static/current/portal-static serve --config /etc/portal-static/camie.yaml
curl --fail http://127.0.0.1:9144/healthz
```

上述命令需要先通过受保护的环境文件注入 `CAMIE_DB_DSN`、`CAMIE_STATIC_TOKEN`；`camie-staging.yaml` 的输出目录须与正式目录不同。长期运行宜使用独立的 systemd unit，按 [通用部署指南](deployment.md) 把站点名、YAML、环境文件、端口、读写目录改成 CAMIE。启动后从 Dia “静态化管理”执行全站生成并确认任务成功。

Dia 门户后台“静态化设置”的**输出路径**优先于 `dist_root`，但必须等于 `allowed_output_root` 或位于其下级；Dia 未配置或请求未携带路径时使用 `dist_root`。**访问地址**必须从后台进程可达，**访问令牌名**填 `CAMIE_STATIC_TOKEN` 之类的环境变量名；Portal 与静态化进程须注入同名同值的令牌。令牌值、数据库口令和 JWT 密钥不得写入 Git 或截图。

## 4. 同域名路由

浏览器必须通过 HTTP(S) 访问生成站点。`file://` 不能正常访问绝对路径的登录、注册、API 或上传文件。对外域名须满足下表，反向代理转发时保留这些请求路径：

| 公网路径 | 目标 |
| --- | --- |
| `/`、`/index.html`、`/pages/`、`/list/`、`/article/`、`/js/`、`/css/`、`/assets/` 等 | `paths.dist_root` 静态产物 |
| `/business_portal/` 管理页面 | Portal 前端构建产物 |
| `/business_portal/api/` | Portal 后端 API，包括 `/member-zone/...` 和私有文件签名/播放请求 |
| `/business_portal/uploads/` | Portal 公开上传；可由 Portal 后端或单独的公开上传目录提供 |
| `/business_member/` 登录、注册、会员中心 | Member 前端构建产物 |
| `/business_member/api/` | Member 后端 API，包括 `/member/profile` |

`/business_portal/private_uploads/` **不得**作为公开静态目录。会员私有图片、附件和视频须经过 Portal 的签名接口；视频 Range 请求和签名 URL 的五分钟有效期需在代理层保持可用。Portal 配置的 `PORTAL_MEMBER_JWT_SECRET` 必须与 Member 的 `MEMBER_JWT_SECRET` 相同；Portal 的 `redis.member_token_db` 必须指向 Member 登出黑名单所在 Redis DB。两套前端及 API 均应走同一对外域名，避免登录回跳和浏览器同源限制出错。

## 5. 上线验收与日常重建

| 检查 | 通过标准 |
| --- | --- |
| 首页与公开栏目 | 七个导航页面、对应栏目列表和公开文章详情可打开；首页文章详情左侧显示其正式栏目，不显示“首页内容”投放栏目 |
| 公开内容增删改 | 发布后首页/列表/详情/搜索同步；删除或下线后旧详情不再存在，搜索索引不再出现标题 |
| 公开与私有边界 | 生成 HTML 与 `generated-content.js` 不含会员正文、私有物理路径或完整签名 URL；私有上传不能匿名直取 |
| 会员页面 | 未登录跳同源登录并带 `returnUrl`；登录后返回原 CAMIE 页面；`active` 可看列表、四类详情和视频；非 `active` 页面会拒绝，API 直调限制为本次已接受风险 |
| 媒体 | 公开视频直接播放；私有视频能拖动、Range 播放，长视频在签名续期后尽量保持进度 |
| 端点和设备 | 桌面、手机页面布局及 `/healthz`、Portal/Member API 正常；无混合内容和上传文件 404 |

公开文章的发布、删除触发静态化属于尽力而为：后台业务操作成功，不代表静态文件已经同步。值班时应监控静态化任务/失败日志、产物更新时间和关键页面；失败后修复服务或配置，再从后台手动“生成全站”，复核旧详情、列表、首页和搜索。会员内容由浏览器实时读取，发布或下线后刷新页面即可核对，不靠公开静态化重建。

回滚时先暂停发布，恢复上一版二进制、配置、资源和数据库模板；必要时恢复上版静态产物。若伴随数据库迁移，按两库各自的迁移回滚方案执行，不能只切换静态目录。完成后再做公开文章和会员 API 的最小验收。
