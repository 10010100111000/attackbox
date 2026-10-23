#!/usr/bin/env bash
set -e

echo "=========================================="
echo "Resetting AttackBox Database Structure Cleanly"
echo "=========================================="

DB_NAME="attackbox_db"
DB_USER="attackbox"

# 1. 停止 Web 服务以释放数据库连接
echo "--> [1/6] 停止 attackbox-web.service 释放连接..."
sudo systemctl stop attackbox-web.service || true

# 2. 重建 public schema（彻底清空所有旧表、旧约束、旧数据）
echo "--> [2/6] 重置 public schema..."
sudo -u postgres psql -d "${DB_NAME}" << 'EOF'
DROP SCHEMA IF EXISTS public CASCADE;
CREATE SCHEMA public;
GRANT ALL ON SCHEMA public TO attackbox;
GRANT ALL ON SCHEMA public TO postgres;
EOF

# 3. 创建迁移记录表
echo "--> [3/6] 初始化迁移记录表..."
sudo -u postgres psql -d "${DB_NAME}" << 'EOF'
CREATE TABLE IF NOT EXISTS "__drizzle_migrations_journal" (
  id serial PRIMARY KEY,
  tag varchar(255) NOT NULL UNIQUE,
  applied_at timestamp with time zone DEFAULT now() NOT NULL
);
ALTER TABLE "__drizzle_migrations_journal" OWNER TO attackbox;
EOF

# 4. 依次执行标准迁移脚本
echo "--> [4/6] 依次执行全量迁移脚本..."

MIGRATIONS_DIR="/tmp/migrations_payload"
rm -rf "${MIGRATIONS_DIR}"
mkdir -p "${MIGRATIONS_DIR}"
cp /home/laji/migrations/*.sql "${MIGRATIONS_DIR}/" 2>/dev/null || true
chmod -R 777 "${MIGRATIONS_DIR}"

for sqlfile in 0000_fancy_selene.sql 0001_yummy_trauma.sql 0002_mixed_juggernaut.sql; do
    if [ -f "${MIGRATIONS_DIR}/${sqlfile}" ]; then
        echo "    正在应用 ${sqlfile}..."
        sudo -u postgres psql -d "${DB_NAME}" -f "${MIGRATIONS_DIR}/${sqlfile}"
        TAG="${sqlfile%.sql}"
        sudo -u postgres psql -d "${DB_NAME}" -c "INSERT INTO __drizzle_migrations_journal (tag) VALUES ('${TAG}') ON CONFLICT DO NOTHING;"
    else
        echo "    警告: 未找到 ${sqlfile}"
        exit 1
    fi
done

# 5. 权限赋权与所有权归正
echo "--> [5/6] 规范所有表所有权与权限..."
sudo -u postgres psql -d "${DB_NAME}" << 'EOF'
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
        EXECUTE 'ALTER TABLE public.' || quote_ident(r.tablename) || ' OWNER TO attackbox;';
        EXECUTE 'GRANT ALL PRIVILEGES ON TABLE public.' || quote_ident(r.tablename) || ' TO attackbox;';
    END LOOP;
    FOR r IN (SELECT sequencename FROM pg_sequences WHERE schemaname = 'public') LOOP
        EXECUTE 'ALTER SEQUENCE public.' || quote_ident(r.sequencename) || ' OWNER TO attackbox;';
        EXECUTE 'GRANT ALL PRIVILEGES ON SEQUENCE public.' || quote_ident(r.sequencename) || ' TO attackbox;';
    END LOOP;
END $$;
EOF

# 6. 初始化初始管理员账号
echo "--> [6/6] 初始化管理员账户 (admin@attackbox.org)..."
cd /var/www/attackbox-web/.output/server
node << 'EOF'
import('@node-rs/argon2').then(async ({ hash, Algorithm }) => {
  const { default: postgres } = await import('postgres');
  const sql = postgres('postgres://attackbox:attackbox_pwd_9a7b12@127.0.0.1:5432/attackbox_db');
  const passwordHash = await hash('AttackBoxAdmin2026!', {
    algorithm: Algorithm.Argon2id,
    memoryCost: 65536,
    timeCost: 3,
    parallelism: 1
  });
  const [user] = await sql`
    INSERT INTO users (email, display_name, password_hash, role, status, email_verified_at, updated_at)
    VALUES ('admin@attackbox.org', 'Super Admin', ${passwordHash}, 'admin', 'active', now(), now())
    RETURNING id, email, role, status, display_name;
  `;
  console.log('[SUCCESS] Clean Admin created:', user);
  await sql.end();
});
EOF

rm -rf "${MIGRATIONS_DIR}"

# 7. 重启 Web 应用
echo "--> 重新启动 attackbox-web.service..."
sudo systemctl start attackbox-web.service
sleep 2

if ! sudo systemctl is-active --quiet attackbox-web.service; then
    echo "ERROR: attackbox-web.service 启动失败！"
    sudo journalctl -u attackbox-web.service --no-pager -n 20
    exit 1
fi

echo "=========================================="
echo "Database Reset & Service Restart Complete!"
echo "=========================================="
