#!/usr/bin/env bash
node -e '
try {
  const m = { exports: {} };
  process.dlopen(m, "/var/www/attackbox-web/.output/server/node_modules/argon2/prebuilds/linux-x64/argon2.glibc.node");
  console.log("dlopen success!", Object.keys(m.exports));
} catch(e) {
  console.error("dlopen error:", e);
}
'
