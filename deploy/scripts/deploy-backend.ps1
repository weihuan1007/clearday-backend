param(
  [Parameter(Mandatory = $true)]
  [string]$VmHost,

  [string]$VmUser = "ubuntu",

  [string]$SshKey = "",

  [string]$RemotePackage = "/tmp/clearday-release.tar.gz"
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path -Path $PSScriptRoot -ChildPath "..\..")).Path
$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$packagePath = Join-Path ([System.IO.Path]::GetTempPath()) "clearday-release-$timestamp.tar.gz"
$sshTarget = "$VmUser@$VmHost"
$sshArgs = @()
$windowsOpenSSH = Join-Path -Path $env:WINDIR -ChildPath "System32\OpenSSH"
$scpCommand = Join-Path -Path $windowsOpenSSH -ChildPath "scp.exe"
$sshCommand = Join-Path -Path $windowsOpenSSH -ChildPath "ssh.exe"
$tarCommand = Join-Path -Path $env:WINDIR -ChildPath "System32\tar.exe"

if (-not (Test-Path -LiteralPath $scpCommand)) {
  $scpCommand = "scp"
}

if (-not (Test-Path -LiteralPath $sshCommand)) {
  $sshCommand = "ssh"
}

if (-not (Test-Path -LiteralPath $tarCommand)) {
  $tarCommand = "tar"
}

if ($SshKey -ne "") {
  if (-not (Test-Path -LiteralPath $SshKey)) {
    throw "SSH key file was not found: $SshKey"
  }

  $resolvedSshKey = (Resolve-Path -LiteralPath $SshKey).Path
  $sshArgs += @("-i", $resolvedSshKey, "-o", "IdentitiesOnly=yes")
}

try {
  Write-Host "Packaging ClearDay release..."
  if (Test-Path -LiteralPath $packagePath) {
    Remove-Item -LiteralPath $packagePath -Force
  }
  & $tarCommand --exclude "backend/.env" -czf $packagePath -C $repoRoot backend deploy
  if ($LASTEXITCODE -ne 0) {
    throw "release packaging failed"
  }

  Write-Host "Uploading release to $sshTarget..."
  & $scpCommand @sshArgs $packagePath "${sshTarget}:$RemotePackage"
  if ($LASTEXITCODE -ne 0) {
    throw "scp upload failed"
  }

$remoteCommandTemplate = @'
set -e
REMOTE_PACKAGE='__REMOTE_PACKAGE__'
sudo rm -rf /tmp/clearday-release
mkdir -p /tmp/clearday-release
tar -xzf "$REMOTE_PACKAGE" -C /tmp/clearday-release

RELEASE_DIR=/tmp/clearday-release
if [ ! -d "$RELEASE_DIR/backend" ]; then
  UPDATE_SCRIPT=$(find "$RELEASE_DIR" -maxdepth 4 -type f -path '*/deploy/aws/update-backend.sh' -print -quit)
  if [ -n "$UPDATE_SCRIPT" ]; then
    RELEASE_DIR=$(dirname "$(dirname "$(dirname "$UPDATE_SCRIPT")")")
  fi
fi

echo "Using release folder: $RELEASE_DIR"
if [ ! -d "$RELEASE_DIR/backend" ]; then
  echo "Release archive contents:" >&2
  find /tmp/clearday-release -maxdepth 4 -type d | sort >&2
  exit 1
fi

sudo bash "$RELEASE_DIR/deploy/aws/update-backend.sh" "$RELEASE_DIR"
'@

  $remoteCommand = $remoteCommandTemplate.Replace("__REMOTE_PACKAGE__", $RemotePackage)
  $remoteCommand = $remoteCommand -replace "`r`n", "`n"

  Write-Host "Building and restarting backend on $sshTarget..."
  $remoteCommand | & $sshCommand @sshArgs $sshTarget "bash -s"
  if ($LASTEXITCODE -ne 0) {
    throw "remote deployment failed"
  }

  Write-Host "Deployment completed."
}
finally {
  if (Test-Path -LiteralPath $packagePath) {
    Remove-Item -LiteralPath $packagePath -Force
  }
}
