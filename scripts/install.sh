#!/usr/bin/env bash
set -euo pipefail

readonly APP_ID=2240620
readonly BEPINEX_VERSION=5.4.23.5
readonly BEPINEX_URL="https://github.com/BepInEx/BepInEx/releases/download/v${BEPINEX_VERSION}/BepInEx_win_x64_${BEPINEX_VERSION}.zip"
readonly BEPINEX_SHA256=82f9878551030f54657792c0740d9d51a09500eeae1fba21106b0c441e6732c4
readonly BEPINEX_ROOT_FILES=(winhttp.dll doorstop_config.ini .doorstop_version changelog.txt)
readonly OVERRIDE_SECTION='[Software\\Wine\\DllOverrides]'
readonly OVERRIDE_VALUE='native,builtin'
readonly PLUGIN_CONFIG=xyz.diaduck.unbeatable-rpc.cfg

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
bin_dir="$HOME/.local/bin"
unit_dir="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
state_dir="${XDG_STATE_HOME:-$HOME/.local/state}/unbeatable-rpc"
state_file="$state_dir/install.env"

mode=install
game_dir=""
download_dir=""
bepinex_zip=""
use_prefix=ask
use_service=auto
remove_all=false
assume_yes=false

usage() {
    cat <<EOF
Usage: ./install.sh [options]
       ./install.sh --uninstall [--all]

Installs BepInEx, the UnbeatableRPC plugin and the unbeatable-rpc daemon for UNBEATABLE.

  --game-dir DIR       UNBEATABLE install folder (default: found via Steam's libraries)
  --launch-options     don't touch the Proton prefix; print Steam launch options instead
  --no-service         don't install the systemd user service; use the launch wrapper instead
  --bepinex-zip FILE   use a downloaded BepInEx_win_x64_${BEPINEX_VERSION}.zip instead of fetching it
  --uninstall          remove the plugin, daemon and service
  --all                with --uninstall: also remove BepInEx and the Proton prefix override,
                       if this installer added them
  -y, --yes            don't ask questions
  -h, --help           show this help
EOF
}

say() { printf '%s\n' "$*"; }
step() { printf '\n==> %s\n' "$*"; }
die() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }

confirm() {
    $assume_yes && return 0
    local reply
    read -r -p "$1 [Y/n] " reply || return 1
    [[ -z "$reply" || "$reply" =~ ^[Yy] ]]
}

steam_roots() {
    local root
    for root in "$HOME/.local/share/Steam" "$HOME/.steam/steam" "$HOME/.var/app/com.valvesoftware.Steam/.local/share/Steam"; do
        [[ -f "$root/steamapps/libraryfolders.vdf" ]] && realpath "$root"
    done | sort -u
}

steam_libraries() {
    local root
    while IFS= read -r root; do
        printf '%s\n' "$root"
        sed -n 's/^[[:space:]]*"path"[[:space:]]*"\(.*\)"[[:space:]]*$/\1/p' "$root/steamapps/libraryfolders.vdf"
    done < <(steam_roots) | sed 's/\\\\/\\/g' | sort -u
}

find_game() {
    local lib install_dir
    while IFS= read -r lib; do
        if [[ -f "$lib/steamapps/appmanifest_${APP_ID}.acf" ]]; then
            install_dir="$(sed -n 's/^[[:space:]]*"installdir"[[:space:]]*"\(.*\)"[[:space:]]*$/\1/p' "$lib/steamapps/appmanifest_${APP_ID}.acf")"
            printf '%s\n' "$lib/steamapps/common/$install_dir"
            return 0
        fi
    done < <(steam_libraries)
    return 1
}

prefix_registry() {
    local lib
    lib="$(cd "$1/../../.." 2>/dev/null && pwd)" || return 0
    printf '%s\n' "$lib/steamapps/compatdata/${APP_ID}/pfx/user.reg"
}

game_running() { pgrep -x UNBEATABLE.exe >/dev/null 2>&1; }

prefix_in_use() {
    local pid
    for pid in $(pgrep -x wineserver 2>/dev/null); do
        grep -qz "compatdata/${APP_ID}" "/proc/$pid/environ" 2>/dev/null && return 0
    done
    return 1
}

