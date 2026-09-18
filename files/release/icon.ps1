# 由 assets/icon.png 生成多尺寸 assets/icon.ico。
# 用法：powershell -File files/release/icon.ps1
$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Drawing

$root = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$src = Join-Path $root "assets\icon.png"
$dst = Join-Path $root "assets\icon.ico"
if (-not (Test-Path -LiteralPath $src)) { throw "missing $src" }

# 源图非正方形（319x320），先填充成正方形再缩放，避免变形。
$source = [System.Drawing.Image]::FromFile($src)
$side = [Math]::Max($source.Width, $source.Height)
$square = New-Object System.Drawing.Bitmap -ArgumentList $side, $side

$g = [System.Drawing.Graphics]::FromImage($square)
$g.Clear([System.Drawing.Color]::Transparent)
$g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
$g.DrawImage($source, [int](($side - $source.Width) / 2), [int](($side - $source.Height) / 2), $source.Width, $source.Height)
$g.Dispose()

$sizes = 16, 32, 48, 64, 128, 256
$pngs = @()
foreach ($s in $sizes) {
    $bmp = New-Object System.Drawing.Bitmap -ArgumentList $s, $s
    $g2 = [System.Drawing.Graphics]::FromImage($bmp)
    $g2.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $g2.DrawImage($square, 0, 0, $s, $s)
    $g2.Dispose()
    $ms = New-Object System.IO.MemoryStream
    $bmp.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png)
    $pngs += , @{ Size = $s; Bytes = $ms.ToArray() }
    $ms.Dispose()
    $bmp.Dispose()
}
$square.Dispose()
$source.Dispose()

# ICO 容器：6 字节头 + 每图 16 字节目录项 + PNG 数据
$out = New-Object System.IO.MemoryStream
$bw = New-Object System.IO.BinaryWriter -ArgumentList $out
$bw.Write([UInt16]0); $bw.Write([UInt16]1); $bw.Write([UInt16]$pngs.Count)
$offset = 6 + 16 * $pngs.Count
foreach ($p in $pngs) {
    $dim = if ($p.Size -ge 256) { 0 } else { $p.Size }
    $bw.Write([Byte]$dim); $bw.Write([Byte]$dim)
    $bw.Write([Byte]0); $bw.Write([Byte]0)
    $bw.Write([UInt16]1); $bw.Write([UInt16]32)
    $bw.Write([UInt32]$p.Bytes.Length); $bw.Write([UInt32]$offset)
    $offset += $p.Bytes.Length
}
foreach ($p in $pngs) { $bw.Write($p.Bytes) }
$bw.Flush()
[System.IO.File]::WriteAllBytes($dst, $out.ToArray())
$bw.Dispose()
$out.Dispose()

Write-Output "wrote $dst"
