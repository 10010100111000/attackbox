#!/usr/bin/env python3
import sys, json, os, subprocess, secrets

CONFIG_PATH = "/etc/attackbox/config.json"

def load_config():
    with open(CONFIG_PATH, "r", encoding="utf-8") as f:
        return json.load(f)

def save_config(cfg):
    tmp = CONFIG_PATH + ".tmp"
    with open(tmp, "w", encoding="utf-8") as f:
        json.dump(cfg, f, indent=2, ensure_ascii=False)
    os.replace(tmp, CONFIG_PATH)
    os.chmod(CONFIG_PATH, 0o600)
    subprocess.run(["systemctl", "restart", "attackboxd"], check=False)

def cmd_list():
    cfg = load_config()
    users = cfg.get("users", [])
    print(f"{'USER':<15} {'BOUND BOX':<12} {'TOKEN'}")
    print("-" * 65)
    for u in users:
        print(f"{u.get('name',''):<15} {u.get('box',''):<12} {u.get('token','')}")

def cmd_add(name, token=None):
    cfg = load_config()
    users = cfg.setdefault("users", [])
    for u in users:
        if u.get("name") == name:
            print(f"Error: user {name} already exists")
            sys.exit(1)
    if not token:
        token = secrets.token_hex(16)
    users.append({"name": name, "box": "box1", "token": token})
    save_config(cfg)
    print(f"Success: created user '{name}' with token: {token}")

def cmd_del(name):
    cfg = load_config()
    users = cfg.get("users", [])
    new_users = [u for u in users if u.get("name") != name]
    if len(new_users) == len(users):
        print(f"Error: user {name} not found")
        sys.exit(1)
    cfg["users"] = new_users
    save_config(cfg)
    print(f"Success: deleted user '{name}'")

if __name__ == "__main__":
    if len(sys.argv) < 2 or sys.argv[1] == "list":
        cmd_list()
    elif sys.argv[1] == "add":
        if len(sys.argv) < 3:
            print("Usage: attackbox-user add <username> [token]")
            sys.exit(1)
        tok = sys.argv[3] if len(sys.argv) > 3 else None
        cmd_add(sys.argv[2], tok)
    elif sys.argv[1] == "del":
        if len(sys.argv) < 3:
            print("Usage: attackbox-user del <username>")
            sys.exit(1)
        cmd_del(sys.argv[2])
    else:
        print("Usage: attackbox-user list | add <username> [token] | del <username>")
