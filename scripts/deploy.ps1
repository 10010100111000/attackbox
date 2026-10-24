# Push main, then run the canonical deploy on the server.
# The server must be able to fetch the repository (see docs/implementation/deploy.md).
$ErrorActionPreference = 'Stop'
$server = if ($env:ATTACKBOX_SERVER) { $env:ATTACKBOX_SERVER } else { 'laji@77.67.1.106' }
$branch = if ($env:ATTACKBOX_BRANCH) { $env:ATTACKBOX_BRANCH } else { 'main' }

git push origin $branch
ssh $server "cd /home/laji/attackbox && ./scripts/deploy.sh"