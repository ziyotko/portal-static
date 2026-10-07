# CAMIE 门户页面区块与栏目维护对照表

> 核对时间：2026-10-02。依据当前 CAMIE 生成器、十份 `camie-*` 数据库模板，以及本机 `camie_portal` 测试库。这里的本机栏目 ID 只是测试数据，正式库应按稳定的 `column.code` 核对，不能照抄 ID。尚未部署正式站。

Dia 把模板作为页面，栏目直接通过 `column.template_id` 归属模板。本机已把顶部七个导航页面分别设为七条启用的“首页”类型模板；在“栏目管理 → 首页”依次点“首页”“党建专栏”等模板，就能分别看到各自栏目树。`camie-list` 是通用栏目页渲染模板，`camie-article` 是通用详情渲染模板，`camie-layout` 是公共布局。原 CAMIE 的 64 个分类节点仍完整保留；另有 12 个首页专用节点。

| 顶部页面模板 | 稳定编码 | 本机栏目节点数 | 生成页面 |
| --- | --- | ---: | --- |
| 首页 | `camie-home` | 12 | `index.html` |
| 党建专栏 | `camie-party` | 4 | `pages/party.html` 及其子栏目 |
| 部委动态 | `camie-ministry` | 8 | `pages/ministry.html` 及其子栏目 |
| 新闻中心 | `camie-news` | 5 | `pages/news.html` 及其子栏目 |
| 交流培训 | `camie-training` | 9 | `pages/training.html` 及其子栏目 |
| 科技标准 | `camie-standards` | 8 | `pages/standards.html` 及其子栏目 |
| 关于协会 | `camie-about` | 16 | `pages/about.html` 及其子栏目 |

视频专区、政策研究和原会员栏目结构仍属于通用 `camie-list` 模板下的 14 个节点；真正的会员内容取自独立的 `member_column`/`member_content`，不会因为此分组改变。

## CAAM 后台的页面与栏目参照

本机 CAAM 测试库仍使用 `template → page → column`：`page.template_id` 绑定模板，`column.page_id` 绑定页面。其“模板管理”点击“应用页面数”只列关联页面；到“栏目管理”选择一个页面后，才显示该页面的栏目树。示例：`首页`模板 → `首页`页面 → 21 个栏目，`协会概况`模板 → `协会概况`页面 → 12 个栏目。通用栏目、通用详情、专题页面在该测试库中没有栏目。

Dia 后台已于 2026-09-21 将页面层合并进模板，使用 `column.template_id`。因此当前 CAMIE 是“模板（即页面）→ 栏目”，与 CAAM 测试库的“模板 → 页面 → 栏目”数据结构不同。本机用允许多个同时启用的“首页”类型承载七个顶部页面，保留唯一启用的通用“栏目页”类型模板；页面源码按稳定模板编码由 CAMIE 生成器调用。若以后要恢复 CAAM 式独立页面表，仍需调整 Dia 表结构、接口、前端投放流程与静态化取数。上述 CAAM 数量来自本机测试库，用于理解维护关系，不作为正式库的数据基准。

## 先看懂“死”和“活”

| 本文标记 | 实际含义 | 修改后何时可见 |
| --- | --- | --- |
| **静态活** | 内容由后台 `column`、`article`、栏目发布关系取出，但被写入公开 HTML 和搜索索引；浏览器访问时不再查后台 | 文章审核发布/下线/删除通常会触发静态化；改栏目、模板、图片或链接后应手动生成全站并核对任务结果 |
| **实时活** | 页面 HTML 只是空壳，浏览器打开后向会员接口查当前状态和内容 | 后台会员内容发布或下线后，刷新会员页面即可看到变化 |
| **固定（“死”）** | 名称、顺序、图片、外链或样式写在模板、Go 配置或静态资源中；后台新增文章不会改变这块固定信息 | 修改对应源码/数据库模板或资源，再生成全站；代码与资源变更还要经过后续部署 |
| **占位/未接入** | 看起来是入口或配置，但当前没有对应功能，或后台表没有被 CAMIE 生成器消费 | 需要单独开发接入，不能靠录入栏目/文章解决 |

**关键区别：**“静态活”不是实时页面。公开文章要同时满足栏目和模板启用、类型为图文 `1` 或视频 `2`、已关联目标栏目、审核通过、已发布且发布时间不晚于当前时间，才进入生成结果。会员内容走另一组表和接口，不能靠在公开 `column` 下投放文章来维护。

