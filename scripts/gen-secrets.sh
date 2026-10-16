#!/usr/bin/env bash
# =============================================================================
# gen-secrets.sh - Isi JWT_SECRET & JWT_REFRESH_SECRET di .env dengan nilai acak.
# Jika .env belum ada, file dibuat dari .env.example.
#
# Pemakaian (dari root repo):
#   cp .env.example .env
#   ./scripts/gen-secrets.sh
# =============================================================================
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
env_path="$root/.env"
example_path="$root/.env.example"

if [ ! -f "$env_path" ]; then
    if [ ! -f "$example_path" ]; then
        echo "Tidak menemukan .env.example di $root" >&2
        exit 1
    fi
    cp "$example_path" "$env_path"
    echo "Membuat .env dari .env.example"
fi

gen_secret() {
    head -c 48 /dev/urandom | base64 | tr -d '\n'
}

access_secret="$(gen_secret)"
refresh_secret="$(gen_secret)"

tmp="$(mktemp)"
awk -v a="$access_secret" -v r="$refresh_secret" '
    /^[[:space:]]*JWT_SECRET[[:space:]]*=/         { print "JWT_SECRET=" a; sa=1; next }
    /^[[:space:]]*JWT_REFRESH_SECRET[[:space:]]*=/ { print "JWT_REFRESH_SECRET=" r; sr=1; next }
    { print }
    END { if (!sa) print "JWT_SECRET=" a; if (!sr) print "JWT_REFRESH_SECRET=" r }
' "$env_path" > "$tmp"
mv "$tmp" "$env_path"

echo "Secret JWT ditulis ke .env ($env_path)"
