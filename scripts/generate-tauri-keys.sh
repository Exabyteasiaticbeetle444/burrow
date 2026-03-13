#!/bin/sh
set -e

# Generate a Tauri v2 updater signing keypair (Ed25519 / minisign format).
#
# Prerequisites: cargo-tauri CLI (cargo install tauri-cli)
#
# After generating:
#   1. Copy the PUBLIC key into tauri.conf.json -> plugins.updater.pubkey
#   2. Store the PRIVATE key as a GitHub Actions secret: TAURI_SIGNING_PRIVATE_KEY
#   3. Store the password as a GitHub Actions secret: TAURI_SIGNING_PRIVATE_KEY_PASSWORD

OUTPUT="${1:-./tauri-keys}"
PASSWORD="${2:-}"

if ! command -v cargo >/dev/null 2>&1; then
    echo "cargo is required. Install Rust first."
    exit 1
fi

if ! cargo tauri --version >/dev/null 2>&1; then
    echo "tauri-cli not found. Install with: cargo install tauri-cli"
    exit 1
fi

if [ -n "$PASSWORD" ]; then
    cargo tauri signer generate -p "$PASSWORD" -w "$OUTPUT" --ci --force
else
    cargo tauri signer generate -w "$OUTPUT" --force
fi

echo ""
echo "Keys written to:"
echo "  Private: $OUTPUT"
echo "  Public:  ${OUTPUT}.pub"
echo ""
echo "Next steps:"
echo "  1. Update web/client/src-tauri/tauri.conf.json with the public key"
echo "  2. Add TAURI_SIGNING_PRIVATE_KEY secret to GitHub repository"
echo "  3. Add TAURI_SIGNING_PRIVATE_KEY_PASSWORD secret to GitHub repository"
