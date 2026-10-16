[CmdletBinding()]
param([string]$Source = (Join-Path $env:ProgramData 'ajean\memory\generale'))
$ErrorActionPreference = 'Stop'
$destination = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot 'data\memory\generale'))
$sourceDirectory = Get-Item -LiteralPath $Source
if (-not $sourceDirectory.PSIsContainer -or ($sourceDirectory.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
    throw 'Choisir un vrai dossier de notes, sans lien symbolique.'
}
if ($sourceDirectory.FullName -eq $destination) { throw 'La source et la destination doivent etre differentes.' }
New-Item -ItemType Directory -Path $destination -Force | Out-Null
$copied = 0; $existing = 0; $encrypted = 0
foreach ($file in Get-ChildItem -LiteralPath $sourceDirectory.FullName -Filter '*.md' -File) {
    if ($file.Attributes -band [IO.FileAttributes]::ReparsePoint) { continue }
    if ($file.Name -match '(?i)(^|[._ -])(archives?|backup|bak|old|corrupted|copy|copie|temp|tmp)([._ -]|$)') { continue }
    $target = Join-Path $destination $file.Name
    if (Test-Path -LiteralPath $target) { $existing++; continue }
    $stream = [IO.File]::OpenRead($file.FullName)
    try {
        $prefix = New-Object byte[] 10
        $count = $stream.Read($prefix, 0, 10)
        $magic = [Text.Encoding]::ASCII.GetString($prefix, 0, $count)
    } finally { $stream.Dispose() }
    if ($magic -eq 'AJEANMEMv1') { $encrypted++; continue }
    [IO.File]::Copy($file.FullName, $target, $false)
    $copied++
}
Write-Host "Copies creees : $copied ; deja presentes : $existing ; notes chiffrees ignorees : $encrypted"
Write-Host "Destination : $destination"
Write-Host 'Les fichiers source ont uniquement ete lus. Aucun fichier existant dans le test n a ete remplace.'