extract_zip() {
    local zip="$1" dest="$2"
    if command -v unzip >/dev/null; then
        unzip -qn "$zip" -d "$dest"
    elif command -v bsdtar >/dev/null; then
        bsdtar -xkf "$zip" -C "$dest"
    elif command -v python3 >/dev/null; then
        python3 - "$zip" "$dest" <<'PY'
import os, sys, zipfile
with zipfile.ZipFile(sys.argv[1]) as z:
    for m in z.infolist():
        if not os.path.exists(os.path.join(sys.argv[2], m.filename)):
            z.extract(m, sys.argv[2])
PY
    else
        die "need unzip, bsdtar or python3 to extract BepInEx"
    fi
}

registry_override() {
    SEC="$OVERRIDE_SECTION" awk '
        BEGIN { sec = ENVIRON["SEC"] }
        index($0, sec) == 1 { inside = 1; next }
        inside && /^$/ { exit }
        inside && index($0, "\"winhttp\"=") == 1 { sub(/^"winhttp"="/, ""); sub(/"$/, ""); print; exit }
    ' "$1"
}

set_registry_override() {
    local reg="$1" value="$2" tmp
    tmp="$(mktemp "${reg}.XXXXXX")"
    SEC="$OVERRIDE_SECTION" awk -v value="$value" -v now="$(date +%s)" '
        BEGIN { sec = ENVIRON["SEC"]; line = "\"winhttp\"=\"" value "\"" }
        NR == FNR {
            if (index($0, sec) == 1) { has_sec = 1; scan = 1; next }
            if (scan && /^$/) scan = 0
            if (scan && index($0, "\"winhttp\"=") == 1) has_key = 1
            next
        }
        index($0, sec) == 1 { print; inside = 1; next }
        inside && /^#/ { print; next }
        inside {
            if (has_key) {
                if (index($0, "\"winhttp\"=") == 1) { if (value != "") print line; inside = 0; next }
                if (/^$/) inside = 0
                print; next
            }
            if (value != "") print line
            inside = 0; print; next
        }
        { print }
        END {
            if (value == "") exit
            if (!has_sec) { print ""; print sec " " now; print line }
            else if (inside && !has_key) print line
        }
    ' "$reg" "$reg" >"$tmp"
    cat "$tmp" >"$reg"
    rm -f "$tmp"
}

load_state() {
    BEPINEX_INSTALLED=false PREFIX_REG="" PREFIX_PREVIOUS="" SERVICE=false GAME_DIR=""
    # shellcheck disable=SC1090
    [[ -f "$state_file" ]] && source "$state_file"
    [[ -n "$game_dir" ]] || game_dir="$GAME_DIR"
}

write_state() {
    mkdir -p "$state_dir"
    {
        printf 'GAME_DIR=%q\n' "$game_dir"
        printf 'BEPINEX_INSTALLED=%q\n' "$BEPINEX_INSTALLED"
        printf 'PREFIX_REG=%q\n' "$PREFIX_REG"
        printf 'PREFIX_PREVIOUS=%q\n' "$PREFIX_PREVIOUS"
        printf 'SERVICE=%q\n' "$SERVICE"
    } >"$state_file"
}

locate_game() {
    step "Finding UNBEATABLE"
    if [[ -z "$game_dir" ]]; then
        game_dir="$(find_game)" || die "couldn't find UNBEATABLE (app $APP_ID) in any Steam library; pass --game-dir"
    fi
    [[ -f "$game_dir/UNBEATABLE.exe" ]] || die "no UNBEATABLE.exe in $game_dir"
    say "Game: $game_dir"
    if game_running; then die "UNBEATABLE is running; quit it first"; fi
}

