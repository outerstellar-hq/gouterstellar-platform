[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$modulePath = 'github.com/outerstellar-hq/gouterstellar-platform'
$allowedPackages = @(
    "$modulePath/auth"
    "$modulePath/durablefile"
    "$modulePath/i18n"
    "$modulePath/migration"
    "$modulePath/observability"
    "$modulePath/ui"
    "$modulePath/web"
)
$allowedDirectModules = @(
    'github.com/alexedwards/argon2id'
    'github.com/alexedwards/scs/v2'
    'github.com/exaring/otelpgx'
    'github.com/golang-jwt/jwt/v5'
    'github.com/golang-migrate/migrate/v4'
    'github.com/gorilla/csrf'
    'github.com/jackc/pgx/v5'
    'github.com/magiconair/properties'
    'github.com/natefinch/atomic'
    'github.com/pquerna/otp'
    'go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc'
    'go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp'
    'go.opentelemetry.io/otel'
    'go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp'
    'go.opentelemetry.io/otel/sdk'
    'go.opentelemetry.io/otel/trace'
    'google.golang.org/grpc'
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
    'durablefile'
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

    $actualDirectModules = @(Invoke-Checked go list '-m' '-f' '{{if and (not .Main) (not .Indirect)}}{{.Path}}{{end}}' 'all') |
        Where-Object { $_ } |
        Sort-Object
    $moduleDifference = @(Compare-Object ($allowedDirectModules | Sort-Object) $actualDirectModules)
    if ($moduleDifference.Count -gt 0) {
        $rendered = $moduleDifference | ForEach-Object { "$($_.SideIndicator) $($_.InputObject)" }
        $failures.Add("direct dependency allow-list changed:`n  $($rendered -join "`n  ")")
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

    $forbiddenPaths = $trackedFiles | Where-Object {
        $_ -match '(^|/)(cmd|config|deployments?|extensions?|plugins?|queries|static)(/|$)'
    }
    if ($forbiddenPaths) {
        $failures.Add("application-host paths are forbidden:`n  $($forbiddenPaths -join "`n  ")")
    }

    $forbiddenArtifacts = $trackedFiles | Where-Object {
        $_ -match '(^|/)(Dockerfile[^/]*|compose[^/]*\.ya?ml)$' -or
        $_ -match '(^|/)(deploy|deployment|server|startup)([._-][^/]*)?\.(ya?ml|json|toml|ps1|sh|cmd|bat|service)$' -or
        $_ -match '(^|/)(plugin|extension|host|server|application|startup|wire)([_-][^/]*)?\.(go|ps1|sh|cmd|bat|ya?ml|json|toml)$' -or
        $_ -match '(\.pb|_generated|\.gen|_gen)\.go$' -or
        $_ -match '\.(sql|toml|exe|dll|so|dylib|jar|class|wasm|a|o|obj)$'
    }
    if ($forbiddenArtifacts) {
        $failures.Add("application or deployment artifacts are forbidden:`n  $($forbiddenArtifacts -join "`n  ")")
    }

    $consumerAssets = $trackedFiles | Where-Object {
        $_ -match '\.(html|css|jsx?|tsx?|png|jpe?g|gif|webp|svg|ico|woff2?|ttf|otf)$' -and
        $_ -ne 'ui/templates/shell.html'
    }
    if ($consumerAssets) {
        $failures.Add("consumer-owned templates and assets are forbidden:`n  $($consumerAssets -join "`n  ")")
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
            Write-Error $failure -ErrorAction Continue
        }
        exit 1
    }

    Write-Output 'Library-only repository boundary verified.'
} finally {
    Pop-Location
}
