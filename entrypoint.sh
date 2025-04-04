#!/bin/sh
CONFIG_PATH="/app/config/config.yaml"
DEFAULT_CONFIG="/app/config.default.yaml"

# Jika config.yaml tidak ada di volume, salin dari bawaan image
if [ ! -f "$CONFIG_PATH" ]; then
    echo "[INFO] Config file not found, copying default config..."
    cp "$DEFAULT_CONFIG" "$CONFIG_PATH"
fi

# Jalankan aplikasi
exec "$@"
