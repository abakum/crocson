VSCODE_DIR := .vscode
SETTINGS_FILE := $(VSCODE_DIR)/settings.json
WSL_HOST_IP := $(shell ip route list default | awk '{print $$3}')
VERSION_NAME := $(shell grep -E '^\s*Version\s*=' FyneApp.toml | sed -E 's/^\s*Version\s*=\s*"([^"]+)".*/\1/')
BUILD_NUMBER := $(shell grep -E '^\s*Build\s*=' FyneApp.toml | sed -E 's/^\s*Build\s*=\s*([0-9]+).*/\1/')
DEB_FILE := crocson_$(VERSION_NAME)_amd64.deb

.PHONY: all clean arm arm64 386 amd64 linux windows wsl darwin ios install darm emulator adb wsladb logcat atags tags wtags t windowsgui deb debi debr debp useri userr repo local relay syso apk aapt apksigner align appimage

all: arm64

clean:
	go clean
	rm -f crocson.apk crocson.exe crocson_*.deb crocson*.xy crocson*.AppImage
	rm -rf crocson.app

arm:
	fyne package -os android/arm --release --sign

arm64:
	fyne package -os android/arm64 --release --sign

386:
	fyne package -os android/386 --release --sign

amd64:
	fyne package -os android/amd64 --release --sign