install_bepinex() {
    step "BepInEx"
    if [[ -f "$game_dir/BepInEx/core/BepInEx.dll" && -f "$game_dir/winhttp.dll" ]]; then
        say "Already installed, leaving it as is."
        return
    fi
    download_dir="$(mktemp -d)"
    trap 'rm -rf "$download_dir"' EXIT
    if [[ -z "$bepinex_zip" ]]; then
        bepinex_zip="$download_dir/bepinex.zip"
        say "Downloading BepInEx $BEPINEX_VERSION"
        if command -v curl >/dev/null; then
            curl -fsSL -o "$bepinex_zip" "$BEPINEX_URL"
        elif command -v wget >/dev/null; then
            wget -q -O "$bepinex_zip" "$BEPINEX_URL"
        else
            die "need curl or wget to download BepInEx (or pass --bepinex-zip)"
        fi
    fi
    echo "$BEPINEX_SHA256  $bepinex_zip" | sha256sum -c --quiet - ||
        die "$bepinex_zip isn't BepInEx_win_x64_${BEPINEX_VERSION}.zip (checksum mismatch)"
    extract_zip "$bepinex_zip" "$game_dir"
    BEPINEX_INSTALLED=true
    say "Installed BepInEx $BEPINEX_VERSION into the game folder."
}

install_plugin_and_daemon() {
    step "Plugin and daemon"
    install -Dm644 "$here/UnbeatableRPC.dll" "$game_dir/BepInEx/plugins/UnbeatableRPC/UnbeatableRPC.dll"
    install -Dm755 "$here/unbeatable-rpc" "$bin_dir/unbeatable-rpc"
    install -Dm755 "$here/unbeatable-launch.sh" "$bin_dir/unbeatable-launch.sh"
    say "Installed the plugin into the game's BepInEx/plugins/ and the daemon into $bin_dir."

    if [[ "$use_service" == auto ]] && systemctl --user show-environment >/dev/null 2>&1; then
        install -Dm644 "$here/unbeatable-rpc.service" "$unit_dir/unbeatable-rpc.service"
        systemctl --user daemon-reload
        systemctl --user enable unbeatable-rpc.service >/dev/null 2>&1
        systemctl --user restart unbeatable-rpc.service
        SERVICE=true
        say "Enabled the unbeatable-rpc user service (starts at login)."
    elif ! $SERVICE; then
        [[ "$use_service" == no ]] || say "No systemd user session found."
        say "The daemon will be started by the launch wrapper instead."
    fi
}

configure_prefix() {
    step "Loading BepInEx"
    local reg
    reg="$(prefix_registry "$game_dir")"
    if [[ "$use_prefix" == no ]]; then
        say "Skipping the Proton prefix (--launch-options)."
        return 1
    elif [[ -z "$reg" || ! -f "$reg" ]]; then
        say "The game's Proton prefix doesn't exist yet. Launch the game once and re-run to skip launch options."
        return 1
    elif [[ "$(registry_override "$reg")" == "$OVERRIDE_VALUE" ]]; then
        say "The Proton prefix already loads winhttp.dll natively."
        return 0
    elif prefix_in_use; then
        say "The game's Proton prefix is still in use. Re-run once it has fully closed to skip launch options."
        return 1
    fi

    say "BepInEx loads through winhttp.dll, and Proton can be told to use it in one of two ways:"
    say "  - a DLL override stored in the game's Proton prefix (nothing to add to Steam), or"
    say "  - WINEDLLOVERRIDES in the game's Steam launch options."
    say "The prefix setting is lost if the prefix is deleted (e.g. Steam's 'delete local content')."
    confirm "Store the override in the Proton prefix?" || return 1

    cp -p "$reg" "$reg.unbeatable-rpc.bak"
    [[ "$PREFIX_REG" == "$reg" ]] || PREFIX_PREVIOUS="$(registry_override "$reg")"
    set_registry_override "$reg" "$OVERRIDE_VALUE"
    PREFIX_REG="$reg"
    say "Done (backup: $reg.unbeatable-rpc.bak)."
}

print_launch_options() {
    local prefix_done="$1" opts=""
    step "Steam launch options"
    $prefix_done || opts='WINEDLLOVERRIDES="winhttp=n,b" '
    $SERVICE || opts+="$bin_dir/unbeatable-launch.sh "
    if [[ -z "$opts" ]]; then
        say "Nothing to add. Leave UNBEATABLE's launch options empty, or as they were (e.g. 'gamemoderun %command%')."
        say "If you set launch options for this mod before, remove its WINEDLLOVERRIDES and unbeatable-launch.sh parts."
    else
        say "Set UNBEATABLE's launch options (Properties -> General) to:"
        say ""
        say "    ${opts}%command%"
        say ""
        say "Other wrappers go before %command%, e.g. ${opts}gamemoderun %command%"
    fi
}