## 首页 `index.html`

| 页面位置 | 对应栏目 / 数据 | 哪些活、哪些固定 | 维护位置 |
| --- | --- | --- | --- |
| 顶部重点新闻轮播 | `home-hero`，最多 3 篇 | **静态活**：标题、封面、发布时间、来源、详情链接。**固定**：轮播位置与切换样式；无内容时没有自动补位 | 后台“栏目管理 → 环保机械协会 → 首页内容 → 重点新闻轮播”及“图文管理”维护；封面在文章里维护 |
| 轮播右侧三组资讯标签 | 依次为 `home-news-notice`、`home-news-association`、`home-news-member`，每组最多 6 篇 | **静态活**：标签名取首页专用栏目名，列表取对应栏目文章。**固定**：只有这三组、顺序和每组数量上限 | 后台选“首页”模板，在对应首页子栏目投放；增减标签需改生成器 |
| 快捷栏“申请入会” | `/business_member/register` | **固定入口**，注册流程属于会员系统 | 模板 `camie-home`；会员系统维护注册流程 |
| 快捷栏“会员中心” | `pages/member.html` | **固定入口 → 实时活页面** | 模板 `camie-home`；会员栏目与内容见下文 |
| 快捷栏“移动媒体” | 三张微信二维码图片 | **固定**图片及文字，不从后台二维码、广告或链接表取数 | `assets/images/qr-wechat-service.jpg`、`qr-wechat-video.png`、`qr-wechat-subscription.jpg` 与首页模板 |
| 快捷栏“视频专区” | `videos.html`，聚合公开视频 | **固定入口**，目标列表是**静态活** | 首页模板固定入口；公开视频文章维护见下文 |
| 中部六组资讯标签 | 依次为 `home-training-meetings`、`home-standards-work`、`home-standards-innovation`、`home-training-international`、`home-training-talent`、`home-policy-reports`，每组最多 6 篇 | **静态活**：标签名、标题、日期、详情链接。**固定**：六组顺序及每组右侧配图 | 后台选“首页”模板，在对应首页子栏目投放；图片映射在 CAMIE 生成器中 |
| 大会横幅 | 点击指向 `training-meetings` | **固定**：横幅图 `assets/images/conference-banner.png`；**静态活**：点击目标按栏目 code 找到生成页。当前不读取后台“广告管理” | 首页模板及图片资源；栏目内容在“会议活动”维护 |
| 横幅下四个专题入口 | `standards-green-promotion`、`policy-major-equipment-catalogue`、`policy-qualified-enterprises`、`policy-innovation-tasks` | **固定**：四个标题和顺序；**静态活**：链接指向各栏目生成页。当前不读取后台“友情链接管理” | 首页模板改标题/数量；后台栏目与文章改目标内容 |
| “分支机构”九张卡片 | `branch-water`、`branch-atmosphere`、`branch-solid-waste`、`branch-monitoring`、`branch-noise`、`branch-uv`、`branch-ozone`、`branch-ai`、`branch-engineering` | **固定**：九个名称、顺序和图标；**静态活**：每张卡片指向各自栏目。后台新增第十个分支栏目不会自动长出首页卡片 | CAMIE 生成器中的 `Branches` 改卡片；后台栏目与文章改分支内容 |
| “专家委员会” | 标题到 `about-experts`；内容取 `home-experts` 最多 3 篇 | **静态活**：首篇标题/链接及其余标题。**固定**：首篇配图 `assets/images/expert-photo.png` 和“中国环保机械行业协会／专家委员会”字样，未使用文章封面 | 后台选“首页”模板，在“首页内容 → 专家委员会”投放；图片、固定字样在首页模板 |
| “副会长单位”滚动标志 | Go 生成器的 `PartnerRows` 数组 | **固定**：单位名称、标志文件、官网 URL、排序均写在代码里；**未接入**后台“友情链接管理” | CAMIE 生成器 `PartnerRows` 与 `assets/images/partner-logos/` |

首页标签的“查看更多”效果由点击标签直接进入对应首页专用栏目列表实现；悬停/聚焦只切换当前展示的列表。首页专用栏目的文章与公开分类栏目的文章可为同一篇，需在后台分别投放到两个栏目。旧数据若尚未建立 `home-*` 栏目，生成器会临时回退到原分类栏目；一旦建好首页专用栏目，即使为空也不会回退。

