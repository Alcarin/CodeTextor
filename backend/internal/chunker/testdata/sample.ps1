# Sample PowerShell Script
# TODO: Implement more features

function Get-Hello {
    param(
        [string]$Name = "World"
    )
    Write-Host "Hello, $Name!"
}

filter Where-Even {
    if ($_ % 2 -eq 0) { $_ }
}

$Global:AppVersion = "1.0.0"

# Import another module
using module ./MyModule.psm1
. ./Common.ps1

Get-Hello -Name "User" 1..10 | Where-Even
