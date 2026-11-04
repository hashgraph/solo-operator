#!/bin/sh
set -u

log() { echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) reloader: $*"; }

# kubelet atomically repoints each volume's ..data symlink when its content changes.
fingerprint() {
	for d in $WATCH_DIRS; do readlink "$d/..data"; done
}

send_reload() {
	if command -v nc >/dev/null 2>&1; then
		echo reload | nc -w 30 "$MASTER_CLI_HOST" "$MASTER_CLI_PORT"
	elif command -v bash >/dev/null 2>&1; then
		bash -c 'exec 3<>"/dev/tcp/$0/$1" || exit 1; echo reload >&3; timeout 30 cat <&3' "$MASTER_CLI_HOST" "$MASTER_CLI_PORT"
	else
		echo "neither nc nor bash is available"
		return 1
	fi
}

applied=$(fingerprint)
pending=""
log "watching ${WATCH_DIRS} every ${RELOAD_INTERVAL_SECONDS}s"
while sleep "$RELOAD_INTERVAL_SECONDS"; do
	current=$(fingerprint)
	if [ "$current" = "$applied" ]; then
		pending=""
		continue
	fi
	# Debounce: volumes updated together trigger one reload.
	if [ "$current" != "$pending" ]; then
		pending=$current
		continue
	fi
	pending=""
	out=$(send_reload 2>&1)
	case "$out" in
	Success=1*)
		log "reload succeeded"
		applied=$current
		;;
	Success=0*)
		log "reload rejected, previous config keeps serving"
		applied=$current
		;;
	*)
		log "master CLI unreachable, retrying"
		;;
	esac
	[ -n "$out" ] && printf '%s\n' "$out"
done