do_install() {
    [[ -f "$here/unbeatable-rpc" && -f "$here/UnbeatableRPC.dll" ]] ||
        die "unbeatable-rpc and UnbeatableRPC.dll must be next to install.sh (run it from the release folder)"
    load_state
    locate_game
    [[ -f "$game_dir/UNBEATABLE_Data/Managed/Assembly-CSharp.dll" ]] ||
        die "$game_dir isn't the Mono build this plugin was made for"
    install_bepinex
    install_plugin_and_daemon
    local prefix_done=false
    if configure_prefix; then prefix_done=true; fi
    write_state
    print_launch_options "$prefix_done"
    say ""
    say "Installed. Launch UNBEATABLE and your Discord status will follow along."
}

do_uninstall() {
    load_state
    [[ -n "$game_dir" ]] || game_dir="$(find_game)" || game_dir=""
    if [[ -n "$game_dir" ]] && game_running; then die "UNBEATABLE is running; quit it first"; fi

    step "Daemon and service"
    if [[ -f "$unit_dir/unbeatable-rpc.service" ]]; then
        systemctl --user disable --now unbeatable-rpc.service >/dev/null 2>&1 || true
        rm -f "$unit_dir/unbeatable-rpc.service"
        systemctl --user daemon-reload 2>/dev/null || true
        say "Removed the unbeatable-rpc user service."
    fi
    rm -f "$bin_dir/unbeatable-rpc" "$bin_dir/unbeatable-launch.sh"
    SERVICE=false
    say "Removed the daemon from $bin_dir."

    step "Plugin"
    if [[ -d "$game_dir" ]]; then
        rm -rf "$game_dir/BepInEx/plugins/UnbeatableRPC"
        rm -f "$game_dir/BepInEx/config/$PLUGIN_CONFIG"
        say "Removed the plugin and its config from $game_dir."
    else
        say "Couldn't find the game, so there's no plugin to remove (pass --game-dir if it moved)."
    fi

    if $remove_all; then
        step "BepInEx"
        if $BEPINEX_INSTALLED && [[ -d "$game_dir" ]]; then
            local f
            for f in "${BEPINEX_ROOT_FILES[@]}"; do rm -f "$game_dir/$f"; done
            rm -rf "$game_dir/BepInEx"
            BEPINEX_INSTALLED=false
            say "Removed BepInEx from the game folder."
        else
            say "BepInEx wasn't installed by this installer, leaving it."
        fi

        step "Proton prefix"
        if [[ -n "$PREFIX_REG" && -f "$PREFIX_REG" ]]; then
            if prefix_in_use; then die "the game's Proton prefix is in use; re-run once it has fully closed"; fi
            set_registry_override "$PREFIX_REG" "$PREFIX_PREVIOUS"
            say "Restored the winhttp override in $PREFIX_REG."
            PREFIX_REG="" PREFIX_PREVIOUS=""
        else
            say "The prefix wasn't changed by this installer, leaving it."
        fi
    fi

    if ! $BEPINEX_INSTALLED && [[ -z "$PREFIX_REG" ]]; then
        rm -f "$state_file"
    else
        write_state
    fi

    step "Steam launch options"
    say "Remove this mod's parts from UNBEATABLE's launch options, if you added any"
    say "(WINEDLLOVERRIDES=\"winhttp=n,b\" and unbeatable-launch.sh)."
    $remove_all || say "BepInEx is still installed; run with --all to remove it too."
}

while (($#)); do
    case "$1" in
        --game-dir) game_dir="${2:?--game-dir needs a path}"; shift ;;
        --launch-options) use_prefix=no ;;
        --no-service) use_service=no ;;
        --bepinex-zip) bepinex_zip="${2:?--bepinex-zip needs a file}"; shift ;;
        --uninstall) mode=uninstall ;;
        --all) remove_all=true ;;
        -y | --yes) assume_yes=true ;;
        -h | --help) usage; exit 0 ;;
        *) usage >&2; die "unknown option: $1" ;;
    esac
    shift
done

case "$mode" in
    install) do_install ;;
    uninstall) do_uninstall ;;
esac
