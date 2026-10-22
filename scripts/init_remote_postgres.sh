#!/usr/bin/env bash
set -euo pipefail

echo "=========================================="
echo "Initializing PostgreSQL for AttackBox"
echo "=========================================="

sudo systemctl enable --now postgresql

DB_USER="attackbox"
DB_PASS="attackbox_pwd_9a7b12"
PROD_DB="attackbox_db"
TEST_DB="attackbox_test"

sudo -u postgres psql <<EOF
DO \$\$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '${DB_USER}') THEN
        CREATE ROLE ${DB_USER} WITH LOGIN PASSWORD '${DB_PASS}';
    ELSE
        ALTER ROLE ${DB_USER} WITH PASSWORD '${DB_PASS}';
    END IF;
END
\$\$;
EOF

# 创建正式库
if ! sudo -u postgres psql -lqt | cut -d \| -f 1 | grep -qw "${PROD_DB}"; then
    echo "Creating database ${PROD_DB}..."
    sudo -u postgres createdb -O "${DB_USER}" "${PROD_DB}"
else
    echo "Database ${PROD_DB} already exists."
    sudo -u postgres psql -c "ALTER DATABASE ${PROD_DB} OWNER TO ${DB_USER};"
fi

# 创建测试库
if ! sudo -u postgres psql -lqt | cut -d \| -f 1 | grep -qw "${TEST_DB}"; then
    echo "Creating database ${TEST_DB}..."
    sudo -u postgres createdb -O "${DB_USER}" "${TEST_DB}"
else
    echo "Database ${TEST_DB} already exists."
    sudo -u postgres psql -c "ALTER DATABASE ${TEST_DB} OWNER TO ${DB_USER};"
fi

# 授予权限
sudo -u postgres psql -d "${PROD_DB}" -c "GRANT ALL PRIVILEGES ON SCHEMA public TO ${DB_USER};"
sudo -u postgres psql -d "${TEST_DB}" -c "GRANT ALL PRIVILEGES ON SCHEMA public TO ${DB_USER};"

# 验证连接
echo "Verifying connection to ${TEST_DB}..."
PGPASSWORD="${DB_PASS}" psql -h 127.0.0.1 -U "${DB_USER}" -d "${TEST_DB}" -c "SELECT current_user, current_database(), version();"

echo "PostgreSQL setup completed successfully."
