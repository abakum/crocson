#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
FYNEAPP="$SCRIPT_DIR/../FyneApp.toml"
YML="$SCRIPT_DIR/com.github.abakum.crocson.yml"

VERSION="${1:-$(grep -E '^\s*Version\s*=' "$FYNEAPP" | sed -E 's/^\s*Version\s*=\s*"([^"]+)".*/\1/')}"
BUILD="${2:-$(grep -E '^\s*Build\s*=' "$FYNEAPP" | sed -E 's/^\s*Build\s*=\s*([0-9]+).*/\1/')}"
COMMIT_SHA="${3:-$(git rev-list -n 1 "v${VERSION}" 2>/dev/null || git rev-parse HEAD)}"
YML="${4:-$YML}"

if [ -z "$VERSION" ] || [ -z "$BUILD" ]; then
    echo "ERROR: Could not extract Version or Build from $FYNEAPP"
    exit 1
fi

TOOLS_SHA="${TOOLS_SHA:-$(git ls-remote https://github.com/abakum/tools refs/heads/main | awk '{print $1}')}"

if [ -z "$VERSION" ] || [ -z "$BUILD" ]; then
    echo "ERROR: Could not extract Version or Build from $FYNEAPP"
    exit 1
fi

echo "Version: $VERSION"
echo "Build: $BUILD"
echo "Commit: $COMMIT_SHA"
echo "Tools SHA: $TOOLS_SHA"
echo "Output: $YML"

HEADER_DEFAULT="Categories:
  - Internet
  - Connectivity
  - Multimedia
  - Security
License: ISC
AuthorName: Konstantin Abakumov
AuthorEmail: koka.abakum@gmail.com
SourceCode: https://github.com/abakum/crocson
IssueTracker: https://github.com/abakum/crocson/issues
Changelog: https://github.com/abakum/crocson/releases

AutoName: crocson

RepoType: git
Repo: https://github.com/abakum/crocson"

TAIL="AutoUpdateMode: Version
UpdateCheckMode: Tags ^v[\\d.]+\$
VercodeOperation:
  - '%c'
  - '%c + 1'
  - '%c + 2'
  - '%c + 3'
UpdateCheckData: FyneApp.toml|Build\\s*=\\s*(\\d+)|FyneApp.toml|Version\\s*=\\s*\"([^\"]+)\""

generate_builds() {
  echo "Builds:"
  OFF=0
  for ABI in arm arm64 386 amd64; do
    VC=$((BUILD + OFF))
    OFF=$((OFF + 1))
    cat << BEOF
  - versionName: ${VERSION}
    versionCode: ${VC}
    commit: ${COMMIT_SHA}
    sudo: apt-get install -y golang-go
    output: crocson-${ABI}.apk
    prebuild: sed -i 's/^Build = .*/Build = \$\$VERCODE\$\$/' FyneApp.toml
    build:
      - export GOPATH=\$HOME/go
      - export PATH=\$GOPATH/bin:\$PATH
      - git clone https://github.com/abakum/tools /tmp/tools
      - cd /tmp/tools/cmd/fyne
      - git checkout ${TOOLS_SHA}
      - go install .
      - cd -
      - rm -rf /tmp/tools
      - fyne package -os android/${ABI} --release
      - mv crocson.apk crocson-${ABI}.apk
    ndk: r27d

BEOF
  done
}

BUILDS=$(generate_builds)

if [ -f "$YML" ]; then
  HEADER=$(sed '/^Builds:/q' "$YML" | sed '$d')
else
  HEADER="$HEADER_DEFAULT"
fi

{
  printf '%s\n' "$HEADER"
  echo ""
  printf '%s\n' "$BUILDS"
  echo ""
  printf '%s\n' "$TAIL"
  echo "CurrentVersion: ${VERSION}"
  echo "CurrentVersionCode: $((BUILD + 3))"
} > "$YML"

echo "=== Generated recipe ==="
cat "$YML"
