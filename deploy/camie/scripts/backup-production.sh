#!/usr/bin/env bash
set -euo pipefail

backup_root=${1:-}
login_path=${MYSQL_LOGIN_PATH:-camie-backup}

if [[ -z "$backup_root" ]]; then
  echo "用法: MYSQL_LOGIN_PATH=camie-backup $0 /安全备份目录/camie-before-<version>" >&2
  exit 1
fi
if [[ -e "$backup_root" ]]; then
  echo "备份目标已存在，拒绝覆盖: $backup_root" >&2
  exit 1
fi

install -d -m 0700 "$backup_root"
mysqldump --login-path="$login_path" --single-transaction --routines --triggers --events \
  --default-character-set=utf8mb4 camie_portal | gzip -9 >"$backup_root/camie_portal.sql.gz"
mysqldump --login-path="$login_path" --single-transaction --routines --triggers --events \
  --default-character-set=utf8mb4 camie_member | gzip -9 >"$backup_root/camie_member.sql.gz"

paths=()
for path in \
  /srv/camie/site \
  /srv/camie/portal/uploads \
  /srv/camie/portal/private_uploads \
  /srv/camie/member/uploads \
  /etc/camie; do
  [[ -e "$path" ]] && paths+=("$path")
done
if [[ ${#paths[@]} -gt 0 ]]; then
  tar -czf "$backup_root/files-and-config.tar.gz" "${paths[@]}"
fi

gzip -t "$backup_root/camie_portal.sql.gz" "$backup_root/camie_member.sql.gz"
(cd "$backup_root" && sha256sum ./* > SHA256SUMS)
chmod -R go-rwx "$backup_root"
echo "生产备份完成: $backup_root"
