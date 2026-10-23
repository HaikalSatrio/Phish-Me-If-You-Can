param(
    [ValidateRange(10, 600)][int]$TimeoutSeconds = 120
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $repositoryRoot 'deploy/searxng/compose.yaml'
$healthURL = 'http://127.0.0.1:8888/healthz'

function Test-SearchReady {
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri $healthURL -TimeoutSec 3
        return $response.StatusCode -eq 200 -and $response.Content.Trim() -eq 'OK'
    } catch {
        return $false
    }
}

function Test-DockerReady {
    $ErrorActionPreference = 'Continue'
    & docker info --format '{{.ServerVersion}}' *> $null
    return $LASTEXITCODE -eq 0
}

if (-not (Test-SearchReady)) {
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        throw 'Docker is not installed or not in PATH. Install and start Docker Desktop with Linux containers, then run this script again.'
    }
    if (-not (Test-DockerReady)) {
        Write-Host 'Starting Docker Desktop. Waiting for the Linux container engine...'
        & docker desktop start --timeout $TimeoutSeconds
        if ($LASTEXITCODE -ne 0 -or -not (Test-DockerReady)) {
            throw 'Docker engine is unavailable. Open Docker Desktop, select Linux containers, and resolve any WSL startup error before retrying. No search was attempted.'
        }
    }
    if ([string]::IsNullOrWhiteSpace($env:SEARXNG_SECRET)) {
        $secretBytes = New-Object byte[] 32
        $random = [System.Security.Cryptography.RandomNumberGenerator]::Create()
        try { $random.GetBytes($secretBytes) } finally { $random.Dispose() }
        $env:SEARXNG_SECRET = [Convert]::ToBase64String($secretBytes)
    }
    & docker compose -f $composeFile up -d
    if ($LASTEXITCODE -ne 0) {
        throw 'SearXNG container startup failed. Review the Docker error above. No search was attempted.'
    }
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    while (-not (Test-SearchReady)) {
        if ([DateTime]::UtcNow -ge $deadline) {
            throw 'SearXNG did not become healthy. Inspect: docker compose -f deploy/searxng/compose.yaml logs --tail 50'
        }
        Start-Sleep -Seconds 2
    }
}

try {
    $serviceConfig = Invoke-RestMethod -Uri 'http://127.0.0.1:8888/config' -TimeoutSec 5
    if (-not $serviceConfig.PSObject.Properties['engines']) {
        throw 'The service on port 8888 did not return a SearXNG engine configuration.'
    }
} catch {
    throw "SearXNG configuration check failed: $($_.Exception.Message)"
}

Write-Host 'SearXNG is ready at http://127.0.0.1:8888. Upstream search availability still depends on the network.'
Write-Host 'Run from the repository root:'
Write-Host '.\bin\pmiyc.exe search -source web -name "Haikal Satrio Dewandaru"'
Write-Host 'To search display names on DEV:'
Write-Host '.\bin\pmiyc.exe search -source web -platform devto -name "Ben"'
