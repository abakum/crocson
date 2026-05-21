#!/usr/bin/env bash
set -euo pipefail

VERSION="$1"
BUILD="$2"
COMMIT_SHA="$3"
YML="$4"

HEADER_DEFAULT="Categories:
  - Internet
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
  for entry in "android/arm arm 0" "android/arm64 arm64 1" "android/386 386 2" "android/amd64 amd64 3"; do
    OS=$(echo "$entry" | cut -d' ' -f1)
    ABI=$(echo "$entry" | cut -d' ' -f2)
    OFF=$(echo "$entry" | cut -d' ' -f3)
    VC=$((BUILD + OFF))
    cat << BEOF
  - versionName: ${VERSION}
    versionCode: ${VC}
    commit: ${COMMIT_SHA}
    sudo: apt-get install -y golang-go
    output: crocson-${ABI}.apk
    forceversion: true
    forcevercode: true
    prebuild:
      - sed -i 's/^Build = .*/Build = \$\$VERCODE\$\$/' FyneApp.toml
      - sed -i '/versionCode/s/=\"[0-9]*\"/=\"\$\$VERCODE\$\$\"/' AndroidManifest.xml
      - sed -i '/versionName/s/=\"[^\"]*/=\"\$\$VERSION\$\$/' AndroidManifest.xml
    build:
      - export GOPATH=\$HOME/go
      - export PATH=\$GOPATH/bin:\$PATH
      - git clone https://github.com/abakum/tools /tmp/tools
      - cd /tmp/tools/cmd/fyne
      - git checkout 95e3874065474636a130efaea55a13dc45907713
      - go install .
      - cd -
      - rm -rf /tmp/tools
      - fyne package -os ${OS} --release
      - zip -d crocson.apk "META-INF/*" || true
      - mv crocson.apk crocson-${ABI}.apk
    ndk: r26d
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
