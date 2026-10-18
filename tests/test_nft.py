import subprocess

rule = """table inet attackbox {
    chain forward {
        type filter hook forward priority 0; policy drop;
        ct state established,related accept
        iifname "br0" oifname "br0" accept
        iifname "br0" oifname "docker0" accept
        iifname "br0" oifname "podman*" accept
        iifname "br0" oifname "tun*" accept
        iifname "br0" oifname "wg*" accept
        iifname "br0" ip daddr { 1.1.1.1, 8.8.8.8, 8.8.4.4, 9.9.9.9 } udp dport 53 accept
        iifname "br0" ip daddr { 1.1.1.1, 8.8.8.8, 8.8.4.4, 9.9.9.9 } tcp dport 53 accept
        iifname "br0" oifname "ens3" tcp dport { 80, 443 } accept
        iifname "br0" oifname "ens3" reject
    }
    chain input {
        type filter hook input priority 0;
        iifname "br0" ct state established,related accept
        iifname "br0" icmp type echo-request accept
        iifname "br0" reject
    }
    chain postrouting {
        type nat hook postrouting priority 100; policy accept;
        iifname "br0" oifname "ens3" masquerade
    }
}
"""

p = subprocess.run(["sudo", "nft", "-c", "-f", "-"], input=rule, capture_output=True, text=True)
print("RC:", p.returncode)
print("STDOUT:", p.stdout)
print("STDERR:", p.stderr)