## 全站公共区域

| 页面位置 | 对应栏目 / 数据 | 当前状态 | 维护位置 |
| --- | --- | --- | --- |
| Logo、协会名称、标语 | `brand-logo.png`、`brand-slogan.png` | **固定**图片和站名；不读后台“站点名称/Logo”设置 | `camie-layout` 与图片资源 |
| 顶部七个导航入口 | 首页，以及 `party`、`ministry`、`news`、`training`、`standards`、`about` | **固定**显示名称与顺序；**静态活**链接按栏目 code 定位 | `camie-layout`；七个页面模板需启用，对应栏目 code 需稳定 |
| “政策研究” | `policy-research` | **静态活**栏目页，归在“部委动态”的导航高亮下；顶部没有独立主导航按钮 | 后台栏目与文章；入口主要来自专题/其他链接 |
| 栏目页左侧菜单 | 当前顶级栏目的启用子栏目；二、三级栏目按层级展示 | **静态活**，随栏目树生成；视频栏目另外追加“会员专享”入口 | 后台“栏目管理”维护名称、层级、排序、启用状态；修改后重建 |
| 面包屑 | 栏目树、当前栏目、详情标题 | **静态活**。公开详情最终是文章标题；会员详情见下文 | 后台栏目名及文章标题；页面模板控制结构 |
| 顶部与搜索页搜索 | `search.html`、公开文章快照、`generated-content.js` | **静态活**：仅搜索已生成的公开文章标题/摘要等文本，浏览器本地筛选；**不实时查后台**，不含会员内容 | 发布/下线公开文章并确认静态化成功 |
| `中 | EN` | 无英文栏目或英文站点 | **占位/未接入**：点击仅提示“英文版内容将在后续阶段接入” | 需要另做英文站接入 |
| 页脚“友情链接” | 模板内固定的外站 URL | **固定**，不读取后台“友情链接管理” | `camie-layout` 模板 |
| 页脚版权、备案文字 | 模板内固定文字 | **固定**，不读取后台基础设置里的版权/ICP备案号 | `camie-layout` 模板 |
| 返回顶部、移动导航、轮播切换 | `js/main.js` | **固定交互**，与栏目数据无关 | 前端脚本、CSS |

## 公开列表、详情与视频

| 页面 | 栏目/内容来源 | 维护口径 |
| --- | --- | --- |
| `pages/party.html`、`pages/ministry.html`、`pages/news.html`、`pages/training.html`、`pages/standards.html`、`pages/about.html` | 分别对应 `party`、`ministry`、`news`、`training`、`standards`、`about`；也是 `list/<栏目ID>/1.html` 的别名入口 | **静态活**。顶级栏目列表聚合其公开子孙栏目文章，左侧菜单按启用栏目树生成 |
| `pages/policy.html`、`pages/experts.html`、`pages/videos.html` | 分别对应 `policy-research`、`about-experts`、`videos` | **静态活**栏目别名。`pages/videos.html` 与根目录 `videos.html` 均为公开视频入口 |
| 其余栏目列表 `list/<栏目ID>/<页码>.html` | 对应每个启用公开栏目的 `column.code`；本机 ID 仅为示例 | **静态活**：10 条/页；文章置顶优先、发布时间倒序。栏目改名不要求改 code |
| “关于协会/协会简介”内容区 | `about` 页优先取 `about-introduction` 的首篇公开图文；`about-introduction` 自身也有简介页 | **静态活**正文；没有文章时显示“暂无公开内容” |
| 公开文章详情 `article/YYYY/MM/<文章ID>.html` | 后台公开 `article` + 栏目发布关系 + 附件 | **静态活**：标题、日期、来源、正文、附件、上一篇/下一篇；公开富文本会在生成时净化。文章若设置有效的 HTTP(S) 外链 `url`，列表直接跳外链，不生成该文章的本地详情 |
| `videos.html`、`pages/videos.html` 与视频详情 | 公开类型 `2` 的视频文章；`videos-news` 的栏目列表也使用视频卡片样式 | **静态活**：在“图文管理”新增视频并投放到“CAMIE栏目 / 视频资讯”（栏目展示方式须为“视频展示”）；详情优先播放文章的 MP4 附件，也兼容正文中的视频标签。没有视频源时显示提示，不显示假的播放按钮 |
| 视频侧栏“会员专享” | `pages/member.html?column=会员专享&mode=video` | **固定入口 → 实时活内容**；不把私有视频生成成公开详情 |
| `detail.html`、`video-detail.html` | 各自取一篇公开图文/视频的兼容别名 | **静态活**兼容页；具体文章以生成时内容为准，不能作为固定文章地址对外长期引用 |

