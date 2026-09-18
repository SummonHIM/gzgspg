# 由 Product.wxs 生成 system / user 两个 MSI。
# 用法：powershell -File files/release/build-msi.ps1 -Version 1.3.0 -Arch amd64 -ExeName gzgspg-windows-amd64.exe
param(
    [Parameter(Mandatory = $true)][string]$Version,
    [Parameter(Mandatory = $true)][string]$Arch,
    [Parameter(Mandatory = $true)][string]$ExeName,
    [string]$DistDir = "dist",
    [string]$OutDir = "dist"
)

$ErrorActionPreference = "Stop"

# MSI 版本必须是四段数字：1.3.0 -> 1.3.0.0
$parts = $Version.Split("-")[0].Split(".")
while ($parts.Count -lt 4) { $parts += "0" }
$msiVersion = ($parts[0..3] -join ".")

# Go 的 GOARCH -> wix 的 -arch 与安装根目录。
# 系统级装在 ProgramFiles64Folder 时必须是 64 位 MSI；386 用 32 位 ProgramFiles。
switch ($Arch) {
    "amd64" { $wixArch = "x64";   $pfDir = "ProgramFiles64Folder" }
    "arm64" { $wixArch = "arm64"; $pfDir = "ProgramFiles64Folder" }
    "386"   { $wixArch = "x86";   $pfDir = "ProgramFilesFolder" }
    default { throw "unsupported arch: $Arch" }
}

$template = Get-Content -LiteralPath (Join-Path $PSScriptRoot "Product.wxs") -Raw

# wix build 把 <File Source> 相对 .wxs 所在目录解析，而模板写到 %TEMP%，故必须用绝对路径。
$exePath = (Resolve-Path (Join-Path $DistDir $ExeName)).Path

$scopes = @(
    @{ Name = "system"; Scope = "perMachine"; RootId = $pfDir },
    @{ Name = "user";   Scope = "perUser";    RootId = "LocalAppDataFolder" }
)

foreach ($s in $scopes) {
    $wxs = $template
    $wxs = $wxs.Replace("@@VERSION@@", $msiVersion)
    $wxs = $wxs.Replace("@@SCOPE@@", $s.Scope)
    $wxs = $wxs.Replace("@@INSTALL_ROOT_ID@@", $s.RootId)
    $wxs = $wxs.Replace("@@EXE@@", $exePath)

    $tmpWxs = Join-Path $env:TEMP "gzgspg-$($s.Name).wxs"
    Set-Content -LiteralPath $tmpWxs -Value $wxs -Encoding UTF8

    $out = Join-Path $OutDir "gzgspg-windows-$Arch-$Version-$($s.Name).msi"
    wix build -arch $wixArch -o $out $tmpWxs
    if (-not $?) { throw "wix build failed for scope $($s.Name)" }

    Remove-Item -LiteralPath $tmpWxs -Force
    Write-Output "wrote $out"
}
