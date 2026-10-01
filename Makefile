BINDIR    ?= $(HOME)/.local/bin
GAME_DIR  ?= $(HOME)/.local/share/Steam/steamapps/common/UNBEATABLE
PLUGIN_DIR = $(GAME_DIR)/BepInEx/plugins/UnbeatableRPC

GO     ?= go
DOTNET ?= dotnet

.PHONY: all plugin daemon test install uninstall clean

all: daemon plugin

plugin:
	$(DOTNET) build plugin/UnbeatableRPC.csproj -c Release -p:GameDir="$(GAME_DIR)"

daemon:
	$(GO) build -trimpath -o bin/unbeatable-rpc ./daemon/cmd/unbeatable-rpc
	$(GO) build -trimpath -o bin/fakegame ./daemon/cmd/fakegame

test:
	$(GO) vet ./...
	$(GO) test -race ./...

install: daemon plugin
	install -Dm755 bin/unbeatable-rpc "$(BINDIR)/unbeatable-rpc"
	install -Dm755 scripts/unbeatable-launch.sh "$(BINDIR)/unbeatable-launch.sh"
	@echo
	@echo "Installed. Steam launch options:"
	@echo '  WINEDLLOVERRIDES="winhttp=n,b" $(BINDIR)/unbeatable-launch.sh %command%'

uninstall:
	rm -f "$(BINDIR)/unbeatable-rpc" "$(BINDIR)/unbeatable-launch.sh"
	rm -rf "$(PLUGIN_DIR)"
	@echo "Removed the daemon, launch script and plugin. BepInEx is still in $(GAME_DIR);"
	@echo "see README.md to remove it too."

clean:
	rm -rf bin plugin/bin plugin/obj
