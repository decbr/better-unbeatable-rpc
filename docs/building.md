# Building

Needs Go 1.25+, the .NET SDK 8+, `make`, and the game installed: the plugin compiles against its DLLs, which can't be in the repo. Without the .NET SDK, use the prebuilt plugin in `plugin/dist/` with `PREBUILT_PLUGIN=plugin/dist/UnbeatableRPC.dll`.

## Linux

Also needs WebKitGTK 4.1 development files for the settings app.

```bash
make install    # or GAME_DIR=/path/to/UNBEATABLE make install; NO_GUI=1 skips the settings app
make test       # the daemon's tests
make test-gui   # the settings app's tests
```

## Windows, from Linux

Cross-compiles with no cgo or Windows toolchain (`zip` is needed for the zip):

```bash
make test-windows-cross    # type-checks the Windows code, tests included
make release-windows       # dist/better-unbeatable-rpc-<version>-windows-x86_64.zip
```

## Windows, on Windows

With Go 1.25+, GNU make (`winget install ezwinports.make`) and Git for Windows's `bin` folder on PATH, this builds and installs. Without the .NET SDK it uses the prebuilt plugin:

```bash
make install-windows    # INSTALL_ARGS=-Yes skips the questions
```

## Trying it without the game

`fakegame` sends the daemon what the plugin would:

```bash
make daemon
./bin/fakegame                 # menus, song, pause, results, story, closed
./bin/fakegame -hold arcade    # one state until Ctrl+C
```

The installed daemon already holds port 47820. Stop it first, or run a second one with `--port`/`--status-port` and `fakegame -port`.
