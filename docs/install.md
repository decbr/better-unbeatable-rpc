# Install

## Windows (experimental)

You need UNBEATABLE on Steam and the Discord desktop app.

1. Download the latest `-windows-x86_64.zip` from [releases](https://github.com/decbr/better-unbeatable-rpc/releases) and extract it.
2. Quit the game, then double-click **install.cmd**. If Windows warns about an unrecognised app, choose *More info → Run anyway*: the programs aren't code-signed.
3. Launch UNBEATABLE.

The installer:

- finds the game through Steam's libraries, and installs [BepInEx 5](https://github.com/BepInEx/BepInEx) into it if it isn't there (checksum-checked)
- copies the plugin into `BepInEx\plugins`
- puts the daemon and the settings app in `%LOCALAPPDATA%\Programs\unbeatable-rpc`
- starts the daemon, and at every login from then on
- adds **UNBEATABLE RPC** to the Start menu

No launch options are needed. Re-running **install.cmd** is always safe, and is how you update.

If it says it can't write to the game folder, right-click **install.cmd** → *Run as administrator*.

| | |
|---|---|
| Settings | `%APPDATA%\unbeatable-rpc\config.json` |
| Daemon log | `%LOCALAPPDATA%\unbeatable-rpc\daemon.log`, also on the app's **Daemon** tab |
| Options | `install.cmd -GameDir <folder>`, `-BepInExZip <file>`, `-Yes` |

### Uninstall

Open **UNBEATABLE RPC**, then Advanced → **Uninstall…**. Or double-click **uninstall.cmd** in the release folder; `uninstall.cmd -All` also removes BepInEx, if the installer added it. None of the game's own files are modified.

## Linux (best)

You need UNBEATABLE on Steam, running under Proton, and [Vesktop](https://github.com/Vencord/Vesktop) with **arRPC** on, or the official Discord client. The settings app needs WebKitGTK 4.1 (`webkit2gtk-4.1` on Arch, `libwebkit2gtk-4.1-0` on Debian/Ubuntu); everything else works without it.

1. Download the latest `-linux-x86_64.tar.gz` from [releases](https://github.com/decbr/better-unbeatable-rpc/releases) and extract it.
2. Quit the game, then run:
   ```bash
   cd better-unbeatable-rpc-*-linux-x86_64
   ./install.sh
   ```
3. Launch UNBEATABLE.

The installer:

- finds the game (Flatpak Steam included), and installs [BepInEx 5](https://github.com/BepInEx/BepInEx) into it if it isn't there (checksum-checked)
- copies the plugin into `BepInEx/plugins`, and the daemon and settings app into `~/.local/bin`
- sets the daemon up as a systemd user service that starts at login
- offers to configure the game's Proton prefix so **no launch options are needed**
- offers to open the settings app; after that it's **UNBEATABLE RPC** in your app menu

If you say no to the prefix, set this as the game's launch options instead:
```
WINEDLLOVERRIDES="winhttp=n,b" %command%
```

Re-running `./install.sh` is always safe, e.g. after Steam's *delete local content* resets the prefix. Run `./install.sh --help` for options, such as `--game-dir`, or `--no-service` to start the daemon with the game through a launch wrapper instead of systemd.

| | |
|---|---|
| Settings | `~/.config/unbeatable-rpc/config.json` |
| Daemon log | `journalctl --user -u unbeatable-rpc`, also on the app's **Daemon** tab |

### Uninstall

Open **UNBEATABLE RPC**, then Advanced → **Uninstall…**. Or from a terminal:

```bash
unbeatable-rpc-uninstall          # removes the plugin, the daemon, the settings app and your settings
unbeatable-rpc-uninstall --all    # also removes BepInEx and the Proton prefix change
```

`./uninstall.sh` in the release folder does the same. Remove the launch options too, if you set any. None of the game's own files are modified.
