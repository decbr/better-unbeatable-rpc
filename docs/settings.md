# Settings

The settings app writes everything to one file, and the daemon picks up changes within a couple of seconds. You can edit it by hand too.

| | |
|---|---|
| Windows | `%APPDATA%\unbeatable-rpc\config.json` |
| Linux | `~/.config/unbeatable-rpc/config.json` |

```json
{"style": "spotify"}
```

A broken file never stops the daemon: it keeps the settings it had, and the settings app shows what's wrong.

## Keys

| Key | Flag | Environment variable | Default |
|---|---|---|---|
| `style` | `--style` | `UNBEATABLE_RPC_STYLE` | `detailed`. A built-in or custom style's name; see [Styles](styles.md) |
| `customStyles` | | | Styles made in the app |
| `hidden` | | `UNBEATABLE_RPC_HIDDEN` | `false`. Keeps the game off Discord without closing it: the app's **Show on Discord** box |
| `largeImage` | `--large-image` | `UNBEATABLE_RPC_LARGE_IMAGE` | `logo` (`drp_icon` with the game's own client ID) |
| `clientId` | `--client-id` | `UNBEATABLE_RPC_CLIENT_ID` | `1555205185792516146`, the "UNBEATABLE" Discord app. The game's own is `892184493899804682` |
| `socket` | `--socket` | `UNBEATABLE_RPC_SOCKET` | Searched for. A Discord socket (Linux) or pipe (Windows) path |
| `verbose` | `--verbose` | `UNBEATABLE_RPC_VERBOSE` | `false`. Logs every message and activity |
| | `--port` | `UNBEATABLE_RPC_PORT` | `47820`. Must match the mod's `UdpPort` |
| | `--status-port` | `UNBEATABLE_RPC_STATUS_PORT` | `47821`. `0` turns the status endpoint off |
| | `--config` | | The file above |

Environment variables win over the file, and flags win over both. The settings app locks anything they set. Run `unbeatable-rpc -h` for the full list.

## The mod's settings

The plugin has its own file, `<game>/BepInEx/config/xyz.diaduck.unbeatable-rpc.cfg`, created on the first launch. The settings app edits the first two, while the game is closed.

| Key | Default | |
|---|---|---|
| `DisableGameDiscord` | `true` | Stops the game's built-in Discord integration ("UNBEATABLE [white label]") |
| `DebugLogging` | `false` | Logs scenes and every message to `BepInEx/LogOutput.log` |
| `UdpPort` | `47820` | Must match the daemon's `--port` |
