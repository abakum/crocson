#!/usr/bin/env bash
set -euo pipefail

VERSION="$1"
BUILD="$2"
COMMIT_SHA="$3"
YML="$4"

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

TAIL="AllowedAPKSigningKeys: 15ea332b2b0f96a2fcf1cfcb77d4352cf2ae5af64e869e9fc51627ad5788ad5c

AutoUpdateMode: Version
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
    forceversion: true
    forcevercode: true
    prebuild:
      - sed -i 's/^Build = .*/Build = \$\$VERCODE\$\$/' FyneApp.toml
      - sed -i '/versionCode/s/="[0-9]*"/="\$\$VERCODE\$\$"/' AndroidManifest.xml
      - sed -i '/versionName/s/="[^"]*/="\$\$VERSION\$\$/' AndroidManifest.xml
    build:
      - export GOPATH=\$HOME/go
      - export PATH=\$GOPATH/bin:\$PATH
      - git clone https://github.com/abakum/tools /tmp/tools
      - cd /tmp/tools/cmd/fyne
      - git checkout 95e3874065474636a130efaea55a13dc45907713
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
