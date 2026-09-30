# CAMIE column mapping

CAMIE uses `column.code` as its stable publishing key. Display names may change and are never used to decide access scope.

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

The generator excludes `member`, every `member-*` descendant and `videos-members` from public list pages, article details, and `generated-content.js`. It only emits `pages/member.html` and `pages/member-detail.html` as empty runtime shells. Authentication, member content, and protected media remain outside static generation until the member platform contract is finalized.

## Template codes

Production binds templates by `template.code`: `camie-layout`, `camie-home`, `camie-list`, `camie-article`, and `camie-about`. The example names are preview fallbacks only.
