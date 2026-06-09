# Plan: Integrate apks-filter into GitHub Actions workflow

## Goal
After `bundletool build-apks`, run `go run ./cmd/apks` to filter the `.apks` to keep only uncompressed `.so` variants with patched SDK targeting.

## Changes to `.github/workflows/aab.yml`

### 1. Add new step after "bundletool — all ABIs APKs" (after line 154)

```yaml
      - name: Filter .apks — keep uncompressed .so only
        if: ${{ inputs.build-apks }}
        run: |
          cd $GITHUB_WORKSPACE/workspace/crocson
          go run ./cmd/apks -o crocson-filtered.apks crocson.apks
          mv crocson-filtered.apks crocson.apks
          echo "Filtered .apks contents:"
          unzip -l crocson.apks
```

This replaces the original `crocson.apks` with the filtered version. No other steps need changes — the "Upload crocson.apks" step will upload the filtered file.

## That's it

Only one new step is needed. The existing upload step (`Upload crocson.apks`) already references `crocson.apks`, so after `mv` it picks up the filtered version automatically.
