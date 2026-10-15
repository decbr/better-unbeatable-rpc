# better-unbeatable-rpc

Discord Rich Presence for [UNBEATABLE](https://store.steampowered.com/app/2240620/UNBEATABLE/) on Linux, and on [Windows](#windows) too (experimental).

| Where you are | Top line | Second line |
|---|---|---|
| Arcade song | **RE-TUNED - peak divide ft. Rachel Lake** | UNBEATABLE Lv 20 · PB: 1,155,577 (B) |
| Paused | **RE-TUNED - peak divide ft. Rachel Lake (paused)** | same |
| Results | **Results: RE-TUNED - peak divide ft. Rachel Lake** | same, PB updated if you just beat it |
| Story mode | **Story mode - Chapter 4** | ★ 8.703 |
| Menus | **In menus** | ★ 8.703 (your star rating) |

The timer shows how long the game has been open, in every state and every style.

The PB is for the board you're playing on: Classic, Double Time or Half Time. Prefer something simpler? See [Styles](#styles).

## How it works

```
UNBEATABLE (Proton)                         Linux
┌─────────────────────────┐   UDP 47820   ┌──────────────────┐  discord-ipc-0  ┌─────────┐
│ BepInEx + UnbeatableRPC ├─────────────▶│ unbeatable-rpc   ├───────────────▶│ Vesktop │
│ (reads game state)      │  JSON, local  │ (daemon, Go)     │   Unix socket   │ (arRPC) │
└─────────────────────────┘               └──────────────────┘                 └─────────┘
```

A game under Proton can't reach a Linux Discord socket, so a BepInEx plugin sends the game's state to a small native daemon, which updates Discord. The plugin also turns off the game's own leftover Discord integration ("UNBEATABLE [white label]").

## Requirements

- UNBEATABLE on Steam, running under Proton.
- [Vesktop](https://github.com/Vencord/Vesktop) with **arRPC** enabled, or the official Discord client.
- For the settings app only: WebKitGTK 4.1 (`webkit2gtk-4.1` on Arch, `libwebkit2gtk-4.1-0` on Debian/Ubuntu).

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
At the end it offers to open the settings app; after that it's **UNBEATABLE RPC** in your app menu.

Re-running `./install.sh` is always safe, e.g. after Steam's *delete local content* resets the prefix. Run `./install.sh --help` for options.

## Styles

| Style | Song | Story mode | Menus |
|---|---|---|---|
| `detailed` (default) | the table at the top | | |
| `spotify` | **RE-TUNED** / peak divide ft. Rachel Lake | **Story mode** / Chapter 4 | **In menus** / ★ 8.703 |
| `osu` | **peak divide - RE-TUNED (re-kouned) [UNBEATABLE]** / Doing crimes | **Story mode** / Chapter 4 | **In menus** / ★ 8.703 |

To switch, open **UNBEATABLE RPC** from your app menu and pick one on the **Styles** tab. It takes effect within a few seconds, no restart needed. The **Status** tab shows whether the game and Discord are detected, and what your status currently says.

### Custom styles

On the **Styles** tab, **New style** makes your own, or **Duplicate** starts from a built-in one. Each state (song, results, a song in story mode, story mode, menus) has two lines, built from pieces you drag in: plain text, or one of

`title` `artist` `charter` `difficulty` `level` `pb` `rank` `stars` `chapter`

Click a piece to give it text before and after, like ` - ` before `artist`. A piece is left out when its information is missing, along with that text, so a song with no artist doesn't end in a stray ` - `. It can show something else instead: `pb` with *No PB yet*.

**Edit as text** shows the lines as templates, which is what's saved: `{title}[ - {artist}]` is a title and an artist piece, square brackets marking what's left out together, and `[PB: {pb}|No PB yet]` shows what's after the `|` when there's no PB. Write `\[`, `\]`, `\{`, `\}`, `\|` or `\\` for the character itself. Results falls back to Song and a story-mode song to Story when left empty; Story and Menus fall back to `detailed`.

To share a style, **Copy** puts it on your clipboard as text to paste anywhere, and **Save…** writes it to a `.json` file. **Import** takes either: paste the text (a chat message around it is fine) or open the file. It opens in the editor so you can check it before saving.

Without the app, edit `~/.config/unbeatable-rpc/config.json`:
```json
{"style": "spotify"}
```
Custom styles are saved there too, under `customStyles`, and `style` can name one. The other settings there are `hidden` (keep the game off Discord without closing it, the app's **Show on Discord** box), `largeImage`, `clientId`, `socket` and `verbose`. `UNBEATABLE_RPC_*` environment variables and command-line flags win over the file; run `~/.local/bin/unbeatable-rpc -h` for the list.

## Troubleshooting

Open **UNBEATABLE RPC** first. Its **Setup** section checks BepInEx, the mod, the Proton prefix and the last launch, and says what to fix; the Status section explains Discord problems. **Copy diagnostics** puts everything needed for a bug report on your clipboard.

**Nothing shows up in Discord**
- Vesktop: Settings → *Rich Presence (arRPC)* must be on.
- Discord: User Settings → Activity Privacy → *Share your detected activities with others* must be on.
- Check the daemon's log on the app's **Daemon** tab (or `journalctl --user -u unbeatable-rpc`):
  - `activity set: …`: the daemon is working.
  - `cannot reach Discord`: no Discord socket found. Flatpak Vesktop is found automatically; otherwise set the socket path in the settings app (Advanced) to the one from `ss -xlp | grep discord-ipc`.
  - No `game is running`: the plugin isn't loading. Check `<game>/BepInEx/LogOutput.log` exists, and re-run `./install.sh` if not.

**Another app's status shows instead:** Discord only shows one "Playing" activity. Close or disable the other app's presence while you play.

**After a game update** a patch may stop working (`Failed to apply …` in `BepInEx/LogOutput.log`). The rest keep working until the mod is updated.

## Uninstall

Open **UNBEATABLE RPC**, then Advanced → **Uninstall…**. Or from a terminal:

```bash
unbeatable-rpc-uninstall          # removes the plugin, the daemon, the settings app and your settings
unbeatable-rpc-uninstall --all    # also removes BepInEx and the Proton prefix change
```

(`./uninstall.sh` in the release folder does the same.) Remove the launch options too, if you set any. None of the game's own files are modified.

## Windows

**Experimental:** it's built and tested from Linux and hasn't been confirmed on a real Windows install yet. If something doesn't work, open an issue with the settings app's **Copy diagnostics** output.

It works the same way, with Discord's named pipe in place of the socket and no Proton. You need UNBEATABLE on Steam and the Discord desktop app.

1. Download the latest `-windows-x86_64.zip` from [releases](https://github.com/decbr/better-unbeatable-rpc/releases) and extract it.
2. Quit the game, then double-click **install.cmd**. If Windows warns about an unrecognised app, choose *More info → Run anyway*: the programs aren't code-signed.
3. Launch UNBEATABLE.

The installer finds the game, installs BepInEx if needed, puts the daemon and the settings app in `%LOCALAPPDATA%\Programs\unbeatable-rpc`, and starts the daemon at login. No launch options are needed. The settings app is **UNBEATABLE RPC** in the Start menu, and everything under [Styles](#styles) and [Troubleshooting](#troubleshooting) applies, with these differences:

- Settings are in `%APPDATA%\unbeatable-rpc\config.json`, and the daemon's log is `%LOCALAPPDATA%\unbeatable-rpc\daemon.log` (also on the app's **Daemon** tab).
- If the installer says it can't write to the game folder, right-click **install.cmd** → *Run as administrator*.
- To uninstall, use the app's Advanced → **Uninstall…**, or **uninstall.cmd** in the release folder (`uninstall.cmd -All` also removes BepInEx).

## Building from source

Needs Go 1.25+, the .NET SDK 8+, `make`, WebKitGTK 4.1 development files, and the game installed (the plugin compiles against its DLLs).

```bash
make install    # or GAME_DIR=/path/to/UNBEATABLE make install; NO_GUI=1 skips the settings app
```

The Windows build cross-compiles from Linux, with no cgo or Windows toolchain (`zip` is needed for the zip):

```bash
make test-windows-cross    # type-checks the Windows code, tests included
make release-windows       # dist/better-unbeatable-rpc-<version>-windows-x86_64.zip
```

On Windows itself, with Go 1.25+, GNU make (`winget install ezwinports.make`) and Git for Windows's `bin` folder on PATH, this builds and installs. Without the .NET SDK it uses the prebuilt plugin in `plugin/dist/`:

```bash
make install-windows    # INSTALL_ARGS=-Yes skips the questions
```

---

Unofficial fan project, not affiliated with D-CELL GAMES. UNBEATABLE is a trademark of its owners :)
