# better-unbeatable-rpc

Discord Rich Presence for [UNBEATABLE](https://store.steampowered.com/app/2240620/UNBEATABLE/) on Linux. \
Not Windows at the moment. Sorry! I don't have a Windows machine I can test with.

| Where you are | Top line | Second line |
|---|---|---|
| Arcade song | **RE-TUNED - peak divide ft. Rachel Lake** + elapsed timer | UNBEATABLE Lv 20 · PB: 1,155,577 (B) |
| Paused | **RE-TUNED - peak divide ft. Rachel Lake (paused)** | same, timer hidden |
| Results | **Results: RE-TUNED - peak divide ft. Rachel Lake** | same, PB updated if you just beat it |
| Story mode | **Story mode - Chapter 4** | ★ 8.703 |
| Menus | **In menus** | ★ 8.703 (your star rating) |

The PB is for the board you're playing on: Classic, Double Time or Half Time. Prefer something simpler? See [Styles](#styles).

## How it works

```
UNBEATABLE (Proton)                         Linux
┌─────────────────────────┐   UDP 47820   ┌──────────────────┐  discord-ipc-0  ┌─────────┐
│ BepInEx + UnbeatableRPC ├──────────────▶│ unbeatable-rpc   ├────────────────▶│ Vesktop │
│ (reads game state)      │  JSON, local  │ (daemon, Go)     │   Unix socket   │ (arRPC) │
└─────────────────────────┘               └──────────────────┘                 └─────────┘
```

A game under Proton can't reach a Linux Discord socket, so a BepInEx plugin sends the game's state to a small native daemon, which updates Discord. The plugin also turns off the game's own leftover Discord integration ("UNBEATABLE [white label]").

## Requirements

- UNBEATABLE on Steam, running under Proton.
- [Vesktop](https://github.com/Vencord/Vesktop) with **arRPC** enabled, or the official Discord client.

## Install

1. Download the latest `.tar.gz` from [releases](https://github.com/decbr/better-unbeatable-rpc/releases) and extract it.
2. Quit the game, then run:
   ```bash
   cd better-unbeatable-rpc-*-linux-x86_64
   ./install.sh
   ```
3. Launch UNBEATABLE.

The installer finds the game (Flatpak Steam included), installs [BepInEx 5](https://github.com/BepInEx/BepInEx) if needed, and sets the daemon up as a systemd user service. It offers to configure the game's Proton prefix so **no launch options are needed**. If you say no, set this as the game's launch options instead:
```
WINEDLLOVERRIDES="winhttp=n,b" %command%
```
Re-running `./install.sh` is always safe, e.g. after Steam's *delete local content* resets the prefix. Run `./install.sh --help` for options.

## Styles

| Style | Song | Story mode | Menus |
|---|---|---|---|
| `detailed` (default) | the table at the top | | |
| `spotify` | **RE-TUNED** / peak divide ft. Rachel Lake | **Story mode** / Chapter 4 | **In menus** / ★ 8.703 |

To switch:
```bash
mkdir -p ~/.config/systemd/user/unbeatable-rpc.service.d
printf '[Service]\nEnvironment=UNBEATABLE_RPC_STYLE=spotify\n' > ~/.config/systemd/user/unbeatable-rpc.service.d/override.conf
systemctl --user daemon-reload && systemctl --user restart unbeatable-rpc
```
Delete that file and run the last line again to go back.

Other daemon settings (`UNBEATABLE_RPC_SOCKET`, `UNBEATABLE_RPC_VERBOSE=1`) go in the same file. Run `~/.local/bin/unbeatable-rpc -h` for the full list.

## Troubleshooting

**Nothing shows up in Discord**
- Vesktop: Settings → *Rich Presence (arRPC)* must be on.
- Discord: User Settings → Activity Privacy → *Share your detected activities with others* must be on.
- Check `journalctl --user -u unbeatable-rpc`:
  - `activity set: …`: the daemon is working.
  - `cannot reach Discord`: no Discord socket found. Flatpak Vesktop is found automatically; otherwise set `UNBEATABLE_RPC_SOCKET` to the path from `ss -xlp | grep discord-ipc`.
  - No `game is running`: the plugin isn't loading. Check `<game>/BepInEx/LogOutput.log` exists, and re-run `./install.sh` if not.

**Another app's status shows instead:** Discord only shows one "Playing" activity. Close or disable the other app's presence while you play.

**After a game update** a patch may stop working (`Failed to apply …` in `BepInEx/LogOutput.log`). The rest keep working until the mod is updated.

## Uninstall

```bash
./uninstall.sh          # removes the plugin, the daemon and the service
./uninstall.sh --all    # also removes BepInEx and the Proton prefix change
```

Remove the launch options too, if you set any. None of the game's own files are modified.

## Building from source

Needs Go 1.23+, the .NET SDK 8+, `make`, and the game installed (the plugin compiles against its DLLs).

```bash
make install    # or GAME_DIR=/path/to/UNBEATABLE make install
```

---

Unofficial fan project, not affiliated with D-CELL GAMES. UNBEATABLE is a trademark of its owners :)
