# scripts/deploy_web_phase1.ps1
# AttackBox Frontend Refactoring - Phase 1 Automated Deployment Script
# Target: ssh laji@77.67.1.106 (/var/www/attackbox/)

$ErrorActionPreference = "Stop"
$RemoteUser = "laji"
$RemoteHost = "77.67.1.106"
$RemoteTarget = "${RemoteUser}@${RemoteHost}"
$ArchiveName = "web_dist_phase1.tar.gz"
$InstallScript = "scripts/remote_install.sh"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "  AttackBox Phase 1 Deployment to $RemoteTarget" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# 1. 验证单元与集成测试全部通过
Write-Host "[1/6] 运行前端测试套件 (npm test)..." -ForegroundColor Yellow
Push-Location web
try {
    npm test
    if ($LASTEXITCODE -ne 0) {
        throw "前端单元测试失败，终止部署。"
    }
} finally {
    Pop-Location
}

# 2. 执行静态预渲染生成 (npm run generate)
Write-Host "[2/6] 编译并预渲染全量静态站点 (npm run generate)..." -ForegroundColor Yellow
Push-Location web
try {
    npm run generate
    if ($LASTEXITCODE -ne 0) {
        throw "静态预渲染构建失败，终止部署。"
    }
} finally {
    Pop-Location
}

# 3. 打包构建产物
Write-Host "[3/6] 打包静态资源 (web/.output/public -> $ArchiveName)..." -ForegroundColor Yellow
if (Test-Path $ArchiveName) {
    Remove-Item $ArchiveName -Force
}
tar -czf $ArchiveName -C web/.output/public .
if ($LASTEXITCODE -ne 0 -or !(Test-Path $ArchiveName)) {
    throw "构建产物打包失败。"
}
$archiveSize = (Get-Item $ArchiveName).Length / 1KB
Write-Host "已生成归档包: $ArchiveName ($([math]::Round($archiveSize, 2)) KB)" -ForegroundColor Green

# 4. 上传归档与安装脚本至远端主机
Write-Host "[4/6] 上传归档包与安装脚本至远程服务器 $RemoteTarget..." -ForegroundColor Yellow
scp -B -q $ArchiveName "${RemoteTarget}:/home/${RemoteUser}/${ArchiveName}"
if ($LASTEXITCODE -ne 0) {
    throw "SCP 上传归档包失败。"
}
scp -B -q $InstallScript "${RemoteTarget}:/home/${RemoteUser}/remote_install.sh"
if ($LASTEXITCODE -ne 0) {
    throw "SCP 上传远程安装脚本失败。"
}

# 5. 执行远端安装与重载
Write-Host "[5/6] 远端备份、部署更新并重载 Nginx..." -ForegroundColor Yellow
ssh -n $RemoteTarget "bash /home/${RemoteUser}/remote_install.sh; rm -f /home/${RemoteUser}/remote_install.sh"
if ($LASTEXITCODE -ne 0) {
    throw "远端部署脚本执行失败。"
}

# 6. 验证公网 HTTP 访问
Write-Host "[6/6] 验证公网服务响应..." -ForegroundColor Yellow
Start-Sleep -Seconds 2

$endpoints = @(
    @{ Url = "http://${RemoteHost}/"; Name = "Landing Page (首页)"; Match = "AttackBox" },
    @{ Url = "http://${RemoteHost}/auth"; Name = "Auth Center (认证中心)"; Match = "AttackBox" },
    @{ Url = "http://${RemoteHost}/favicon.ico"; Name = "Favicon (站点图标)"; Match = "" },
    @{ Url = "http://${RemoteHost}/logo.png"; Name = "Brand Logo (品牌图标)"; Match = "" },
    @{ Url = "http://${RemoteHost}/images/auth-explore.webp"; Name = "Auth Explore WebP (登录页插画)"; Match = "" },
    @{ Url = "http://${RemoteHost}/images/auth-explore.png"; Name = "Auth Explore PNG (登录页插画原格式)"; Match = "" },
    @{ Url = "http://${RemoteHost}/images/linux.jpg"; Name = "Static Image (Linux 底图)"; Match = "" }
)

$allPassed = $true
foreach ($ep in $endpoints) {
    try {
        $resp = Invoke-WebRequest -Uri $ep.Url -Method Get -TimeoutSec 10 -UseBasicParsing
        if ($resp.StatusCode -eq 200) {
            if ($ep.Match -and $resp.Content -notmatch $ep.Match) {
                Write-Host "  [-] $($ep.Name): HTTP 200 但未匹配预期关键字 '$($ep.Match)'" -ForegroundColor Red
                $allPassed = $false
            } else {
                Write-Host "  [+] $($ep.Name): HTTP 200 OK ($($ep.Url))" -ForegroundColor Green
            }
        } else {
            Write-Host "  [-] $($ep.Name): 状态码 $($resp.StatusCode)" -ForegroundColor Red
            $allPassed = $false
        }
    } catch {
        Write-Host "  [-] $($ep.Name): 请求异常 - $($_.Exception.Message)" -ForegroundColor Red
        $allPassed = $false
    }
}

# 清理本地归档
if (Test-Path $ArchiveName) {
    Remove-Item $ArchiveName -Force
}

if ($allPassed) {
    Write-Host "==========================================================" -ForegroundColor Green
    Write-Host "  Phase 1 部署成功！体验地址:" -ForegroundColor Green
    Write-Host "  - 落地页:   http://${RemoteHost}/" -ForegroundColor Green
    Write-Host "  - 认证中心: http://${RemoteHost}/auth" -ForegroundColor Green
    Write-Host "==========================================================" -ForegroundColor Green
} else {
    throw "部分端点检查未通过，请检查远端日志。"
}
