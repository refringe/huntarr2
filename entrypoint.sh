#!/bin/sh
set -e

# The image creates huntarr2 with UID/GID 1000; when PUID/PGID request different IDs, remap them before anything
# else.

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

# When ENCRYPTION_KEY is unset, read a previously generated key from /config, or generate one and persist it there.

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

chown -R huntarr2:huntarr2 /config

exec su-exec huntarr2 /bin/huntarr2
