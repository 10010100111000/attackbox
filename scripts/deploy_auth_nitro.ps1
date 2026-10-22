# scripts/deploy_auth_nitro.ps1
# AttackBox Phase 2 Nitro Auth Service Deployment and Remote E2E Verification
$ErrorActionPreference = "Stop"

$RemoteUser = "laji"
$RemoteHost = "77.67.1.106"
$RemoteTarget = "${RemoteUser}@${RemoteHost}"
$ArchiveName = "web_nitro_dist.tar.gz"
$RemoteScript = "scripts/remote_deploy_nitro.sh"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "  AttackBox Nitro Auth Service Deployment to $RemoteTarget" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# 1. 验证输出目录存在
Write-Host "[1/5] 校验构建产物完整性..." -ForegroundColor Yellow
if (!(Test-Path "web/.output/server/index.mjs")) {
    throw "未找到构建产物 web/.output/server/index.mjs，请先执行 npm run build"
}

# 2. 打包生产构建产物与测试套件
Write-Host "[2/5] 打包 Nitro 服务端产物与 E2E 测试套件..." -ForegroundColor Yellow
if (Test-Path $ArchiveName) {
    Remove-Item $ArchiveName -Force
}

Push-Location web
try {
    tar -czf "../$ArchiveName" .output tests/auth-remote-e2e.mjs
} finally {
    Pop-Location
}

if (!(Test-Path $ArchiveName)) {
    throw "归档打包失败: $ArchiveName 未生成"
}

$archiveSize = (Get-Item $ArchiveName).Length / 1MB
Write-Host "已生成归档包: $ArchiveName ($([math]::Round($archiveSize, 2)) MB)" -ForegroundColor Green

# 3. 上传归档与部署脚本至远程主机
Write-Host "[3/5] 上传归档包与部署脚本至远程服务器 $RemoteTarget..." -ForegroundColor Yellow
scp -B -q $ArchiveName "${RemoteTarget}:/home/${RemoteUser}/${ArchiveName}"
scp -B -q $RemoteScript "${RemoteTarget}:/home/${RemoteUser}/remote_deploy_nitro.sh"

# 4. 远程执行解压、服务启动、Nginx 配置与 E2E 验证
Write-Host "[4/5] 远程执行部署与端到端自动化测试..." -ForegroundColor Yellow
ssh -n $RemoteTarget "bash /home/${RemoteUser}/remote_deploy_nitro.sh; rm -f /home/${RemoteUser}/remote_deploy_nitro.sh"
if ($LASTEXITCODE -ne 0) {
    throw "远程部署与端到端测试失败！"
}

# 5. 清理本地临时归档
Write-Host "[5/5] 清理本地临时构建归档..." -ForegroundColor Yellow
Remove-Item $ArchiveName -Force -ErrorAction SilentlyContinue

Write-Host "==========================================================" -ForegroundColor Green
Write-Host "  AttackBox Nitro 认证服务部署与 E2E 验证 100% 成功！" -ForegroundColor Green
Write-Host "==========================================================" -ForegroundColor Green
