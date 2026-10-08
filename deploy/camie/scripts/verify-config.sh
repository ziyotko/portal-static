#!/usr/bin/env bash
set -euo pipefail

release_dir=${1:-/opt/camie/current}

fail() {
  echo "配置校验失败: $*" >&2
  exit 1
}

env_value() {
  local file=$1 key=$2 value
  value=$(sed -n "s/^${key}=//p" "$file" | tail -n 1)
  value=${value%$'\r'}
  if [[ "$value" == \"*\" && "$value" == *\" ]]; then
    value=${value:1:${#value}-2}
  elif [[ "$value" == \'*\' && "$value" == *\' ]]; then
    value=${value:1:${#value}-2}
  fi
  printf '%s' "$value"
}

for file in \
  "$release_dir/bin/portal" \
  "$release_dir/bin/member" \
  "$release_dir/bin/portal-static" \
  /etc/camie/portal.yaml \
  /etc/camie/member.yaml \
  /etc/camie/portal-static.yaml \
  /etc/camie/portal.env \
  /etc/camie/member.env \
  /etc/camie/portal-static.env; do
  [[ -f "$file" ]] || fail "缺少 $file"
done

for binary in portal member portal-static; do
  file "$release_dir/bin/$binary" | grep -q 'ELF 64-bit.*x86-64' || fail "$binary 不是 Linux x86-64 ELF"
done

if grep -R -n -E '__[A-Z0-9_]+__|CHANGE_ME|MUST_EQUAL' /etc/camie --include='*.yaml' --include='*.env'; then
  fail '/etc/camie 中仍有占位符'
fi

for env_file in /etc/camie/portal.env /etc/camie/member.env /etc/camie/portal-static.env; do
  mode=$(stat -c '%a' "$env_file")
  [[ "$mode" == 600 || "$mode" == 640 ]] || fail "$env_file 权限必须是 0600 或 0640，当前为 $mode"
done

portal_member_secret=$(env_value /etc/camie/portal.env PORTAL_MEMBER_JWT_SECRET)
member_secret=$(env_value /etc/camie/member.env MEMBER_JWT_SECRET)
[[ -n "$portal_member_secret" && "$portal_member_secret" == "$member_secret" ]] || \
  fail 'PORTAL_MEMBER_JWT_SECRET 与 MEMBER_JWT_SECRET 不一致'

portal_static_token=$(env_value /etc/camie/portal.env CAMIE_STATIC_TOKEN)
static_token=$(env_value /etc/camie/portal-static.env CAMIE_STATIC_TOKEN)
[[ -n "$portal_static_token" && "$portal_static_token" == "$static_token" ]] || \
  fail 'Portal 与 portal-static 的 CAMIE_STATIC_TOKEN 不一致'

portal_db_password=$(env_value /etc/camie/portal.env PORTAL_DB_PASSWORD)
portal_jwt_secret=$(env_value /etc/camie/portal.env PORTAL_JWT_SECRET)
member_db_password=$(env_value /etc/camie/member.env MEMBER_DB_PASSWORD)
static_dsn=$(env_value /etc/camie/portal-static.env CAMIE_DB_DSN)
[[ -n "$portal_db_password" ]] || fail 'PORTAL_DB_PASSWORD 未设置'
[[ -n "$member_db_password" ]] || fail 'MEMBER_DB_PASSWORD 未设置'
[[ -n "$static_dsn" ]] || fail 'CAMIE_DB_DSN 未设置'
[[ ${#portal_jwt_secret} -ge 32 ]] || fail 'PORTAL_JWT_SECRET 至少 32 个字符'
[[ ${#member_secret} -ge 32 ]] || fail 'MEMBER_JWT_SECRET 至少 32 个字符'
[[ ${#portal_static_token} -ge 32 ]] || fail 'CAMIE_STATIC_TOKEN 至少 32 个字符'

font_path=$(env_value /etc/camie/member.env MEMBER_CERT_FONT)
[[ -f "$font_path" ]] || fail "MEMBER_CERT_FONT 指向的文件不存在"
case "${font_path,,}" in
  *.ttf|*.otf) ;;
  *) fail 'MEMBER_CERT_FONT 必须是 TTF 或 OTF 文件' ;;
esac

grep -Eq '^[[:space:]]*dist_root:[[:space:]]*/srv/camie/site[[:space:]]*$' /etc/camie/portal-static.yaml || \
  fail 'portal-static dist_root 必须是 /srv/camie/site'

echo '配置、密钥关系、字体和 Linux 二进制校验通过。'
