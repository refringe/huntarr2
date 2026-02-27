#!/bin/sh
set -e

# ── PUID / PGID remapping ───────────────────────────────────────────────
# Unraid convention: let users set PUID/PGID to match host filesystem
# ownership. The image creates huntarr2 with UID/GID 1000; if the
# requested IDs differ, remap them before anything else.

PUID="${PUID:-1000}"
PGID="${PGID:-1000}"

CURRENT_UID=$(id -u huntarr2)
CURRENT_GID=$(getent group huntarr2 | cut -d: -f3)

if [ "$PGID" != "$CURRENT_GID" ]; then
    groupmod -o -g "$PGID" huntarr2
fi

if [ "$PUID" != "$CURRENT_UID" ]; then
    usermod -o -u "$PUID" huntarr2
fi

# ── Encryption key auto-generation ──────────────────────────────────────
# If ENCRYPTION_KEY is not set, try to read a previously generated key
# from persistent storage. If no stored key exists, generate one and
# write it to /config so it survives container recreation.

KEY_FILE="/config/encryption.key"

if [ -z "$ENCRYPTION_KEY" ]; then
    if [ -f "$KEY_FILE" ]; then
        ENCRYPTION_KEY=$(cat "$KEY_FILE")
    else
        ENCRYPTION_KEY=$(head -c 32 /dev/urandom | od -A n -t x1 | tr -d ' \n')
        printf '%s' "$ENCRYPTION_KEY" > "$KEY_FILE"
        chmod 600 "$KEY_FILE"
        echo "Generated new encryption key at $KEY_FILE"
    fi
    export ENCRYPTION_KEY
fi

# ── Fix ownership and drop privileges ───────────────────────────────────

chown -R huntarr2:huntarr2 /config

exec su-exec huntarr2 /bin/huntarr2
