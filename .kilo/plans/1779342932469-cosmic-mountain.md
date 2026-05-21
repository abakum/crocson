# Plan: Restructure fdroid4.yml workflow modes

## Goal

Rewrite `.github/workflows/fdroid4.yml` with three modes instead of the current `force` flag:

| `first` | `load` | Action |
|---------|--------|--------|
| `true`  | `false`| **Initial registration**: generate recipe from FyneApp.toml, create symlinks, commit to GitHub, push to fdroiddata GitLab, create MR |
| `false` | `true` | **Load from fdroiddata**: download current `com.github.abakum.crocson.yml` from fdroiddata GitLab into GitHub repo, commit and push |
| `false` | `false`| **Upload to fdroiddata**: take existing recipe + changelogs from GitHub repo, push to fdroiddata GitLab fork, create MR |

## Changes

### 1. Replace `force` input with `first` and `load`

```yaml
on:
  workflow_dispatch:
    inputs:
      first:
        description: 'Initial registration: generate recipe from FyneApp.toml'
        type: choice
        options: ['false', 'true']
        default: 'false'
      load:
        description: 'Load current recipe from fdroiddata GitLab'
        type: choice
        options: ['false', 'true']
        default: 'false'
```

### 2. Steps for `first=true` mode (initial registration)

1. Checkout
2. Read version info from FyneApp.toml
3. Check changelog originals exist
4. Create symlinks for all ABIs and locales
5. Generate recipe with Python (overwrites `Builds:` section completely)
6. Commit and push to GitHub
7. Create MR to fdroiddata (push recipe + changelogs via `cp -L`)

`first` всегда перезаписывает — проверка "version not already in recipe" не нужна.

### 3. Steps for `load=true` mode (download recipe only from fdroiddata)

Чейнжлоги мы делаем сами — они уже есть в репо. Загружаем только YAML-рецепт, т.к. его на стороне F-Droid обновляет скрипт-бот.

1. Checkout
2. Download `metadata/com.github.abakum.crocson.yml` from fdroiddata GitLab via API:
   ```
   curl -H "PRIVATE-TOKEN: ${GITLAB_TOKEN}" \
     "https://gitlab.com/api/v4/projects/fdroid%2Ffdroiddata/repository/files/metadata%2Fcom.github.abakum.crocson.yml/raw?ref=master" \
     -o metadata/com.github.abakum.crocson.yml
   ```
3. Commit and push to GitHub (no MR to fdroiddata)

### 4. Steps for `first=false, load=false` mode (upload to fdroiddata)

1. Checkout
2. Read version info from FyneApp.toml (for branch name and commit message)
3. Verify recipe exists locally
4. Create symlinks for all ABIs and locales (idempotent — re-creates if needed)
5. Commit and push changelogs to GitHub if changed
6. Create MR to fdroiddata: push recipe + changelogs via `cp -L`

### 5. Remove old `force` references

- Rename `force` → `first` in all conditions
- The `first=true` mode always force-pushes the branch (same as old force behavior)
- The `first=false, load=false` mode creates a new MR (like old non-force)

## Implementation details

- All three modes share step 1 (checkout)
- The `load` step needs `GITLAB_TOKEN` secret
- The MR creation step is shared between `first=true` and `first=false, load=false`
- The `Read version info` step is needed for:
  - `first=true`: recipe generation (commit SHA, build number)
  - `first=false, load=false`: branch name (`crocson-v${VERSION}`) and commit message
  - `load=true`: not needed but harmless to run
- The `first=false, load=false` mode reads version from FyneApp.toml for the branch name and commit message
- For `load=true` only the `.yml` recipe is downloaded (changelogs are maintained locally)

## Workflow step flow

```
                    ┌──────────────────────┐
                    │ Checkout             │ (always)
                    └──────────┬───────────┘
                               │
                    ┌──────────▼───────────┐
                    │ Read version info     │ (always)
                    └──────────┬───────────┘
                               │
                 ┌─────────────┼─────────────┐
                 │             │             │
          first=true    first=false     first=false
                         load=true       load=false
                 │             │             │
       ┌─────────▼──┐  ┌──────▼──────┐      │
       │ Check      │  │ Download    │      │
       │ changelogs │  │ recipe from │      │
       │ exist      │  │ fdroiddata  │      │
       └─────┬──────┘  └──────┬──────┘      │
             │                │             │
       ┌─────▼──────┐         │             │
       │ Create     │         │     ┌───────▼───────┐
       │ Create     │         │     │ Create        │
       │ symlinks   │         │     │ symlinks      │
       └─────┬──────┘         │     └───────┬───────┘
             │                │             │
       ┌─────▼──────┐         │             │
       │ Generate   │         │             │
       │ recipe     │         │             │
       │ (Python)   │         │             │
       └─────┬──────┘         │             │
             │                │             │
             └────────┬───────┘             │
                      │                     │
             ┌────────▼────────┐   ┌────────▼────────┐
             │ Commit+push     │   │ Commit+push     │
             │ to GitHub       │   │ symlinks only   │
             └────────┬────────┘   └────────┬────────┘
                      │                     │
                      │            ┌────────▼────────┐
                      │            │ Push recipe +   │
                      ├────────────│ changelogs to   │
                      │            │ fdroiddata fork │
                      │            └────────┬────────┘
                      │                     │
             ┌────────▼────────┐   ┌────────▼────────┐
             │ Create MR to    │   │ Create MR to    │
             │ fdroiddata      │   │ fdroiddata      │
             └─────────────────┘   └─────────────────┘
```

## Files to modify

- `.github/workflows/fdroid4.yml` — complete rewrite of inputs and conditional steps