export GOPRIVATE := github.com/abakum/*,github.com/abakCroc/*
CROC_FORK := github.com/abakCroc/croc/v10
CROC_VERSION := $(shell go list -m -f '{{.Version}}' $(CROC_FORK)@main 2>/dev/null)
PEER_FORK := github.com/abakum/peerdiscovery
PEER_VERSION := $(shell go list -m -f '{{.Version}}' $(PEER_FORK)@main 2>/dev/null)
WH_FORK := github.com/abakum/wormhole-william
WH_VERSION := $(shell go list -m -f '{{.Version}}' $(WH_FORK)@master 2>/dev/null)
WW_MODULE := webwormhole.io
WW_FORK := github.com/abakum/webwormhole
WW_VERSION := $(shell go list -m -f '{{.Version}}' $(WW_FORK)@master 2>/dev/null)
ANET_FORK := github.com/abakum/anet
FYNE_FORK := github.com/abakCroc/fyne/v2
FYNE_VERSION := $(shell go list -m -f '{{.Version}}' $(FYNE_FORK)@fix-v2.7.4 2>/dev/null)

repo:
	@if [ -z "$(CROC_VERSION)" ]; then echo "ERROR: Cannot resolve $(CROC_FORK)@main"; exit 1; fi
	@echo "  $(CROC_FORK) $(CROC_VERSION)"
	@go mod edit -replace=github.com/schollz/croc/v10=$(CROC_FORK)@$(CROC_VERSION)
	@if [ -z "$(PEER_VERSION)" ]; then echo "ERROR: Cannot resolve $(PEER_FORK)@main"; exit 1; fi
	@echo "  $(PEER_FORK) $(PEER_VERSION)"
	@go mod edit -replace=github.com/schollz/peerdiscovery=$(PEER_FORK)@$(PEER_VERSION)
	@if [ -z "$(WH_VERSION)" ]; then echo "ERROR: Cannot resolve $(WH_FORK)@master"; exit 1; fi
	@echo "  $(WH_FORK) $(WH_VERSION)"
	@go mod edit -replace=github.com/psanford/wormhole-william=$(WH_FORK)@$(WH_VERSION)
	@if [ -z "$(WW_VERSION)" ]; then echo "ERROR: Cannot resolve $(WW_FORK)@master"; exit 1; fi
	@echo "  $(WW_FORK) $(WW_VERSION)"
	@go mod edit -replace=$(WW_MODULE)=$(WW_FORK)@$(WW_VERSION)
	@ANET_VERSION=$$(go list -m -f '{{.Version}}' $(ANET_FORK)@main); \
	if [ -z "$$ANET_VERSION" ]; then echo "ERROR: Cannot resolve $(ANET_FORK)@main"; exit 1; fi; \
	echo "  $(ANET_FORK) $$ANET_VERSION"; \
	go mod edit -replace=github.com/wlynxg/anet=$(ANET_FORK)@$$ANET_VERSION
	@if [ -z "$(FYNE_VERSION)" ]; then echo "ERROR: Cannot resolve $(FYNE_FORK)@fix-v2.7.4"; exit 1; fi
	@echo "  $(FYNE_FORK) $(FYNE_VERSION)"
	@go mod edit -replace=fyne.io/fyne/v2=$(FYNE_FORK)@$(FYNE_VERSION)
	@go mod tidy
	@echo "Done."

local:
	@echo "Switching replace directives to local paths in go.mod:"
	@go mod edit -replace=github.com/schollz/croc/v10=../abakCroc/croc
	@go mod edit -replace=github.com/schollz/peerdiscovery=../peerdiscovery
	@go mod edit -replace=github.com/psanford/wormhole-william=../wormhole-william
	@go mod edit -replace=webwormhole.io=../webwormhole
	@go mod edit -replace=github.com/wlynxg/anet=../anet
	@go mod edit -replace=fyne.io/fyne/v2=../abakCroc/fyne
	@go mod tidy
	@echo "Done."

atags:
	@mkdir -p $(VSCODE_DIR)
	@if [ -f $(SETTINGS_FILE) ]; then \
		jq '.gopls["build.buildFlags"] = ["-tags=android"]' $(SETTINGS_FILE) > $(SETTINGS_FILE).tmp && \
		mv $(SETTINGS_FILE).tmp $(SETTINGS_FILE); \
	else \
		echo '{"gopls": {"build.buildFlags": ["-tags=android"]}}' > $(SETTINGS_FILE); \
	fi
	@echo "Enabling Android build tags for gopls press Ctrl+Shift+P Go: Restart Language Server"

wtags:
	@mkdir -p $(VSCODE_DIR)
	@if [ -f $(SETTINGS_FILE) ]; then \
		jq '.gopls["build.buildFlags"] = ["-tags=android"]' $(SETTINGS_FILE) > $(SETTINGS_FILE).tmp && \
		mv $(SETTINGS_FILE).tmp $(SETTINGS_FILE); \
	else \
		echo '{"gopls": {"build.buildFlags": ["-tags=windows"]}}' > $(SETTINGS_FILE); \
	fi
	@echo "Enabling Windows build tags for gopls press Ctrl+Shift+P Go: Restart Language Server"

tags:
	@mkdir -p $(VSCODE_DIR)
	@if [ -f $(SETTINGS_FILE) ]; then \
		jq 'del(.gopls["build.buildFlags"])' $(SETTINGS_FILE) > $(SETTINGS_FILE).tmp && \
		mv $(SETTINGS_FILE).tmp $(SETTINGS_FILE); \
	else \
		echo '{}' > $(SETTINGS_FILE); \
	fi
	@echo "Reset build tags for gopls press Ctrl+Shift+P Go: Restart Language Server"

emulator:
	emulator -avd Medium_Phone_API_36.1

adb:
	adb install -r -d crocson.apk

apk:
	apkanalyzer manifest print crocson.apk

aapt:
	aapt2 dump badging crocson.apk

apksigner:
	apksigner verify -v --print-certs crocson.apk

align:
	$(ANDROID_HOME)/build-tools/35.0.0/zipalign -c -p -v 4 crocson.apk
	
logcat:
	adb logcat|grep "croc    :"

wlogcat:
	cmd.exe /c C:\Users\KAbak\AppData\Local\Android\Sdk\platform-tools\adb logcat|find "croc    :"

wsladb:
	export ADB_SERVER_SOCKET=tcp:$(WSL_HOST_IP):5037

linux:
	fyne package -os linux --release

windows: 
	#sudo apt-get install gcc-mingw-w64-x86-64
	#go install fyne.io/tools/cmd/fyne@latest
	CC=x86_64-w64-mingw32-gcc fyne package -os windows --release

releasew: 
	CC=x86_64-w64-mingw32-gcc fyne release -os windows -certificate croc.p12 -profile "croc" -developer "CN=croc, OU=Personal, O=Konstantin Abakumov, L=Millerovo, ST=Rostov Oblast, C=RU" -password "$(CERT_PASS)"


signexe:
	#sudo apt-get update;sudo apt-get install osslsigncode
	osslsigncode sign -pkcs12 croc.p12 -pass "$(CERT_PASS)" \
		-n "croc" \
		-t http://timestamp.digicert.com \
		-in crocson.exe -out crocson-signed.exe

signps1:
	osslsigncode sign -pkcs12 croc.p12 -pass "$(CERT_PASS)" \
		-n "croc" \
		-in croc-unsigned.ps1 -out croc.ps1

cert:
	rm cert.exe; \
	osslsigncode sign \
		-pkcs12 croc.p12 \
		-pass "$(CERT_PASS)" \
		-n "croc" \
		-in cmd/cert/cert.exe \
		-out cert.exe

links:
	adb shell pm get-app-links com.github.abakum.crocson

view:
	adb shell am start -a android.intent.action.VIEW -d "https://abakum.github.io/#123" com.github.abakum.crocson

signappx: 
	osslsigncode sign -pkcs12 croc.p12 -pass "$(CERT_PASS)" \
		-appx \
		-n "croc" \
		-t http://timestamp.digicert.com \
		-in crocson.appx -out crocson-signed.appx


windowsgui:
	#GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build -ldflags="-s -H windowsgui" -tags=opengl
	GOOS=windows CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 go build -ldflags="-s -H windowsgui"

mwindows:
	GOOS=windows CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 go build -ldflags="-s -extldflags=-mwindows"

wsl:
	GOOS=windows CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 go build -ldflags=-s

darwin: 
	fyne package -os darwin --release
	cp -r crocson.app /Applications/

ios: 
	fyne package -os ios --release

install:
	GOFLAGS=-ldflags=-s go install

relay:
	@CROC_SERVICE_BIN=$$(systemctl cat croc.service 2>/dev/null | grep -E '^ExecStart=' | head -1 | sed 's/ExecStart=//' | awk '{print $$1}'); \
	if [ -z "$$CROC_SERVICE_BIN" ]; then echo "ERROR: Cannot find ExecStart in croc.service"; exit 1; fi; \
	CROC_NEW=$$(which croc); \
	if [ -z "$$CROC_NEW" ]; then echo "ERROR: croc binary not found in PATH"; exit 1; fi; \
	echo "Service binary: $$CROC_SERVICE_BIN"; \
	echo "New binary:     $$CROC_NEW"; \
	sudo systemctl stop croc.service; \
	sudo cp "$$CROC_NEW" "$$CROC_SERVICE_BIN"; \
	sudo systemctl start croc.service; \
	sudo systemctl status croc.service

darm: 
	#brew install glfw
	GOARCH=arm64 fyne package -os darwin --release
	cp -r crocson.app /Applications/

damd: 
	#brew install glfw
	GOARCH=amd64 fyne package -os darwin --release
	cp -r crocson.app /Applications/

deb: crocson.tar.xz build-deb.sh DEBIAN/control DEBIAN/postinst DEBIAN/prerm DEBIAN/postrm
	@echo "Building .deb package..."
	@chmod +x build-deb.sh
	@./build-deb.sh

debi: $(DEB_FILE)
	@echo "Installing $(DEB_FILE)..."
	@sudo dpkg -i "$(DEB_FILE)"

$(DEB_FILE):
	@if [ ! -f "$(DEB_FILE)" ]; then \
		echo "ERROR: $(DEB_FILE) not found. Run 'make deb' first."; \
		exit 1; \
	fi

debr:
	@echo "Removing crocson package..."
	@sudo dpkg -r crocson

debp:
	@echo "Purging crocson package..."
	@sudo dpkg -P crocson

appimage: linux AppImageBuilder.sh
	@chmod +x AppImageBuilder.sh
	@./AppImageBuilder.sh

useri: crocson.tar.xz
	@echo "User installation from tar.xz..."
	@echo "Creating temporary directory..."
	@TEMP_DIR=$$(mktemp -d); \
	trap "rm -rf $$TEMP_DIR" EXIT; \
	echo "Extracting to $$TEMP_DIR..."; \
	tar -xf crocson.tar.xz -C "$$TEMP_DIR"; \
	cd "$$TEMP_DIR"; \
	echo "Installing for current user..."; \
	make user-install; \
	echo "User installation completed! Installed to ~/.local/bin/"; \
	echo "Run it with: gtk-launch com.github.abakum.crocson"

userr: crocson.tar.xz
	@echo "User uninstallation..."
	@echo "Creating temporary directory..."
	@TEMP_DIR=$$(mktemp -d); \
	trap "rm -rf $$TEMP_DIR" EXIT; \
	echo "Extracting to $$TEMP_DIR..."; \
	tar -xf crocson.tar.xz -C "$$TEMP_DIR"; \
	cd "$$TEMP_DIR"; \
	echo "Uninstalling from user directory..."; \
	make user-uninstall; \
	echo "User uninstallation completed! Removed from ~/.local/"

ialt:
	@echo "for Alt Linux..."
	sudo apt-get update
	sudo apt-get install -y \
		pkg-config \
		gcc \
		gcc-c++ \
		make \
		libGL-devel \
		libglfw-devel \
		libX11-devel \
		libXcursor-devel \
		libXrandr-devel \
		libXinerama-devel \
		libXi-devel \
		libXxf86vm-devel
	@echo "Alt Linux done"

ideb:
	@echo "for Debian/Ubuntu..."
	sudo apt-get update
	sudo apt-get install -y \
		pkg-config \
		gcc \
		g++ \
		make \
		libgl1-mesa-dev \
		libglfw3-dev \
		libgl-dev \
		libx11-dev \
		libxcursor-dev \
		libxrandr-dev \
		libxinerama-dev \
		libxi-dev
	@echo "Debian/Ubuntu done"

syso:
	@echo "Generating Windows resource .syso from FyneApp.toml..."
	@cd cmd/syso && go run . $(CURDIR)