[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$modulePath = 'github.com/outerstellar-hq/gouterstellar-platform'
$allowedPackages = @(
    "$modulePath/auth"
    "$modulePath/i18n"
    "$modulePath/migration"
    "$modulePath/observability"
    "$modulePath/ui"
    "$modulePath/web"
)
$allowedTopLevelEntries = @(
    '.github'
    '.gitignore'
    '.golangci-lint.yml'
    'AGENTS.md'
    'LICENSE'
    'Makefile'
    'README.md'
    'auth'
    'docs'
    'go.mod'
    'go.sum'
    'i18n'
    'migration'
    'observability'
    'scripts'
    'ui'
    'web'
)
$forbiddenRootEntries = @(
    'cmd'
    'config'
    'deployments'
    'extensions'
    'internal'
    'platform'
    'queries'
    'static'
)

function Invoke-Checked {
    param(
        [Parameter(Mandatory)]
        [string] $Command,
        [Parameter(ValueFromRemainingArguments)]
        [string[]] $Arguments
    )

    $output = & $Command @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$Command failed with exit code $LASTEXITCODE"
    }
    return $output
}

Push-Location $repoRoot
try {
    $failures = [System.Collections.Generic.List[string]]::new()

    $mainPackages = @(Invoke-Checked go list '-f' '{{if eq .Name "main"}}{{.ImportPath}}{{end}}' './...') |
        Where-Object { $_ }
    if ($mainPackages.Count -gt 0) {
        $failures.Add("executable packages are forbidden: $($mainPackages -join ', ')")
    }

    $actualPackages = @(Invoke-Checked go list '-f' '{{.ImportPath}}' './...') | Sort-Object
    $packageDifference = @(Compare-Object ($allowedPackages | Sort-Object) $actualPackages)
    if ($packageDifference.Count -gt 0) {
        $rendered = $packageDifference | ForEach-Object { "$($_.SideIndicator) $($_.InputObject)" }
        $failures.Add("public package allow-list changed:`n  $($rendered -join "`n  ")")
    }

    $trackedFiles = @(Invoke-Checked git ls-files '--cached' '--others' '--exclude-standard')
    $topLevelEntries = $trackedFiles |
        ForEach-Object { ($_ -split '/')[0] } |
        Sort-Object -Unique
    $entryDifference = @(Compare-Object ($allowedTopLevelEntries | Sort-Object) $topLevelEntries)
    if ($entryDifference.Count -gt 0) {
        $rendered = $entryDifference | ForEach-Object { "$($_.SideIndicator) $($_.InputObject)" }
        $failures.Add("repository root allow-list changed:`n  $($rendered -join "`n  ")")
    }

    foreach ($entry in $forbiddenRootEntries) {
        if (Test-Path -LiteralPath (Join-Path $repoRoot $entry)) {
            $failures.Add("forbidden application-host path exists: $entry")
        }
    }

    $forbiddenArtifacts = $trackedFiles | Where-Object {
        $_ -match '(^|/)(Dockerfile[^/]*|compose[^/]*\.ya?ml)$' -or
        $_ -match '\.(sql|toml)$'
    }
    if ($forbiddenArtifacts) {
        $failures.Add("application or deployment artifacts are forbidden:`n  $($forbiddenArtifacts -join "`n  ")")
    }

    $productPattern = ('star' + 'forge') + '|' + ('star' + 'line')
    $productMatches = $trackedFiles | ForEach-Object {
        Select-String -LiteralPath $_ -Pattern $productPattern -CaseSensitive:$false |
            ForEach-Object { "$($_.Path):$($_.LineNumber):$($_.Line)" }
    }
    if ($productMatches) {
        $failures.Add("product-specific references are forbidden:`n  $($productMatches -join "`n  ")")
    }

    if ($failures.Count -gt 0) {
        foreach ($failure in $failures) {
            Write-Error $failure
        }
        exit 1
    }

    Write-Output 'Library-only repository boundary verified.'
} finally {
    Pop-Location
}
