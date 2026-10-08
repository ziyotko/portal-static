#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
  echo "请以 root 运行 install-layout.sh" >&2
  exit 1
fi
if [[ $# -ne 1 ]]; then
  echo "用法: $0 /opt/camie/releases/camie-release-<version>" >&2
  exit 1
fi

release_dir=$(readlink -f -- "$1")
case "$release_dir" in
  /opt/camie/releases/camie-release-*) ;;
  *) echo "发布目录必须位于 /opt/camie/releases/camie-release-*" >&2; exit 1 ;;
esac
[[ -f "$release_dir/VERSION" && -f "$release_dir/bin/portal" && -f "$release_dir/bin/member" && -f "$release_dir/bin/portal-static" ]] || {
  echo "发布目录不完整" >&2
  exit 1
}

if ! getent group camie >/dev/null; then groupadd --system camie; fi
if ! id camie >/dev/null 2>&1; then
  useradd --system --gid camie --home-dir /srv/camie --shell /usr/sbin/nologin camie
fi

install -d -o root -g camie -m 0750 /etc/camie
install -d -o camie -g camie -m 0750 \
  /srv/camie /srv/camie/site /srv/camie/preview \
  /srv/camie/portal /srv/camie/portal/uploads /srv/camie/portal/private_uploads \
  /srv/camie/member /srv/camie/member/uploads \
  /var/log/camie/portal /var/log/camie/member

chmod 0755 "$release_dir"/bin/* "$release_dir"/ops/scripts/*.sh

install -m 0644 "$release_dir"/ops/systemd/camie-portal.service /etc/systemd/system/camie-portal.service
install -m 0644 "$release_dir"/ops/systemd/camie-member.service /etc/systemd/system/camie-member.service
install -m 0644 "$release_dir"/ops/systemd/camie-static.service /etc/systemd/system/camie-static.service

for name in portal member portal-static; do
  if [[ ! -f "/etc/camie/$name.yaml" ]]; then
    install -m 0640 -o root -g camie "$release_dir/ops/config/$name.yaml" "/etc/camie/$name.yaml"
  fi
  if [[ ! -f "/etc/camie/$name.env" ]]; then
    install -m 0640 -o root -g camie "$release_dir/ops/config/$name.env.example" "/etc/camie/$name.env"
  fi
done

ln -sfn /etc/camie/portal.yaml /srv/camie/portal/config.yaml
ln -sfn /etc/camie/member.yaml /srv/camie/member/config.yaml

systemctl daemon-reload
echo "目录和服务文件已安装。请先修改并校验配置、执行数据库迁移，再切换 /opt/camie/current；本脚本没有切换版本或启动服务。"
