#!/usr/bin/env bash
# Read-only: run over SSH before preparing a single-server acceptance deployment.
# Does not inspect secret files, change services, install packages or create sandboxes.
set -euo pipefail
printf 'User: '; id
printf 'Kernel: '; uname -r
printf 'Machine: '; uname -m
printf '\nRuntime tools\n'
for tool in node npm go btrfs ip nft nsenter sudo; do
  if command -v "$tool" >/dev/null 2>&1; then printf '%s: available\n' "$tool"; elif sudo -n sh -c 'command -v "$1" >/dev/null 2>&1' sh "$tool" 2>/dev/null; then printf '%s: available (sudo PATH)\n' "$tool"; else printf '%s: missing\n' "$tool"; fi
done
printf '\nNon-interactive sudo\n'
if sudo -n true 2>/dev/null; then printf 'available\n'; else printf 'unavailable\n'; fi
printf '\nStorage\n'
df -h / /var /home
findmnt -rn -t btrfs -o TARGET,SOURCE || true
printf '\nCgroup\n'
stat -fc '%T' /sys/fs/cgroup
if test -f /sys/fs/cgroup/cgroup.controllers; then cat /sys/fs/cgroup/cgroup.controllers; fi
printf '\nCurrent AttackBox services\n'
systemctl list-units --all --type=service '*attackbox*' --no-pager --plain || true
printf '\nWeb service location (no environment values)\n'
systemctl show attackbox-web.service -p ActiveState -p User -p WorkingDirectory 2>/dev/null || true
printf '\nListening TCP ports\n'
ss -lnt
printf '\nKnown application directories\n'
for dir in /var/www/attackbox /var/www/attackbox-web /opt/attackbox /var/lib/attackbox; do
  if test -d "$dir"; then stat -c '%A %U:%G %n' "$dir"; fi
done
