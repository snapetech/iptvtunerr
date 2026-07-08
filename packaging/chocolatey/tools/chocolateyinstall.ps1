$toolsDir = "$(Split-Path -parent $MyInvocation.MyCommand.Definition)"
$releaseTag = "v0.1.85"
$url = "https://github.com/snapetech/iptvtunerr/releases/download/v0.1.85/iptv-tunerr-v0.1.85-windows-amd64.zip"
$checksum = "9990afa39587c657ac49e9145e68527c2db1866a11d2c32ccf643b3edc256c86"

Install-ChocolateyZipPackage -PackageName 'iptvtunerr' -Url $url -UnzipLocation $toolsDir -Checksum $checksum -ChecksumType 'sha256'

$exe = Join-Path $toolsDir "iptv-tunerr-$releaseTag-windows-amd64\iptv-tunerr.exe"
if (-not (Test-Path $exe)) {
    throw "Expected executable was not found: $exe"
}

Install-BinFile -Name "iptv-tunerr" -Path $exe
