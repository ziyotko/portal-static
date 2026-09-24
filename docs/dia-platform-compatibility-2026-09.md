# dia-platform 兼容性核对（2026-09-23）

## 固定版本

- `dia-platform`: `8d64da8ba36e4d50d8c8c52f17b1435a43cb2a21`（本地 HEAD 与 `origin/main` 一致）
- `portal-static` 改造基线: `80e6ab11faf96b163039a8a919696423bec9c3df`
- 只读门户参考：`miic-portal` `66b5dc27fa1d2c677d9fc7e63d292d1c82c83219`；`caam-portal` `8357fab58980f9fd6e3af60b77f1b1b1cc5528cf`
- 数据参考：`Dump20260921.sql`（2026-09-21 快照，不作为最新 schema 的唯一依据）

## 已确认差异

1. `page` 实体及表已删除，Template 接管 `code/type/route_path/source_code/layout`；生产查询不能再依赖 `page_id`。
2. `column.template_id` 是栏目归属；父子栏目必须同属一个模板。栏目列表渲染模板仍按后台规则选启用的 `type=column` 模板。
3. `article_column_publish.template_id` 在审核完成时写入栏目所属模板 ID，不是详情渲染模板 ID。有效关系必须与栏目归属一致。
4. 文章进入静态页需 `status=1` 且 `audit_status=2`；内容、视频、数据三类按原页面能力分别处理。
5. 当前 Portal CMS 业务表采用物理删除；启动迁移负责清理旧软删除残留，静态服务不查询不存在的 `deleted_at`，也不写库清理。
6. Link 按 `template_id + column_id` 归属并要求启用；当前 CAAM 生成器没有独立广告读取路径。
7. 栏目预览 URL 为 `JoinAccessPath(column.route_path, columnTemplate.route_path)`；详情预览使用公共 detail 模板路由加文章 ID；站点 URL 与服务器物理输出目录相互独立。
8. 专题来源是启用的 `type=special` Template，单专题接口的 `id` 即模板 ID。
9. 后台只把系统 `StaticPath` 转发给静态服务；静态服务仍独立执行授权根目录和符号链接边界检查。

## 兼容策略

- 模板角色用 code（推荐）或唯一名称显式绑定，并在每次操作热加载。
- MIIC 以 `column.template_id` 维持资讯、业务、平台和关于页面的数据隔离；CAAM 首页槽位仍使用明确模板与精确栏目名，公共列表使用唯一栏目 ID/名称校验。
- 原门户 URL 不变；额外按新版 route_path 发布兼容别名，后台预览不会指向不存在的文件。
- 全站生成在普通页面成功后生成全部专题；专题批量与单项操作沿用公共鉴权、任务、超时和进度机制。

## SQL 快照注意事项

9 月 21 日快照中的模板启用状态和专题数据可能早于 9 月 23 日代码。验收工具只在临时 MySQL 容器内导入目标 schema；遇到缺少/禁用模板时应报告为测试数据不满足最新契约，不得修改生产库或伪造成功。
