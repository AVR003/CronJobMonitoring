# Run once to move node_modules off OneDrive using a Windows junction.
# Safe to re-run.

$target  = "C:\dev-modules\monitoring"
$frontend = Join-Path $PSScriptRoot "frontend"
$link    = Join-Path $frontend "node_modules"

function Is-Junction($path) {
    $item = Get-Item $path -Force -ErrorAction SilentlyContinue
    return ($item -ne $null) -and ($item.Attributes.ToString() -match "ReparsePoint")
}

# --- Case 1: junction already exists ---
if (Is-Junction $link) {
    Write-Host "[setup] Junction already in place: $link -> $target"
    Write-Host "[setup] Running npm install..."
    Push-Location $frontend
    npm install
    Pop-Location
    Write-Host "[setup] Done."
    exit 0
}

# --- Case 2: plain node_modules on OneDrive (first run or after re-clone) ---

# Step 1: install packages into the plain node_modules on OneDrive
Write-Host "[setup] Step 1/3 - npm install (packages go to OneDrive temporarily)..."
Push-Location $frontend
npm install
Pop-Location

# Step 2: move node_modules to C:\dev-modules\monitoring
Write-Host "[setup] Step 2/3 - Moving node_modules to $target..."
if (Test-Path $target) {
    Remove-Item -Recurse -Force $target
}
Move-Item $link $target
Write-Host "[setup] Moved."

# Step 3: create junction from project back to target
Write-Host "[setup] Step 3/3 - Creating junction..."
New-Item -ItemType Junction -Path $link -Target $target | Out-Null
Write-Host "[setup] Junction created: $link -> $target"

Write-Host "[setup] Done. node_modules is at $target and will not be synced by OneDrive."
