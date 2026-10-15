#!/bin/bash
# scripts/box-launch.sh <slot> <ip_suffix> <disp> <rootfs> <rundir> [mode: core|gui|all]
set -euo pipefail
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"

SLOT="$1"
IP_SUFFIX="$2"
DISP="$3"
ROOTFS="$4"
RUNDIR="$5"
MODE="${6:-core}"

BOX_IP="10.200.0.${IP_SUFFIX}"
VETH_HOST="veth-${SLOT}-br"
VETH_GUEST="veth-${SLOT}"
NETNS="ab-${SLOT}"
GUISOCK="${RUNDIR}/gui.sock"
TMUXSOCK="${RUNDIR}/tmux.sock"

start_core() {
  # 1. 建立运行目录
  mkdir -p "$RUNDIR"
  chmod 1777 "$RUNDIR"

  # 2. 原生 ip netns 隔离网络环境
  if ! ip netns list | grep -qw "$NETNS"; then
    ip netns add "$NETNS"
  fi

  # 3. 创建并配置 veth 网卡
  if ! ip link show "$VETH_HOST" >/dev/null 2>&1; then
    ip link del "$VETH_HOST" 2>/dev/null || true
    ip link del "$VETH_GUEST" 2>/dev/null || true
    ip link add "$VETH_GUEST" type veth peer name "$VETH_HOST"
    ip link set "$VETH_HOST" master br0 up
    ip link set "$VETH_GUEST" netns "$NETNS"

    ip netns exec "$NETNS" ip link set lo up
    ip netns exec "$NETNS" ip addr add "${BOX_IP}/24" dev "$VETH_GUEST"
    ip netns exec "$NETNS" ip link set "$VETH_GUEST" up
    ip netns exec "$NETNS" ip route add default via 10.200.0.1
  fi

  # 4. 挂载环境与设备树准备
  mkdir -p "${ROOTFS}/run/attackbox" "${ROOTFS}/proc" "${ROOTFS}/sys" "${ROOTFS}/dev" "${ROOTFS}/tmp/.X11-unix"
  mountpoint -q "${ROOTFS}/run/attackbox" || mount --bind "$RUNDIR" "${ROOTFS}/run/attackbox"
  mountpoint -q "${ROOTFS}/dev" || {
    mount --rbind /dev "${ROOTFS}/dev"
    mount --make-rprivate "${ROOTFS}/dev"
  }

  mkdir -p "${ROOTFS}/dev/pts" "${ROOTFS}/dev/shm"
  mountpoint -q "${ROOTFS}/dev/pts" || mount -t devpts devpts "${ROOTFS}/dev/pts" -o rw,nosuid,noexec,relatime,gid=5,mode=620,ptmxmode=666 2>/dev/null || true
  mountpoint -q "${ROOTFS}/dev/shm" || mount -t tmpfs tmpfs "${ROOTFS}/dev/shm" -o rw,nosuid,nodev,mode=1777 2>/dev/null || true
  chmod 666 "${ROOTFS}/dev/ptmx" 2>/dev/null || true

  mountpoint -q "${ROOTFS}/proc" || mount -t proc proc "${ROOTFS}/proc" 2>/dev/null || true
  mountpoint -q "${ROOTFS}/sys" || mount -t sysfs sys "${ROOTFS}/sys" 2>/dev/null || true
  chmod 1777 "${ROOTFS}/tmp" "${ROOTFS}/tmp/.X11-unix" 2>/dev/null || true

  # 掩盖 /home/laji 宿主管理员主目录，防止沙箱读取
  mkdir -p "${ROOTFS}/home/laji"
  mountpoint -q "${ROOTFS}/home/laji" || mount -t tmpfs tmpfs "${ROOTFS}/home/laji" -o ro,size=1k 2>/dev/null || true

  # 确保沙箱渗透用户 parrot 拥有完整且合法的独立 home 目录
  mkdir -p "${ROOTFS}/home/parrot"
  if [ -d "/home/parrot" ]; then
    cp -a /home/parrot/. "${ROOTFS}/home/parrot/" 2>/dev/null || true
  else
    cp -r /etc/skel/. "${ROOTFS}/home/parrot/" 2>/dev/null || true
  fi
  chown -R parrot:parrot "${ROOTFS}/home/parrot"
  chmod 0755 "${ROOTFS}/home/parrot"

  # 5. 启动 tmux 后台会话（仅占极少量内存）
  ip netns exec "$NETNS" chroot "$ROOTFS" su - parrot -c "
    if ! tmux -S /run/attackbox/tmux.sock has-session -t main 2>/dev/null; then
      tmux -S /run/attackbox/tmux.sock new-session -d -s main -c /home/parrot
    fi
  " 2>/dev/null || true
  chmod 777 "$TMUXSOCK" 2>/dev/null || true

  touch "${RUNDIR}/core.ready"
}

start_gui() {
  # 必须先确保 core 环境就绪
  start_core

  # 如果 gui.sock 已经存在且 Xtigervnc 正常运行，直接退出
  if [ -S "$GUISOCK" ]; then
    return 0
  fi

  rm -f "$GUISOCK"
  nohup ip netns exec "$NETNS" /usr/sbin/chroot "$ROOTFS" /bin/bash -c "
    /usr/bin/Xtigervnc \":$DISP\" -rfbunixpath /run/attackbox/gui.sock -rfbunixmode 384 \
      -SecurityTypes None -AlwaysShared -geometry 1280x800 -depth 24 &
    XVNC_PID=\$!

    for i in \$(seq 1 40); do
      [ -S /tmp/.X11-unix/X$DISP ] && [ -S /run/attackbox/gui.sock ] && break
      sleep 0.1
    done

    su - parrot -c \"
      export DISPLAY=:$DISP
      export XDG_CURRENT_DESKTOP=MATE
      export XDG_SESSION_TYPE=x11
      gsettings set org.mate.Marco.general compositing-manager false 2>/dev/null || true
      gsettings set org.mate.interface enable-animations false 2>/dev/null || true
      exec dbus-run-session /usr/bin/mate-session
    \"
  " >/dev/null 2>&1 < /dev/null &

  echo "$!" > "${RUNDIR}/gui.pid"

  # 等待 GUI 套接字就绪
  for i in $(seq 1 30); do
    [ -S "$GUISOCK" ] && break
    sleep 0.1
  done
}

stop_gui() {
  if [ -f "${RUNDIR}/gui.pid" ]; then
    pid=$(cat "${RUNDIR}/gui.pid" 2>/dev/null || true)
    if [ -n "$pid" ]; then
      kill -TERM "$pid" 2>/dev/null || true
      kill -9 "$pid" 2>/dev/null || true
    fi
    rm -f "${RUNDIR}/gui.pid"
  fi
  rm -f "$GUISOCK"
}

case "$MODE" in
  core)
    start_core
    ;;
  gui)
    start_gui
    ;;
  stop-gui)
    stop_gui
    ;;
  all)
    start_core
    start_gui
    ;;
  *)
    echo "Unknown mode: $MODE (valid: core, gui, stop-gui, all)" >&2
    exit 1
    ;;
esac
