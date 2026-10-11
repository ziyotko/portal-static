#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "CAMIE 数据库发布失败: $*" >&2
  exit 1
}

if [[ $# -ne 2 ]]; then
  echo "用法: $0 /opt/camie/releases/camie-release-<version> /安全备份目录/camie-before-<version>" >&2
  exit 1
fi
release_dir=$(cd -- "$1" && pwd -P) || fail "发布目录不存在: $1"
backup_root=$2
[[ "$backup_root" = /* ]] || fail "备份目录必须是绝对路径"
[[ ! -e "$backup_root" ]] || fail "备份目录已存在: $backup_root"

for file in VERSION SHA256SUMS db/preflight.sql db/migrate.sql ops/scripts/backup-production.sh; do
  [[ -f "$release_dir/$file" ]] || fail "发布包缺少 $file"
done
(cd "$release_dir" && sha256sum --check --status SHA256SUMS) || fail "发布包内部校验失败"
version=$(tr -d '\r\n' < "$release_dir/VERSION")
[[ "$version" =~ ^[0-9A-Za-z][0-9A-Za-z._-]{0,63}$ ]] || fail "发布版本号无效"
grep -Fq "SET @release_version = '$version';" "$release_dir/db/migrate.sql" || \
  fail "migrate.sql 与 VERSION 不匹配"

mysql_login_path=${MYSQL_MIGRATE_LOGIN_PATH:-camie-migrate}
backup_login_path=${MYSQL_BACKUP_LOGIN_PATH:-camie-backup}
state_dir=${CAMIE_RELEASE_DB_STATE_DIR:-/var/lib/camie/release-db}
install -d -m 0700 "$state_dir"
exec 9>"$state_dir/.lock"
flock -n 9 || fail "另一项 CAMIE 数据库发布正在执行"

sql_hash=$(sha256sum "$release_dir/db/migrate.sql" | cut -d ' ' -f 1)
marker="$state_dir/$version.sha256"
if [[ -e "$marker" ]]; then
  [[ $(cat "$marker") = "$sql_hash" ]] || fail "版本 $version 已记录，但迁移 SQL 哈希不同"
  fail "版本 $version 已执行过数据库迁移；请勿重复执行"
fi

MYSQL_LOGIN_PATH="$backup_login_path" \
  bash "$release_dir/ops/scripts/backup-production.sh" "$backup_root"

mysql --login-path="$mysql_login_path" --database=camie_portal \
  --batch --raw --skip-column-names \
  < "$release_dir/db/preflight.sql" > "$backup_root/preflight.tsv"

awk -F '\t' '
  $1 == "BLOCKER" {
    seen++
    if ($3 !~ /^[0-9]+$/ || $3 + 0 != 0) {
      print "BLOCKER " $2 "=" $3 > "/dev/stderr"
      blocked = 1
    }
  }
  END {
    if (seen == 0) {
      print "预检没有返回 BLOCKER 行" > "/dev/stderr"
      blocked = 1
    }
    exit blocked
  }
' "$backup_root/preflight.tsv" || fail "预检未通过，未执行迁移；备份和检查结果在 $backup_root"

mysql --login-path="$mysql_login_path" --database=camie_portal \
  --batch --raw --skip-column-names \
  < "$release_dir/db/migrate.sql" > "$backup_root/migrate.log" || \
  fail "迁移执行失败；请检查 $backup_root/migrate.log 和数据库状态，不要直接重试"

active_templates=$(mysql --login-path="$mysql_login_path" --database=camie_portal \
  --batch --raw --skip-column-names \
  -e "SELECT COUNT(*) FROM template WHERE code IN ('camie-layout','camie-home','camie-party','camie-ministry','camie-news','camie-training','camie-standards','camie-list','camie-article','camie-about') AND status=1 AND CHAR_LENGTH(source_code)>0")
[[ "$active_templates" = 10 ]] || fail "迁移后有效模板数为 $active_templates，预期 10；请检查数据库状态"

umask 077
printf '%s\n' "$sql_hash" > "$marker.tmp.$$"
mv -f -- "$marker.tmp.$$" "$marker"
echo "CAMIE 数据库发布完成: ${version}；备份与执行记录: $backup_root"