公开生成器只接收公开 `article` 类型 `1` 图文、`2` 视频。后台公开 `article` 类型 `3` 数据、`4` 报刊即使投在公开栏目，也不会变成 CAMIE 公开列表/详情；会员专区能显示数据和报刊，但它读取的是另一张 `member_content` 表，不会自动复用公开文章。

## 会员专区：和公开栏目树是两套数据

| 页面/区块 | 实际数据源 | 状态与维护方式 |
| --- | --- | --- |
| `pages/member.html` 会员列表 | `member_column` + `member_content`；默认“行业报告”，也可用 `?column=栏目名称` 或栏目 ID | **实时活**。先读 `localStorage["member-token"]`，再查 `/business_member/api/member/profile`；仅 `status=active` 才请求列表。栏目侧栏由启用的会员栏目实时生成，分页与标题也来自接口 |
| `pages/member-detail.html?id=...` | `/business_portal/api/member-zone/member-contents/detail/:id` | **实时活**。新闻显示正文/附件；数据显示年份、单位、省份、地区等；视频显示封面和完整视频；报刊显示摘要/封面/文件。面包屑最终是详情标题 |
| 私有图片、附件、视频 | 干净文件名 + `/member-zone/member-files/sign?name=...` | **实时活**。浏览器在使用前取五分钟临时地址；视频播放中会续签并尝试恢复进度。静态 HTML 和公开搜索索引不存会员正文、私有文件地址或签名 URL |
| 未登录、失效、非正式会员 | 会员资料接口 | **实时活**。未登录可跳登录并带 `returnUrl` 返回原 CAMIE 页；前端只在 `active` 时请求会员内容。当前 Dia 服务端仅校验会员 Token 和登出状态，未校验 `active`；持有效 Token 的非正式会员仍可能直接调用列表、详情和文件签名接口 |

本机 `member_column` 启用项为 **行业报告、数据中心、电子刊物、会员专享**，对应后台“会员专区”维护。公开 `column` 树里的 `member`、`member-reports`、`member-data`、`member-publications`、`member-space`、`videos-members` 是原栏目结构/访问边界，**不用于会员列表取数**；尤其“我的空间”目前没有对应的 `member_column`，不是一个可通过投放公开文章启用的页面。

**上线阻断项：**如果要求只有 `active` 正式会员能读取私有内容，需要在会员内容 API 服务端或其上游网关执行实时身份校验。当前仅靠 CAMIE 页面中的状态判断，无法阻止持有效 Token 的非 `active` 用户直接请求 API。此项不涉及公开静态化程序；在服务端校验完成前，不能把“非正式会员无法读取会员内容”列为已通过验收。

## 本机完整栏目树（按 `column.code` 对照）

下列是本机测试库与 CAMIE 栏目结构文件核对后的 9 个原始一级栏目、64 个分类节点。顶部六个公开栏目分支分别归属上表六条页面模板；视频专区、政策研究及原会员栏目结构归 `camie-list`。首页另有 `home` 根栏目及 11 个子栏目，归 `camie-home`；首页正文区块只读取上文列出的 `home-*` code。

