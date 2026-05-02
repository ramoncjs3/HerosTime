param(
    [Parameter(Mandatory = $true)]
    [string]$InputFile,

    [string]$OutDir = "configs"
)

$ErrorActionPreference = "Stop"

$servers = @(
    "g1", "g2", "g3",
    "h1", "h2", "h3", "h4", "h5", "h6", "h7", "h8", "h9", "h10", "h11", "h12", "h13", "h14", "h15", "h16", "h17", "h18", "h19", "h20", "h21", "h22", "h23",
    "h5_1", "h5_2", "h5_3", "h5_4", "h5_5", "h5_6", "h5_7", "h5_8", "h5_9", "h5_10", "h5_11", "h5_12", "h5_13", "h5_14", "h5_15",
    "b1", "b2", "b3", "b4", "b5", "b6", "b7", "b8", "b9", "b10", "b11", "b12", "b13", "b14", "b15", "b16", "b17", "b18", "b19", "b20", "b21", "b22", "b23", "b24", "b25"
)

$groupIds = Get-Content -LiteralPath $InputFile |
    ForEach-Object { $_.Trim() } |
    Where-Object { $_ -ne "" -and -not $_.StartsWith("#") }

if ($groupIds.Count -lt $servers.Count) {
    throw "Need at least $($servers.Count) group ids, got $($groupIds.Count)."
}

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

$mapping = [ordered]@{}
for ($i = 0; $i -lt $servers.Count; $i++) {
    $mapping[$servers[$i]] = $groupIds[$i]
}

$spare = @()
for ($i = $servers.Count; $i -lt $groupIds.Count; $i++) {
    $spare += [ordered]@{
        index = $i + 1
        group_id = $groupIds[$i]
        status = "spare"
    }
}

$mappingPath = Join-Path $OutDir "qq-groups.generated.json"
$sparePath = Join-Path $OutDir "qq-groups.spare.json"

$mapping | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $mappingPath -Encoding UTF8
$spare | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $sparePath -Encoding UTF8

Write-Host "Assigned $($servers.Count) servers -> $mappingPath"
Write-Host "Spare groups: $($spare.Count) -> $sparePath"
