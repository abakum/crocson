Release built via GitHub Actions for multiple platforms.
App Version read from `FyneApp.toml`.

### Included Platforms & Files:
- **Windows (x86_64)**: `crocson.exe`, `crocson.appx`
- **Linux (x86_64)**: `.tar.xz` archive and `.deb` package
- **macOS**: `.dmg` files for Apple Silicon (`crocson-arm64.dmg`) and Intel (`crocson-amd64.dmg`)
- **Android**: APK files for ARM64, ARM, x86_64, x86 architectures
- Metadata: `FyneApp.toml` (version info), `BUILD_INFO.txt` (build details)

### Package Manager Installation (Recommended):
- **macOS**: run once `brew install --cask abakum/tap/crocson` then `brew --cask upgrade crocson`
- **Linux**: run once `brew install abakum/tap/crocson` then `brew upgrade crocson`
- **Linux**: run `sudo dpkg -i crocson_*_amd64.deb`
- **Windows**: run once `scoop bucket add abakum https://github.com/abakum/homebrew-tap && scoop install crocson` then `scoop update crocson`
- **Windows**: run once as user [I_trust_the_signer_of_this.exe](https://github.com/abakum/I_trust_the_signer_of_this/releases) then `crocson.appx`

### Manual Installation:

#### Linux (from tar.xz):

**System installation (requires sudo):**
```
TEMP_DIR=$(mktemp -d) && trap "rm -rf $TEMP_DIR" EXIT && tar -xf crocson.tar.xz -C "$TEMP_DIR" && sudo make -C "$TEMP_DIR" -f Makefile install
```

**User installation (no sudo):**
```
TEMP_DIR=$(mktemp -d) && trap "rm -rf $TEMP_DIR" EXIT && tar -xf crocson.tar.xz -C "$TEMP_DIR" && make -C "$TEMP_DIR" -f Makefile user-install
```

#### macOS (from .dmg):
Mount the .dmg file and drag crocson.app to Applications folder

### Launch Instructions:
- **macOS**: `open /Applications/crocson.app` or click in Launchpad
- **Linux**: `crocson` or `gtk-launch com.github.abakum.crocson`
- **Windows**: `crocson` from Command Prompt/PowerShell

### Notes:
- Homebrew installations on Linux require [Homebrew for Linux](https://docs.brew.sh/Homebrew-on-Linux)
- After manual installation, you may need to log out and back in for desktop menu integration
- Android APKs are developer-signed and can be directly sideloaded or installed via ADB
- Since `crocson.appx` is signed with a self-signed certificate, run [I_trust_the_signer_of_this.exe](https://github.com/abakum/I_trust_the_signer_of_this/releases) to install the certificate into the **Trusted People** store automatically.
