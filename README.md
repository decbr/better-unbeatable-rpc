# better-unbeatable-rpc

Discord Rich Presence for [UNBEATABLE](https://store.steampowered.com/app/2240620/UNBEATABLE/) on Linux. \
Not windows at the moment. Sorry! I don't have a Windows machine i can test with.

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

- UNBEATABLE on Steam, running under Proton.
- [Vesktop](https://github.com/Vencord/Vesktop) with **arRPC** enabled, or the official Discord client.

## Install

1. Download `better-unbeatable-rpc-<version>-linux-x86_64.tar.gz` from the [releases](https://github.com/decbr/better-unbeatable-rpc/releases) and extract it.
2. Quit the game, then run the installer:
   ```bash
   cd better-unbeatable-rpc-*-linux-x86_64
   ./install.sh
   ```
3. Launch UNBEATABLE.

The installer:
- finds the game in any of your Steam libraries (Flatpak Steam included);
- downloads [BepInEx 5.4.23.5](https://github.com/BepInEx/BepInEx/releases/tag/v5.4.23.5) and checks its checksum (it skips this if BepInEx is already installed);
- puts the plugin in the game's `BepInEx/plugins/`, and the daemon in `~/.local/bin`, as a systemd user service that starts when you log in. The daemon sits idle until the game is running.
- asks whether to store BepInEx's DLL override in the game's Proton prefix. If you say yes, **no Steam launch options are needed**, and any you already have (e.g. `gamemoderun %command%`) keep working.

If you say no, or the prefix can't be used, the installer prints the launch options to set instead:
```
WINEDLLOVERRIDES="winhttp=n,b" %command%
```
The prefix setting is lost if the prefix is deleted (e.g. Steam's *delete local content*). Re-run `./install.sh` to put it back. Re-running is always safe.

| Option | |
|---|---|
| `--launch-options` | Never touch the Proton prefix; always use launch options. |
| `--no-service` | No systemd service. The daemon is started by `~/.local/bin/unbeatable-launch.sh`, which then goes in the launch options. |
| `--game-dir DIR` | Use this game folder instead of searching. |
| `--bepinex-zip FILE` | Use an already-downloaded `BepInEx_win_x64_5.4.23.5.zip`. |
| `-y` | Don't ask; say yes. |

### From source

Needs Go 1.23+, the .NET SDK (8 or newer) and `make`. The game must be installed too: the plugin compiles against its DLLs.

```bash
make release    # dist/better-unbeatable-rpc-<version>-linux-x86_64.tar.gz
make install    # builds the release, then runs its install.sh
```

Add `GAME_DIR=/path/to/UNBEATABLE` if the game isn't in the default Steam library. To build the plugin with plain `dotnet build` or an IDE, copy `plugin/Directory.Build.props.example` to `plugin/Directory.Build.props` and set `GameDir` in it.

## Configuration

**Plugin:** `<game>/BepInEx/config/xyz.diaduck.unbeatable-rpc.cfg`, created on first launch.

| Key | Default | |
|---|---|---|
| `UdpPort` | `47820` | Must match the daemon's `--port`. |
| `DisableGameDiscord` | `true` | Stop the game's built-in Discord integration. |
| `DebugLogging` | `false` | Log scenes and every message to `BepInEx/LogOutput.log`. |
| `LastKnownStars` | (managed) | The last star rating seen, so the menus can show it before the game recalculates it. |

**Daemon:** flags or environment variables.
- With the service, add them with `systemctl --user edit unbeatable-rpc`, for example:
  ```ini
  [Service]
  Environment=UNBEATABLE_RPC_VERBOSE=1
  ```
- With the launch wrapper, put flags in `UNBEATABLE_RPC_ARGS` in the launch options.

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
- Look at the daemon's log: `journalctl --user -u unbeatable-rpc` with the service, or `~/.local/state/unbeatable-rpc/daemon.log` with the launch wrapper.
  - `connected to …` and `activity set: …` mean the daemon side is fine.
  - `cannot reach Discord` means no socket was found; see below.
- No `game is running` line means the plugin isn't sending. Check `<game>/BepInEx/LogOutput.log` for `UnbeatableRPC … loaded`.
  - If that file doesn't exist, BepInEx isn't loading. Re-run `./install.sh`, or use the `WINEDLLOVERRIDES` launch option.

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
./uninstall.sh          # removes the plugin, the daemon and the service
./uninstall.sh --all    # also removes BepInEx and restores the Proton prefix, if the installer added them
```

Then remove this mod's parts from UNBEATABLE's launch options, if you set any. None of the game's own files are modified, so nothing else needs restoring.

## Development

- `plugin/` is the BepInEx 5 plugin (C#, `netstandard2.1`). It references the game's DLLs from `GameDir` and never copies them into the repo.
- `daemon/` is the Go daemon (standard library only):
  - `internal/discordipc`: the socket protocol;
  - `internal/presence`: state → activity, and the update loop;
  - `cmd/fakegame`: the test sender.
- `scripts/`: the installer (`install.sh`, which also uninstalls), the systemd unit, and the launch wrapper.

Wire format, plugin → daemon (UDP, one JSON object per datagram; unknown fields are omitted):

```json
{"v":1,"state":"idle|arcade|story|results|closed","song":"…","artist":"…","difficulty":"…","level":20,
 "chapter":"Chapter 4","highScore":1155577,"accuracy":0.842,"rank":"B","stars":8.703,"paused":false,"startedAt":1790860862}
```

---

Unofficial fan project, not affiliated with D-CELL GAMES. UNBEATABLE is a trademark of its owners :)