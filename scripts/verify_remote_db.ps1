# scripts/verify_remote_db.ps1
# AttackBox Phase 2 Database Remote Verification Runner
$ErrorActionPreference = "Stop"

$RemoteUser = "laji"
$RemoteHost = "77.67.1.106"
$RemoteTarget = "${RemoteUser}@${RemoteHost}"
$ArchiveName = "db_verify_payload.tar.gz"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "  AttackBox Database Remote Verification to $RemoteTarget" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# 1. 临时打包测试所需文件
Write-Host "[1/4] 打包数据库迁移与验证测试套件 (包含 postgres 驱动)..." -ForegroundColor Yellow
if (Test-Path $ArchiveName) {
    Remove-Item $ArchiveName -Force
}

Push-Location web
try {
    tar -czf "../$ArchiveName" server/db/migrations tests/db-remote-verify.mjs package.json node_modules/postgres
} finally {
    Pop-Location
}

if (!(Test-Path $ArchiveName)) {
    throw "归档打包失败: $ArchiveName 未生成"
}

$archiveSize = (Get-Item $ArchiveName).Length / 1KB
Write-Host "已生成归档包: $ArchiveName ($([math]::Round($archiveSize, 2)) KB)" -ForegroundColor Green

# 2. 上传归档至远程服务器
Write-Host "[2/4] 上传归档至远程服务器 $RemoteTarget..." -ForegroundColor Yellow
scp -B -q $ArchiveName "${RemoteTarget}:/home/${RemoteUser}/${ArchiveName}"

# 3. 远程解压并执行自动化验证测试
Write-Host "[3/4] 远程执行数据库迁移与自动化验证测试..." -ForegroundColor Yellow
$remoteCommand = @"
set -e
rm -rf /home/${RemoteUser}/db_verify
mkdir -p /home/${RemoteUser}/db_verify
tar -xzf /home/${RemoteUser}/${ArchiveName} -C /home/${RemoteUser}/db_verify
rm -f /home/${RemoteUser}/${ArchiveName}
cd /home/${RemoteUser}/db_verify
DATABASE_URL="postgres://attackbox:attackbox_pwd_9a7b12@127.0.0.1:5432/attackbox_test" node --test tests/db-remote-verify.mjs
"@

ssh -n $RemoteTarget $remoteCommand
if ($LASTEXITCODE -ne 0) {
    throw "远程数据库自动化验证失败！"
}

# 4. 清理本地临时归档
Write-Host "[4/4] 清理本地临时归档..." -ForegroundColor Yellow
Remove-Item $ArchiveName -Force -ErrorAction SilentlyContinue

Write-Host "==========================================================" -ForegroundColor Green
Write-Host "  远程数据库全项迁移与约束验证 100% 成功通过！" -ForegroundColor Green
Write-Host "==========================================================" -ForegroundColor Green
