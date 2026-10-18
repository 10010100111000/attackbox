#!/bin/bash
set -e

echo "=== Cleaning up any old test_target container ==="
sudo podman rm -f test_target 2>/dev/null || true

echo "=== Creating multi-service target container ==="
# Run alpine container with interactive init script
sudo podman run -d --name test_target --net podman alpine:latest sh -c "
  apk add --no-cache openssh samba redis python3;
  
  # Setup Web
  mkdir -p /srv/web;
  echo 'FLAG_HTTP_OK_ATTACKBOX' > /srv/web/index.html;
  python3 -m http.server 80 --directory /srv/web &
  
  # Setup SSH
  ssh-keygen -A;
  adduser -D targetuser;
  echo 'targetuser:TargetPass123!' | chpasswd;
  /usr/sbin/sshd;
  
  # Setup Samba (SMB)
  mkdir -p /srv/smb /var/log/samba;
  echo 'FLAG_SMB_OK_ATTACKBOX' > /srv/smb/flag.txt;
  cat << 'EOF' > /etc/samba/smb.conf
[global]
workgroup = WORKGROUP
server string = Target Samba
security = user
map to guest = Bad User
guest account = nobody

[shared]
path = /srv/smb
read only = yes
guest ok = yes
browseable = yes
EOF
  smbd -D;
  
  # Setup Redis
  redis-server --daemonize yes;
  sleep 1;
  redis-cli set target_flag 'FLAG_REDIS_OK';
  
  echo 'Target services ready';
  tail -f /dev/null
"

echo "Waiting for target container to initialize..."
sleep 5

# Fetch target container IP
TARGET_IP=$(sudo podman inspect -f '{{.NetworkSettings.Networks.podman.IPAddress}}' test_target 2>/dev/null || sudo podman inspect -f '{{.NetworkSettings.IPAddress}}' test_target)

echo "TARGET_IP=$TARGET_IP"
echo "$TARGET_IP" > /home/laji/target_ip.txt

# Verify services from host
echo "=== Verifying target ports from VPS host ==="
nc -z -w 3 "$TARGET_IP" 80 && echo "[OK] Target HTTP port 80 open"
nc -z -w 3 "$TARGET_IP" 22 && echo "[OK] Target SSH port 22 open"
nc -z -w 3 "$TARGET_IP" 445 && echo "[OK] Target SMB port 445 open"
nc -z -w 3 "$TARGET_IP" 6379 && echo "[OK] Target Redis port 6379 open"
