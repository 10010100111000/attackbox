#!/bin/bash
# Runs as the box user inside a transient systemd unit.
# Starts Xtigervnc on a unix socket (no TCP port, no auth: the gateway is the
# only client and it connects over a 0700 directory), then the desktop.
set -u
BOX_USER="$1"; DISP="$2"; RUNDIR="$3"; GUISOCK="$4"

export HOME="/home/$BOX_USER"
export XDG_RUNTIME_DIR="$RUNDIR/xdg"
export DISPLAY=":$DISP"
export PATH="/usr/local/bin:/usr/bin:/bin:/usr/games"
export XDG_SESSION_TYPE=x11
export XDG_CURRENT_DESKTOP=MATE
export XDG_DATA_DIRS="/usr/share/mate:/usr/local/share:/usr/share"
export USER="$BOX_USER"; export LOGNAME="$BOX_USER"; export SHELL=/bin/bash

mkdir -p "$XDG_RUNTIME_DIR"
chmod 700 "$XDG_RUNTIME_DIR" 2>/dev/null
rm -f "$GUISOCK"

/usr/bin/Xtigervnc ":$DISP" -rfbunixpath "$GUISOCK" -rfbunixmode 384 -SecurityTypes None -AlwaysShared -geometry 1280x800 -depth 24 -desktop "attackbox $BOX_USER" > "$XDG_RUNTIME_DIR/xvnc.log" 2>&1 &
XVNC_PID=$!

for i in $(seq 1 80); do
  [ -S "$GUISOCK" ] && break
  sleep 0.25
done
echo "Xvnc pid=$XVNC_PID socket=$GUISOCK ready=$([ -S "$GUISOCK" ] && echo yes || echo no)" >> "$XDG_RUNTIME_DIR/xvnc.log"

# Compositing and animations are the two biggest single costs for VNC redraws.
# No helper file on disk: /run is mounted noexec, and /tmp is a shared sticky
# directory where two boxes would fight over the same path.
cd "$HOME" || exit 1
/usr/bin/dbus-run-session -- /bin/sh -c '
gsettings set org.mate.Marco.general compositing-manager false 2>/dev/null
gsettings set org.mate.interface enable-animations false 2>/dev/null
exec /usr/bin/mate-session
' >> "$XDG_RUNTIME_DIR/session.log" 2>&1
echo "session exited, stopping Xvnc" >> "$XDG_RUNTIME_DIR/session.log"
kill "$XVNC_PID" 2>/dev/null
wait "$XVNC_PID" 2>/dev/null
