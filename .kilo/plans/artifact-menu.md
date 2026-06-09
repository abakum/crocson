# Plan: Add artifact selection menu to workflow_dispatch

## Goal
Add `workflow_dispatch` inputs with boolean checkboxes so the user can choose which artifacts to build: `.aab`, `.apk`, `.apks`. Default: only `.aab`.

## Current flow
1. `fyne release` → `crocson.aab` (always, it's the source for everything else)
2. `bundletool — universal APK` → `crocson-all.apk` (from `.aab`, signed with Android keystore)
3. `bundletool — all ABIs APKs` → `crocson.apks` (from `.aab`, signed with Android keystore)
4. Three upload steps (always run)

## Keystore logic (unchanged)
- AAB: signed with **RuStore upload keystore** during `fyne release`
- APK / APKS: signed with **Android keystore** via bundletool

## Changes

### 1. Add `inputs` to `workflow_dispatch` (lines 3–4)

```yaml
on:
  workflow_dispatch:
    inputs:
      build-aab:
        description: 'Upload .aab'
        required: false
        default: true
        type: boolean
      build-apk:
        description: 'Build & upload universal .apk'
        required: false
        default: false
        type: boolean
      build-apks:
        description: 'Build & upload .apks (all ABI splits)'
        required: false
        default: false
        type: boolean
```

### 2. Add `if` conditions to build steps

| Step | Condition |
|---|---|
| `fyne release — build AAB` | Always runs (source for APK/APKS) |
| `bundletool — universal APK` | `${{ inputs.build-apk }}` |
| `bundletool — all ABIs APKs` | `${{ inputs.build-apks }}` |

### 3. Add `if` conditions to upload steps

| Upload step | Condition |
|---|---|
| `Upload crocson.aab` | `${{ inputs.build-aab }}` |
| `Upload crocson-all.apk` | `${{ inputs.build-apk }}` |
| `Upload crocson.apks` | `${{ inputs.build-apks }}` |

Also remove `if: success() || failure()` from upload steps — they should only run on success AND when selected. (Or keep it if you prefer uploads even on partial failure — let me know.)

### 4. Keystore decode — conditional

`Decode RuStore upload keystore` → always (needed for `fyne release`).
`Decode keystore` → only when APK or APKS is requested: `${{ inputs.build-apk || inputs.build-apks }}`

Keystore cleanup (`rm -f /tmp/keystore.jks /tmp/rustore-upload.keystore`) — keep in the last bundletool step that uses it, but only if that step runs. Alternatively, add a dedicated cleanup step that always runs.

### 5. Dedicated cleanup step (new, always runs)

```yaml
      - name: Cleanup keystores
        if: always()
        run: rm -f /tmp/keystore.jks /tmp/rustore-upload.keystore
```

Remove the inline `rm -f` from the bundletool steps.

## File modified
- `.github/workflows/aab.yml`
