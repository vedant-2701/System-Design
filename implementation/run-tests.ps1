[CmdletBinding()]
param(
    [Parameter(Position = 0, Mandatory = $true, ValueFromPipeline = $true)]
    [string]$Target
)

# 1. Resolve target path (handles relative and absolute paths)
$ResolvedPath = Resolve-Path $Target -ErrorAction SilentlyContinue
if (-not $ResolvedPath) {
    Write-Error "[ERROR] Path '$Target' could not be resolved. Please verify the file or directory exists."
    exit 1
}
$PathString = $ResolvedPath.Path

# Determine if target is a file or directory
$IsFile = (Test-Path -Path $PathString -PathType Leaf)
if ($IsFile) {
    $TargetDir = Split-Path -Path $PathString -Parent
    $FileName = Split-Path -Path $PathString -Leaf
    if ($FileName.EndsWith(".java")) {
        $TestClassName = $FileName.Substring(0, $FileName.Length - 5)
    } else {
        $TestClassName = $FileName
    }
} else {
    $TargetDir = $PathString
    $TestClassName = $null
}

# 2. Locate the JUnit console standalone runner JAR relative to this script
$JarPath = Join-Path $PSScriptRoot "lib\junit-platform-console-standalone-6.1.0.jar"
if (-not (Test-Path -Path $JarPath -PathType Leaf)) {
    Write-Error "[ERROR] JUnit Console Standalone runner JAR not found at '$JarPath'."
    exit 1
}

# Print configuration information
Write-Host ""
Write-Host "[INFO] Target Directory: $TargetDir" -ForegroundColor Cyan
if ($TestClassName) {
    Write-Host "[INFO] Target Test Class: $TestClassName" -ForegroundColor Cyan
} else {
    Write-Host "[INFO] Scanning target directory for all tests..." -ForegroundColor Cyan
}

# 3. Switch location to the target directory for compilation and execution
Push-Location $TargetDir

# 4. Compile all Java files in the target directory
Write-Host "[RUNNING] Compiling Java files in target directory..." -ForegroundColor Yellow
javac -cp ".;$JarPath" *.java

if ($LASTEXITCODE -ne 0) {
    Write-Host "[ERROR] Compilation failed. Restoring working directory and cleaning up." -ForegroundColor Red
    Write-Host "[CLEANUP] Removing generated class files..." -ForegroundColor Gray
    Remove-Item *.class -ErrorAction SilentlyContinue
    Pop-Location
    exit $LASTEXITCODE
}
Write-Host "[SUCCESS] Compilation completed successfully." -ForegroundColor Green

# 5. Execute tests using JUnit Console Standalone runner
Write-Host "[RUNNING] Starting JUnit execution..." -ForegroundColor Yellow
if ($TestClassName) {
    java -jar $JarPath execute --class-path . --select-class $TestClassName
} else {
    java -jar $JarPath execute --class-path . --scan-class-path
}
$TestExitCode = $LASTEXITCODE

# 6. Cleanup class files and restore directory location
Write-Host "[CLEANUP] Cleaning up generated class files..." -ForegroundColor Gray
Remove-Item *.class -ErrorAction SilentlyContinue
Pop-Location

# 7. Print final status and exit
if ($TestExitCode -ne 0) {
    Write-Host "[FAIL] Some tests failed or execution encountered errors (Exit Code: $TestExitCode)." -ForegroundColor Red
} else {
    Write-Host "[SUCCESS] All tests executed successfully." -ForegroundColor Green
}

exit $TestExitCode
