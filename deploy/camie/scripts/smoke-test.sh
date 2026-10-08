#!/usr/bin/env bash
set -euo pipefail

origin=${1:-}
if [[ -z "$origin" || "$origin" != http://* && "$origin" != https://* ]]; then
  echo "用法: $0 https://正式域名" >&2
  exit 1
fi
origin=${origin%/}

check_json_code_zero() {
  local name=$1 url=$2 body
  body=$(curl --fail --silent --show-error --max-time 15 "$url")
  if ! printf '%s' "$body" | grep -Eq '"code"[[:space:]]*:[[:space:]]*0'; then
    echo "$name 业务码不是 0: $body" >&2
    return 1
  fi
  echo "OK $name"
}

curl --fail --silent --show-error --max-time 10 http://127.0.0.1:9144/healthz >/dev/null
echo "OK portal-static healthz"
check_json_code_zero "Portal site-info" "$origin/business_portal/api/site-info"
check_json_code_zero "Member site-info" "$origin/business_member/api/site-info"
curl --fail --silent --show-error --max-time 15 "$origin/" | grep -q 'class="home-page"'
echo "OK public home"
curl --fail --silent --show-error --max-time 15 "$origin/assets/images/default-news-cover.png" >/dev/null
echo "OK default news cover"

private_status=$(curl --silent --show-error --output /dev/null --write-out '%{http_code}' --max-time 10 \
  "$origin/business_portal/private_uploads/should-not-exist")
if [[ "$private_status" -ge 200 && "$private_status" -lt 400 ]]; then
  echo "private_uploads 似乎被公开（HTTP $private_status），必须检查 Nginx" >&2
  exit 1
fi
echo "OK private_uploads is not public (HTTP $private_status)"

echo "基础冒烟通过。仍需在后台人工验证 PATCH 提交审核、生成全站、内容上下线和会员视频 Range。"
