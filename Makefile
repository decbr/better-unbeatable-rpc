VERSION  ?= 0.1.0
GAME_DIR ?= $(HOME)/.local/share/Steam/steamapps/common/UNBEATABLE

GO     ?= go
DOTNET ?= dotnet

NAME        = better-unbeatable-rpc-$(VERSION)-linux-x86_64
RELEASE_DIR = dist/$(NAME)
PLUGIN_DIST = plugin/dist/UnbeatableRPC.dll

ifdef PREBUILT_PLUGIN
PLUGIN_DLL   = $(PREBUILT_PLUGIN)
RELEASE_DEPS = daemon
else
PLUGIN_DLL   = plugin/bin/Release/UnbeatableRPC.dll
RELEASE_DEPS = daemon plugin
endif

.PHONY: all plugin plugin-dist daemon test release install uninstall clean

all: daemon plugin

plugin:
	$(DOTNET) build plugin/UnbeatableRPC.csproj -c Release -p:GameDir="$(GAME_DIR)" -p:Version=$(VERSION)

plugin-dist: plugin
	install -Dm644 plugin/bin/Release/UnbeatableRPC.dll "$(PLUGIN_DIST)"

daemon:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o bin/unbeatable-rpc ./daemon/cmd/unbeatable-rpc
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o bin/fakegame ./daemon/cmd/fakegame

test:
	$(GO) vet ./...
	$(GO) test -race ./...

release: $(RELEASE_DEPS)
	rm -rf "$(RELEASE_DIR)"
	install -Dm755 bin/unbeatable-rpc "$(RELEASE_DIR)/unbeatable-rpc"
	install -Dm644 "$(PLUGIN_DLL)" "$(RELEASE_DIR)/UnbeatableRPC.dll"
	install -Dm755 scripts/install.sh scripts/uninstall.sh scripts/unbeatable-launch.sh -t "$(RELEASE_DIR)"
	install -Dm644 scripts/unbeatable-rpc.service README.md -t "$(RELEASE_DIR)"
	tar -C dist -czf "dist/$(NAME).tar.gz" "$(NAME)"
	@echo "Built dist/$(NAME).tar.gz"

install: release
	"$(RELEASE_DIR)/install.sh" $(INSTALL_ARGS)

uninstall:
	scripts/uninstall.sh $(UNINSTALL_ARGS)

clean:
	rm -rf bin dist plugin/bin plugin/obj
