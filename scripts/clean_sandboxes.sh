#!/bin/bash
set -e
echo "Cleaning up lingering sandboxes and cgroups..."
pkill -9 -f "tests.test" || true

for d in /sys/fs/cgroup/attackbox/sbx_*; do
    if [ -d "$d" ]; then
        echo "Killing cgroup $d"
        echo 1 > "$d/cgroup.kill" 2>/dev/null || true
        rmdir "$d" 2>/dev/null || true
    fi
done

for s in /var/lib/attackbox/instances/sbx_*; do
    if [ -d "$s/rootfs" ]; then
        echo "Deleting btrfs snapshot $s/rootfs"
        btrfs subvolume delete "$s/rootfs" 2>/dev/null || true
    fi
    rm -rf "$s" 2>/dev/null || true
done

echo "Current btrfs subvolumes:"
btrfs subvolume list /var/lib/attackbox
