# CAMIE column mapping

For a Chinese, page-by-page maintenance map showing each visible area, its column, and whether it is fixed, statically generated, or fetched live, see [CAMIE 门户页面区块与栏目维护对照表](camie-site-areas.md).

For deployment, database separation, same-origin routes, and production acceptance, see [CAMIE 门户部署与运维交接](../camie-deployment.md).

CAMIE public static publishing uses `column.code` as its stable key. Public display names may change without changing the publishing scope; the separate runtime member-column API currently accepts member column IDs or Chinese names.

## Public navigation roots

| Code | Navigation |
| --- | --- |
| `party` | 党建专栏 |
| `ministry` | 部委动态 |
| `news` | 新闻中心 |
| `training` | 交流培训 |
| `standards` | 科技标准 |
| `about` | 关于协会 |

`policy-research` is grouped under 部委动态. `videos` is a public auxiliary section.

## Dynamic member boundary

The generator excludes `member`, every `member-*` descendant and `videos-members` from public list pages, article details, and `generated-content.js`. It emits `pages/member.html` and `pages/member-detail.html` as runtime shells; member data is fetched only in the browser.

The browser reads `localStorage["member-token"]`, calls `/business_member/api/member/profile`, and fetches member columns and content only when `data.status` is `active`. Operations set `adapter.site.member_login_path` and `adapter.site.member_register_path` in the CAMIE YAML config. Both must be root-relative paths routed on the same origin as the static site; the login link appends the current page as `returnUrl`. Regenerate the site after changing either value. Opening generated HTML via `file://` is unsupported because the member routes require an HTTP server and same-origin routing. `member.html?column=行业报告` selects a member column by name (an ID also works); the default is 行业报告 or the first enabled column. Public video navigation links to `member.html?column=会员专享&mode=video` for protected videos.

Private images, attachments, and videos use `/business_portal/api/member-zone/member-files/sign?name=...` at runtime. The video player refreshes its five-minute URL during long playback and restores the playback position. The HTML and search index contain no member body, private file URL, or signed URL. The member and portal APIs must be served under the same origin as the static CAMIE site.

The CAMIE browser gate protects only the user interface. In the restored `dia-platform` code, the external-member middleware validates the member JWT and logout blacklist, but does not check the current member profile or require `active` status. A non-`active` account with a valid token can call the member-zone list, detail, and file-signing APIs directly. The requirement that only active members can read private content is therefore not met at the server boundary. Resolve this in the API server or an upstream gateway before production deployment; browser checks alone cannot enforce it.

## Template codes

Production binds templates by `template.code`: `camie-home`, `camie-party`, `camie-ministry`, `camie-news`, `camie-training`, `camie-standards`, and `camie-about` are the seven enabled `home`-type top-navigation pages. `camie-list` is the single enabled `column`-type shared list renderer; `camie-article` is the `detail` renderer; `camie-layout` is a `special`-type shared layout. The five section pages define `<code>-list` Go template blocks; `camie-about` defines both `about` and `about-list`. The example names are preview fallbacks only.
