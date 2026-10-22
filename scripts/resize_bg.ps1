Add-Type -AssemblyName System.Drawing

$srcPath = "F:\deepseek\attackbox\bg.png"
$destPath = "F:\deepseek\attackbox\web\public\images\auth-explore.png"

$src = [System.Drawing.Bitmap]::FromFile($srcPath)

# Bounding box with slight margin:
# minX=320, minY=88, maxX=1896, maxY=1840
$cropX = [Math]::Max(0, 300)
$cropY = [Math]::Max(0, 80)
$cropWidth = [Math]::Min($src.Width - $cropX, 1620)
$cropHeight = [Math]::Min($src.Height - $cropY, 1780)

# Target resolution: 600 x 660 (reduced pixel count ~90%, crisp for 2x retina display at ~300-360px CSS width)
$targetWidth = 600
$targetHeight = [int][Math]::Round($cropHeight * ($targetWidth / $cropWidth))

$dest = New-Object System.Drawing.Bitmap($targetWidth, $targetHeight, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
$g = [System.Drawing.Graphics]::FromImage($dest)
$g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
$g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
$g.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
$g.Clear([System.Drawing.Color]::Transparent)

$srcRect = New-Object System.Drawing.Rectangle($cropX, $cropY, $cropWidth, $cropHeight)
$destRect = New-Object System.Drawing.Rectangle(0, 0, $targetWidth, $targetHeight)
$g.DrawImage($src, $destRect, $srcRect, [System.Drawing.GraphicsUnit]::Pixel)

$dest.Save($destPath, [System.Drawing.Imaging.ImageFormat]::Png)

$g.Dispose()
$dest.Dispose()
$src.Dispose()

$fileInfo = Get-Item $destPath
Write-Output "Successfully saved cropped: $($fileInfo.FullName) - ${targetWidth}x${targetHeight} ($([math]::Round($fileInfo.Length / 1KB, 2)) KB)"
