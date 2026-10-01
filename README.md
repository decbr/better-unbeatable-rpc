# better-unbeatable-rpc

Discord Rich Presence for [UNBEATABLE](https://store.steampowered.com/app/2240620/UNBEATABLE/) on Linux. \
Not windows at the moment. Sorry! I don't have a windows machine i can test with.

| Where you are | Top line | Second line |
|---|---|---|
| Arcade song | **RE-TUNED - peak divide ft. Rachel Lake** + elapsed timer | UNBEATABLE Lv 20 · PB: 1,155,577 (B) |
| Paused | **RE-TUNED - peak divide ft. Rachel Lake (paused)** | same, timer hidden |
| Results | **Results: RE-TUNED - peak divide ft. Rachel Lake** | same, PB updated if you just beat it |
| Story mode | **Story mode - Chapter 4** | ★ 8.703 |
| Menus | **In menus** | ★ 8.703 (your star rating / player rank) |

"UNBEATABLE" on the second line is the difficulty: the game's charts go Beginner, Easy, Normal, Hard, UNBEATABLE and Star.

The PB shown is the one for the board you're playing on. The game keeps separate high scores for normal speed ("Classic"), Double Time and Half Time.

## How it works

```
UNBEATABLE (Proton)                         Linux
┌─────────────────────────┐   UDP 47820   ┌──────────────────┐  discord-ipc-0  ┌─────────┐
│ BepInEx + UnbeatableRPC ├──────────────▶│ unbeatable-rpc   ├────────────────▶│ Vesktop │
│ (reads game state)      │  JSON, local  │ (daemon, Go)     │   Unix socket   │ (arRPC) │
└─────────────────────────┘               └──────────────────┘                 └─────────┘
```

A game running under Proton can't reach a Linux Discord socket. So a small BepInEx plugin inside the game sends its state over UDP to `127.0.0.1`, and a native daemon turns that into Rich Presence. All the presence logic lives in the daemon: formatting, rate limiting (at most one update every 4 s), reconnecting when Vesktop restarts, and clearing after 30 s of silence.

The plugin also switches off the game's own Discord integration. That integration is left over from the 2021 demo and shows up as "UNBEATABLE [white label]".

## Requirements

- UNBEATABLE (Steam, Windows build running under Proton).
- [Vesktop](https://github.com/Vencord/Vesktop) with **arRPC** enabled, or the official Discord client.
- To build: Go 1.23+, the .NET SDK (8 or newer), `make`.
- [BepInEx 5.4.23.5](https://github.com/BepInEx/BepInEx/releases/tag/v5.4.23.5) (`BepInEx_win_x64_5.4.23.5.zip`). It needs 5.4.23.5 or newer for Unity 6.

## Install

1. **BepInEx:** extract the BepInEx zip into the game folder, so that `winhttp.dll` sits next to `UNBEATABLE.exe`. The game folder is usually `~/.local/share/Steam/steamapps/common/UNBEATABLE`.
2. **Optional, for building with plain `dotnet build` or an IDE:**
   ```bash
   cp plugin/Directory.Build.props.example plugin/Directory.Build.props
   # edit GameDir in it if your game isn't in the default Steam library
   ```
   The Makefile passes the game folder itself, so `make` doesn't need this.
3. **Build and install:**
   ```bash
   make install    # daemon + launch script -> ~/.local/bin, plugin -> the game's BepInEx/plugins/
   ```
   If the game is in another Steam library, add `GAME_DIR=/path/to/UNBEATABLE`.
4. **Steam launch options** (right-click UNBEATABLE → Properties → General):
   ```
   WINEDLLOVERRIDES="winhttp=n,b" ~/.local/bin/unbeatable-launch.sh %command%
   ```
   Other wrappers go after the script, for example for gamemoderun:
   ```
   WINEDLLOVERRIDES="winhttp=n,b" ~/.local/bin/unbeatable-launch.sh gamemoderun %command%
   ```

The launch script starts the daemon, runs the game, and stops the daemon when the game exits. The daemon log is `~/.local/state/unbeatable-rpc/daemon.log`, and the previous run's log is `daemon.log.old`.

## Configuration

**Plugin:** `<game>/BepInEx/config/xyz.diaduck.unbeatable-rpc.cfg`, created on first launch.

| Key | Default | |
|---|---|---|
| `UdpPort` | `47820` | Must match the daemon's `--port`. |
| `DisableGameDiscord` | `true` | Stop the game's built-in Discord integration. |
| `DebugLogging` | `false` | Log scenes and every message to `BepInEx/LogOutput.log`. |
| `LastKnownStars` | (managed) | The last star rating seen, so the menus can show it before the game recalculates it. |

**Daemon:** flags, or environment variables. In the launch script, put extra flags in `UNBEATABLE_RPC_ARGS`, e.g. `UNBEATABLE_RPC_ARGS="--verbose"` in the launch options.

| Flag | Env | Default |
|---|---|---|
| `--client-id` | `UNBEATABLE_RPC_CLIENT_ID` | `1555205185792516146` (the "UNBEATABLE" app) |
| `--port` | `UNBEATABLE_RPC_PORT` | `47820` |
| `--large-image` | `UNBEATABLE_RPC_LARGE_IMAGE` | `logo` (`drp_icon` with the game's own client ID) |
| `--socket` | `UNBEATABLE_RPC_SOCKET` | searched automatically (see below) |
| `--verbose` | `UNBEATABLE_RPC_VERBOSE` | off |

## Testing without the game

```bash
make daemon
./bin/unbeatable-rpc --verbose &
./bin/fakegame              # menus → song → pause → results → story → closed, 8 s each
./bin/fakegame -once '{"v":1,"state":"idle","stars":12.345}'
make test
```

## Troubleshooting

**Nothing shows up in Discord**
- Check that Vesktop's arRPC is on: Vesktop Settings → *Rich Presence (arRPC)*.
- Check that Discord's activity sharing is on: User Settings → Activity Privacy → *Share your detected activities with others*.
- Look at `~/.local/state/unbeatable-rpc/daemon.log`:
  - `connected to …` and `activity set: …` mean the daemon side is fine.
  - `cannot reach Discord` means no socket was found; see below.
- No `game is running` line means the plugin isn't sending. Check `BepInEx/LogOutput.log` for `UnbeatableRPC … loaded`. If BepInEx itself isn't there, the `WINEDLLOVERRIDES` part of the launch options is missing.

**Another app's status shows up in Discord**
- Vesktop's arRPC passes every connected app's activity to Discord, and Discord then shows only one "Playing" activity. The daemon can't take priority over, or remove, another app's activity.
- The fix would be to turn off the other app's presence (or simply close the culprit) while you play. This is a limitation of all of discord.
- 
**Flatpak Vesktop**
- The socket lives at `$XDG_RUNTIME_DIR/.flatpak/dev.vencord.Vesktop/xdg-run/discord-ipc-0`. The daemon checks there automatically.
- If yours is somewhere else, pass `--socket /path/to/discord-ipc-0`. Find it with `ss -xlp | grep discord-ipc`.

**Stale sockets:** a `discord-ipc-N` file left behind by a client that has quit is harmless. The daemon skips sockets that refuse connections.

**After a game update** a patch may no longer apply. `BepInEx/LogOutput.log` shows `Failed to apply <patch>`. The other patches keep working.

## Uninstall

```bash
make uninstall   # removes ~/.local/bin/unbeatable-rpc, unbeatable-launch.sh and the plugin
```

Then, in the game folder:
- To remove BepInEx: delete `winhttp.dll`, `doorstop_config.ini`, `.doorstop_version`, `changelog.txt` and the `BepInEx/` folder.
- Set the Steam launch options back to empty, or just `gamemoderun %command%` or whatever you had before.

None of the game's own files are modified, so nothing needs restoring.

## Development

- `plugin/` is the BepInEx 5 plugin (C#, `netstandard2.1`). It references the game's DLLs from `GameDir` and never copies them into the repo.
- `daemon/` is the Go daemon (standard library only):
  - `internal/discordipc`: the socket protocol;
  - `internal/presence`: state → activity, and the update loop;
  - `cmd/fakegame`: the test sender.

Wire format, plugin → daemon (UDP, one JSON object per datagram; unknown fields are omitted):

```json
{"v":1,"state":"idle|arcade|story|results|closed","song":"…","artist":"…","difficulty":"…","level":20,
 "chapter":"Chapter 4","highScore":1155577,"accuracy":0.842,"rank":"B","stars":8.703,"paused":false,"startedAt":1790860862}
```

---

Unofficial fan project, not affiliated with D-CELL GAMES. UNBEATABLE is a trademark of its owners :)