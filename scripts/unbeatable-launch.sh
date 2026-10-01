#!/usr/bin/env bash

set -u

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
daemon="$script_dir/unbeatable-rpc"
[[ -x "$daemon" ]] || daemon="$(command -v unbeatable-rpc || true)"

log_dir="${XDG_STATE_HOME:-$HOME/.local/state}/unbeatable-rpc"
log_file="$log_dir/daemon.log"
mkdir -p "$log_dir"
[[ -f "$log_file" ]] && mv -f "$log_file" "$log_file.old"

daemon_pid=""
if [[ -n "$daemon" ]]; then
    # shellcheck disable=SC2086 # word splitting of extra args is intended
    env -u LD_PRELOAD "$daemon" ${UNBEATABLE_RPC_ARGS:-} >>"$log_file" 2>&1 &
    daemon_pid=$!
else
    echo "unbeatable-launch: unbeatable-rpc not found; starting the game without Rich Presence" >>"$log_file"
fi

stop_daemon() {
    if [[ -n "$daemon_pid" ]] && kill -0 "$daemon_pid" 2>/dev/null; then
        kill -TERM "$daemon_pid" 2>/dev/null
        for _ in 1 2 3 4 5 6 7 8 9 10; do
            kill -0 "$daemon_pid" 2>/dev/null || break
            sleep 0.2
        done
        kill -KILL "$daemon_pid" 2>/dev/null || true
    fi
}
trap stop_daemon EXIT
trap 'exit 143' TERM
trap 'exit 130' INT

# Not exec: the trap must run after the game exits.
"$@"
