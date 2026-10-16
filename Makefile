VERSION  ?= 0.3.2
GAME_DIR ?= $(HOME)/.local/share/Steam/steamapps/common/UNBEATABLE

GO     ?= go
DOTNET ?= dotnet

NAME        = better-unbeatable-rpc-$(VERSION)-linux-x86_64
RELEASE_DIR = dist/$(NAME)
PLUGIN_DIST = plugin/dist/UnbeatableRPC.dll

# the settings app links against WebKitGTK 4.1; NO_GUI=1 leaves it out
GUI_TAGS = desktop,production,webkit2_41

ifeq ($(OS),Windows_NT)
ifeq ($(shell command -v $(DOTNET)),)
PREBUILT_PLUGIN ?= $(PLUGIN_DIST)
endif
endif

ifdef PREBUILT_PLUGIN
PLUGIN_DLL   = $(PREBUILT_PLUGIN)
RELEASE_DEPS = daemon
else
PLUGIN_DLL   = plugin/bin/Release/UnbeatableRPC.dll
RELEASE_DEPS = daemon plugin
endif
ifndef NO_GUI
RELEASE_DEPS += gui
endif

WIN_NAME         = better-unbeatable-rpc-$(VERSION)-windows-x86_64
WIN_RELEASE_DIR  = dist/$(WIN_NAME)
WIN_BIN          = bin/windows
WIN_ENV          = GOOS=windows GOARCH=amd64 CGO_ENABLED=0
WIN_RELEASE_DEPS = $(filter plugin,$(RELEASE_DEPS)) daemon-windows gui-windows
WINRES = $(GO) run github.com/tc-hib/go-winres@v0.3.3 simply --arch amd64 \
	--product-name "UNBEATABLE RPC" --product-version $(VERSION) --file-version $(VERSION)

.PHONY: all plugin plugin-dist daemon gui test test-gui release install uninstall clean \
	winres daemon-windows gui-windows test-windows-cross stage-windows release-windows install-windows

all: daemon plugin gui

plugin:
	$(DOTNET) build plugin/UnbeatableRPC.csproj -c Release -p:GameDir="$(GAME_DIR)" -p:Version=$(VERSION)

plugin-dist: plugin
	mkdir -p plugin/dist
	install -m644 plugin/bin/Release/UnbeatableRPC.dll "$(PLUGIN_DIST)"

daemon:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o bin/unbeatable-rpc ./daemon/cmd/unbeatable-rpc
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o bin/fakegame ./daemon/cmd/fakegame

gui:
	cd gui && $(GO) build -tags $(GUI_TAGS) -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o ../bin/unbeatable-rpc-settings .

test:
	$(GO) vet ./...
	$(GO) test -race ./...

test-gui:
	cd gui && $(GO) vet -tags $(GUI_TAGS) . && $(GO) test -tags $(GUI_TAGS) .

release: $(RELEASE_DEPS)
	rm -rf "$(RELEASE_DIR)"
	mkdir -p "$(RELEASE_DIR)"
	install -m755 bin/unbeatable-rpc "$(RELEASE_DIR)/unbeatable-rpc"
	install -m644 "$(PLUGIN_DLL)" "$(RELEASE_DIR)/UnbeatableRPC.dll"
	install -m755 scripts/install.sh scripts/uninstall.sh scripts/unbeatable-launch.sh "$(RELEASE_DIR)"
	install -m644 scripts/unbeatable-rpc.service README.md LICENSE "$(RELEASE_DIR)"
ifndef NO_GUI
	install -m755 bin/unbeatable-rpc-settings "$(RELEASE_DIR)"
	install -m644 scripts/unbeatable-rpc-settings.desktop "$(RELEASE_DIR)"
endif
	tar -C dist -czf "dist/$(NAME).tar.gz" "$(NAME)"
	@echo "Built dist/$(NAME).tar.gz"

winres:
	$(WINRES) --manifest gui --file-description "UNBEATABLE RPC settings" --out gui/rsrc
	$(WINRES) --manifest none --file-description "UNBEATABLE RPC daemon" --out daemon/cmd/unbeatable-rpc/rsrc

daemon-windows: winres
	$(WIN_ENV) $(GO) build -trimpath -ldflags="-s -w -H windowsgui -X main.version=$(VERSION)" -o $(WIN_BIN)/unbeatable-rpc.exe ./daemon/cmd/unbeatable-rpc
	$(WIN_ENV) $(GO) build -trimpath -ldflags="-s -w" -o $(WIN_BIN)/fakegame.exe ./daemon/cmd/fakegame

gui-windows: winres
	cd gui && $(WIN_ENV) $(GO) build -tags desktop,production -trimpath -ldflags="-s -w -H windowsgui -X main.version=$(VERSION)" -o ../$(WIN_BIN)/unbeatable-rpc-settings.exe .

test-windows-cross:
	GOOS=windows $(GO) vet ./...
	cd gui && GOOS=windows $(GO) vet -tags desktop,production .

stage-windows: $(WIN_RELEASE_DEPS)
	rm -rf "$(WIN_RELEASE_DIR)"
	mkdir -p "$(WIN_RELEASE_DIR)/files"
	install -m644 scripts/install.cmd scripts/uninstall.cmd README.md LICENSE "$(WIN_RELEASE_DIR)"
	install -m644 scripts/install.ps1 $(WIN_BIN)/unbeatable-rpc.exe $(WIN_BIN)/unbeatable-rpc-settings.exe "$(PLUGIN_DLL)" "$(WIN_RELEASE_DIR)/files"

release-windows: stage-windows
	rm -f "dist/$(WIN_NAME).zip"
	cd dist && zip -qr "$(WIN_NAME).zip" "$(WIN_NAME)"
	@echo "Built dist/$(WIN_NAME).zip"

install: release
	"$(RELEASE_DIR)/install.sh" $(INSTALL_ARGS)

install-windows: stage-windows
	powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$(WIN_RELEASE_DIR)/files/install.ps1" $(INSTALL_ARGS)

uninstall:
	scripts/uninstall.sh $(UNINSTALL_ARGS)

clean:
	rm -rf bin dist plugin/bin plugin/obj gui/*.syso daemon/cmd/unbeatable-rpc/*.syso