```text
党建专栏 party
  党建要闻 party-news；学习教育 party-study；支部活动 party-branch-activity
部委动态 ministry
  工作动态 ministry-work；政策文件 ministry-policy-files
    国家鼓励发展的重大环保技术装备目录 ministry-major-equipment-catalogue
    环保装备制造业规范条件企业 ministry-qualified-enterprises
    重大环保技术装备创新任务揭榜挂帅 ministry-innovation-tasks；其他文件 ministry-other-files
  政策解读 ministry-policy-interpretation
新闻中心 news
  热点关注 news-hot；通知公告 news-notice；协会动态 news-association；会员动态 news-member
交流培训 training
  会议活动 training-meetings；人才培训 training-talent
    通知及动态 training-talent-notices；人才交流平台 training-talent-exchange
  国际交流与合作 training-international
    出海培训 training-overseas-training；海外考察 training-overseas-visits；交流对接 training-business-matching
科技标准 standards
  科技创新及成果转化 standards-innovation
    科技成果评价 standards-achievement-evaluation；科技成果转化平台 standards-transfer-platform
  标准工作 standards-work
    通知及动态 standards-work-notices；协会标准发布 standards-publications
  绿色技术推广 standards-green-promotion
关于协会 about
  协会简介 about-introduction；组织架构 about-organization；协会章程 about-charter
  分支机构 about-branches
    水分会 branch-water；大气分会 branch-atmosphere；固废分会 branch-solid-waste
    环境监测分会 branch-monitoring；噪声分会 branch-noise；紫外线分会 branch-uv
    臭氧分会 branch-ozone；人工智能分会 branch-ai；环境工程分会 branch-engineering
  专家委员会 about-experts
    专家视野 about-expert-insights
会员中心 member（公开静态化排除，会员实时内容另见 member_column）
  行业报告 member-reports；数据中心 member-data；电子刊物 member-publications；我的空间 member-space
视频专区 videos
  视频资讯 videos-news；会员专享 videos-members（私有静态化排除）
政策研究 policy-research
  行业报告 policy-reports；数据中心 policy-data
  国家鼓励发展的重大环保技术装备目录 policy-major-equipment-catalogue
  环保装备制造业规范条件企业 policy-qualified-enterprises
  重大环保技术装备创新任务揭榜挂帅 policy-innovation-tasks
```

注意三个容易重名的栏目：“行业报告”有公开的 `policy-reports` 和原结构中的 `member-reports`，真正会员列表还使用独立的 `member_column`“行业报告”；“数据中心”同理。维护时先确认页面区域与数据表，不能只按中文名投放。

## 后台改什么、怎样确认生效

1. **改公开文章**：在门户后台“图文管理”新建图文/视频，选择目标公开分类栏目；需要上首页时，再选择 `camie-home` 的对应首页栏目。填写不晚于当前时间的发布时间，提交审核并发布。检查静态化任务成功，再打开目标列表、详情、首页相关区块和搜索页。
2. **下线或删除公开文章**：后台操作后检查旧详情 URL 已失效，列表、首页、搜索均不再出现。自动静态化是尽力而为；若任务失败，恢复服务后手动重建全站并复查。
3. **改栏目名称、层级、排序或启用状态**：在“栏目管理”维护，保留已有稳定 code。公开页面需要重新生成；首页固定的区块数量、顺序、卡片和配图不会因为新增栏目自动变化。
4. **改会员栏目与会员内容**：在后台“会员专区”维护 `member_column`/`member_content`，不需要生成公开静态页；使用正式会员账号刷新动态页面核对，并用非正式会员账号核对前端拦截。服务端 `active` 校验尚未实现，需另外验证 API 直调并解决上述上线阻断项。
5. **改固定区块**：生产模板 `camie-home` 负责首页，`camie-party` 等五条页面模板负责各自栏目树的列表外观，`camie-about` 负责协会简介及其栏目列表；`camie-list` 负责视频/政策等通用列表及搜索、会员动态壳，`camie-article` 负责公开详情，`camie-layout` 负责全站布局。其 `source_code` 在后台“模板管理”；固定分支卡片/合作单位映射在生成器；CSS、JS、图片在源码资源目录。修改前先备份模板或资源，再生成到预览/测试目录核对。不要直接编辑 `dist/camie-portal` 或 `dist/camie-preview`。

当前首页横幅、专题入口、副会长单位和页脚友情链接**没有消费**后台广告/友情链接表；原 `camie-portal/frontend-preview` 中的 `data-display-type="ad/link"` 标记不等于这套生成器已完成接入。

### 特别容易误以为“已经接活”的位置

- 原预览“关于协会”菜单里出现过“公开公示”“联系我们”；当前 64 栏目树和生成页面中没有这两个栏目入口。若要恢复，需先确定栏目结构和展示方式，再接入生成器。
- `policy-data` 有公开栏目列表页，但首页没有独立“数据中心”模块。生成器虽计算过 `DataCenterURL`，当前首页模板没有使用它；只往该栏目发文章不会让首页多出数据中心卡片。
- 原预览首页广告位/友情链接的标记只是静态页面标记；当前生产模板里的横幅、专题入口、合作单位和页脚外链仍按上文的固定映射输出。
- `member-space` 是原公开栏目树中的结构节点，当前会员动态页只展示 `member_column` 的启用项；“我的空间”不是现有会员列表的第五个栏目。
