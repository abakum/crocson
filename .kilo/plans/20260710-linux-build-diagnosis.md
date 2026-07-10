# Diagnose and Fix Linux Build Failure in GitHub Actions

## Problem Summary
The Linux build is failing in GitHub Actions but works locally (albeit slowly). The workflow run shows:
- `build-linux` job failed after 3m 14s
- Step "Build with fyne for Linux" failed with exit code 1
- Step "Upload Linux artifacts" found no files

## Key Observations
From the GitHub Actions run logs:
1. Error annotation: "Build with fyne for Linux" - Process completed with exit code 1
2. Upload artifacts failed: No files found at expected paths:
   - `/home/runner/work/crocson/crocson/workspace/crocson/crocson.tar.xz`
   - `crocson_*_amd64.deb`
   - `crocson-*.AppImage`
3. Local build works but is slow

## Root Cause Analysis
The most likely causes are:

1. **Timeout during fyne package build** - The step runs `fyne package --os linux --release` which may be timing out on GitHub Actions due to:
   - Limited resources (CPU/RAM) on runners
   - Slower compilation of Go code with CGO
   - Network issues during dependency downloads

2. **Missing dependencies or environment issues** - Unlike your local environment, the GitHub runner may:
   - Have different system library versions
   - Missing required tools
   - Environment variable differences

3. **build-deb.sh and AppImageBuilder.sh dependencies** - These scripts depend on artifacts from the previous step (crocson.tar.xz), which may not have been created successfully

## Investigation Plan

### Step 1: Examine Build Scripts
- Read `AppImageBuilder.sh` to understand its dependencies
- Read `DEBIAN/control` to verify packaging requirements
- Check `FyneApp.toml` for version and build configuration

### Step 2: Compare Environment
- Document the differences between local and CI environments
- Check Go version, Fyne CLI version, and system libraries
- Identify any local customizations not present in CI

### Step 3: Analyze Timeout Issues
- Determine if the build is timing out (3m 14s total runtime)
- Check if GitHub Actions has a default timeout that's too short
- Look for optimization opportunities to speed up the build

### Step 4: Review Build Configuration
- Examine `go.mod` for CGO dependencies
- Check if any Make targets or build scripts are used locally but not in CI
- Verify that all required files (FyneApp.toml, Icon.png, etc.) are present

## Proposed Solutions

### Solution A: Increase Timeout and Add Diagnostics (Quickest)
1. Add explicit `timeout-minutes` to the build-linux job
2. Add diagnostic commands before each step:
   - Print Go version: `go version`
   - Print Fyne version: `fyne version`
   - Print system info: `uname -a`
   - List directory contents: `ls -la`
3. Add verbose flags to build commands to see what's happening

### Solution B: Optimize Build Process (Better Long-term)
1. Pre-compile dependencies before running `fyne package`
2. Use Go build cache more effectively
3. Consider splitting the build into multiple steps with caching
4. Add artifact caching for Go modules and build outputs

### Solution C: Add Fallback/Retry Logic (Most Robust)
1. If `fyne package` fails, retry once
2. If still failing, skip to next build method (e.g., direct `go build`)
3. Add comprehensive error logging to capture actual failure messages

### Solution D: Skip Problematic Builds (Temporary Workaround)
1. If .deb and AppImage builds are non-essential, skip them temporarily
2. Focus on getting the basic `fyne package` working first
3. Re-enable other build steps after fixing the core issue

## Validation Steps
After implementing fixes:
1. Re-run the GitHub Actions workflow manually
2. Check logs for actual error messages (not just exit code 1)
3. Verify all expected artifacts are created:
   - `crocson.tar.xz` from fyne package
   - `crocson_*_amd64.deb` from build-deb.sh
   - `crocson-*.AppImage` from AppImageBuilder.sh
4. Download and test artifacts on a real Linux system

## Open Questions
1. What is the actual error message from the failed `fyne package` step? (Not visible in the GitHub UI summary)
2. Is there a specific error in the runner logs that would indicate the root cause?
3. Have any recent changes to dependencies or Fyne CLI version affected the build?
4. Is the local build using any environment variables or tools not present in CI?

## Recommended Next Steps
1. First, read the actual workflow run logs to see the real error message
2. Examine the AppImageBuilder.sh and DEBIAN/control files
3. Compare the local build process with the CI workflow
4. Implement Solution A (timeout + diagnostics) first for quick feedback
5. Consider Solution B or C based on findings from diagnostics

## Files to Examine
- `.github/workflows/fyne.yml` (already reviewed)
- `build-deb.sh` (already reviewed)
- `AppImageBuilder.sh` (not yet examined)
- `DEBIAN/control` (not yet examined)
- `FyneApp.toml` (already partially visible)
- Makefile (visible in open tabs)