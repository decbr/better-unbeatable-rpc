# Troubleshooting

Open **UNBEATABLE RPC** first. Its **Setup** section checks the daemon, BepInEx, the mod and the last launch (and the Proton prefix, on Linux), and says what to fix. The Status section explains Discord problems. **Copy diagnostics** puts everything needed for a bug report on your clipboard: paste it into a [GitHub issue](https://github.com/decbr/better-unbeatable-rpc/issues).

## Nothing shows up in Discord

- **Discord:** User Settings → Activity Privacy → *Share your detected activities with others* must be on.
- **Vesktop:** Settings → *Rich Presence (arRPC)* must be on.
- Check the daemon's log on the app's **Daemon** tab:
  - `activity set: …`: the daemon is working.
  - `cannot reach Discord`: no Discord socket or pipe found. Is Discord running? On Linux, Flatpak Vesktop is found automatically; otherwise set the socket path in the settings app (Advanced) to the one from `ss -xlp | grep discord-ipc`.
  - No `game is running`: the plugin isn't loading. Check `<game>/BepInEx/LogOutput.log` exists, and re-run the installer if not.

## Another app's status shows instead

Discord only shows one "Playing" activity. Close or disable the other app's presence while you play.

## After a game update

A patch may stop working (`Failed to apply …` in `BepInEx/LogOutput.log`). The rest keep working until the mod is updated.

## Linux: it stopped working after Steam deleted local content

That resets the Proton prefix, which held the setting that loads BepInEx. Re-run `./install.sh`.

## Windows: "Windows protected your PC"

The programs aren't code-signed. Choose *More info → Run anyway*.
