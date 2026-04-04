#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
ENV_FILE="$ROOT_DIR/.env"

random_hex() {
  openssl rand -hex 32
}

random_b64() {
  openssl rand -base64 32 | tr -d '\n'
}

if [ ! -f "$ENV_FILE" ]; then
  cat >"$ENV_FILE" <<EOF
FOUNDRY_PUBLISH_ADDR=8080
FOUNDRY_ADMIN_SESSION_SECRET=$(random_hex)
FOUNDRY_ADMIN_TOTP_SECRET_KEY=$(random_b64)
EOF
  echo "Created .env with local Docker secrets."
  exit 0
fi

append_if_missing() {
  key=$1
  value=$2
  if ! grep -q "^${key}=" "$ENV_FILE"; then
    printf '%s=%s\n' "$key" "$value" >>"$ENV_FILE"
  fi
}

append_if_missing "FOUNDRY_PUBLISH_ADDR" "8080"
append_if_missing "FOUNDRY_ADMIN_SESSION_SECRET" "$(random_hex)"
append_if_missing "FOUNDRY_ADMIN_TOTP_SECRET_KEY" "$(random_b64)"

echo ".env already existed; ensured required Docker variables are present."
