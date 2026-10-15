#!/bin/bash
set -e
echo "=== 4-User Concurrent Launch Benchmark ==="
TOKENS=("tok-user1-pass" "tok-user2-pass" "tok-user3-pass" "tok-user4-pass")

for i in "${!TOKENS[@]}"; do
  tok="${TOKENS[$i]}"
  echo "Launching user $((i+1)) (box$((i+1)))..."
  curl -s -k -X POST https://127.0.0.1:443/login -d "token=$tok" >/dev/null &
done
wait

echo "Waiting 3 seconds for all sandboxes to initialize..."
sleep 3

echo ""
echo "=== Active btrfs Ephemeral Snapshots ==="
sudo btrfs subvolume list / | grep 'attackbox/boxes'

echo ""
echo "=== Active NetNS ==="
sudo ip netns list

echo ""
echo "=== Active Displays and Sockets ==="
ls -l /run/attackbox/*/gui.sock /run/attackbox/*/tmux.sock 2>/dev/null

echo ""
echo "=== Memory Consumption ==="
free -h

echo ""
echo "=== Per-User/Sandbox Memory (RSS) Breakdown ==="
ps -u parrot,box1,box2,box3,box4 -o pid,user,rss,comm | awk '
NR>1 { user_sum[$2] += $3; total += $3 }
END {
  for (u in user_sum) {
    printf "User %-8s: %8.2f MB\n", u, user_sum[u]/1024
  }
  printf "TOTAL DESKTOPS PHYSICAL RAM: %.2f MB (%.2f GB)\n", total/1024, total/1024/1024
}'

echo ""
echo "=== Host CPU Load ==="
uptime

echo ""
echo "=== Releasing All 4 Boxes ==="
for i in "${!TOKENS[@]}"; do
  tok="${TOKENS[$i]}"
  COOKIE=$(curl -s -k -i -X POST https://127.0.0.1:443/login -d "token=$tok" | grep -i Set-Cookie | awk '{print $2}' | tr -d '\r;')
  curl -s -k -X POST https://127.0.0.1:443/admin/kick -H "Cookie: $COOKIE" >/dev/null &
done
wait
sleep 2

echo ""
echo "=== Verification After Teardown ==="
echo -n "Residual btrfs snapshots: "
sudo btrfs subvolume list / | grep 'attackbox/boxes' || echo "0 (Clean)"
echo -n "Residual netns: "
sudo ip netns list | grep 'ab-box' || echo "0 (Clean)"
echo -n "Residual veth: "
ip link show | grep 'veth-box' || echo "0 (Clean)"
