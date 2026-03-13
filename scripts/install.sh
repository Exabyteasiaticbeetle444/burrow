#!/bin/sh
set -e

REPO="FrankFMY/burrow"
BINARY="burrow-server"

sha256_cmd() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | cut -d' ' -f1
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$1" | cut -d' ' -f1
    else
        echo "No sha256sum or shasum found. Cannot verify checksum." >&2
        exit 1
    fi
}

main() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$ARCH" in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
    esac

    case "$OS" in
        linux|darwin) ;;
        *) echo "Unsupported OS: $OS"; exit 1 ;;
    esac

    VERSION=$(curl -sL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | head -1 | cut -d'"' -f4)
    if [ -z "$VERSION" ]; then
        echo "Failed to fetch latest version"
        exit 1
    fi

    TARBALL="${BINARY}_${OS}_${ARCH}.tar.gz"
    URL="https://github.com/$REPO/releases/download/$VERSION/$TARBALL"
    CHECKSUMS_URL="https://github.com/$REPO/releases/download/$VERSION/checksums.txt"
    echo "Downloading $BINARY $VERSION for $OS/$ARCH..."

    TMP=$(mktemp -d)
    TARBALL_PATH="$TMP/$TARBALL"
    CHECKSUMS_PATH="$TMP/checksums.txt"

    curl -sfL "$URL" -o "$TARBALL_PATH" || { echo "Failed to download $URL"; rm -rf "$TMP"; exit 1; }
    curl -sfL "$CHECKSUMS_URL" -o "$CHECKSUMS_PATH" || { echo "Failed to download checksums.txt"; rm -rf "$TMP"; exit 1; }

    EXPECTED=$(grep "$TARBALL" "$CHECKSUMS_PATH" | cut -d' ' -f1)
    if [ -z "$EXPECTED" ]; then
        echo "Checksum for $TARBALL not found in checksums.txt"
        rm -rf "$TMP"
        exit 1
    fi

    ACTUAL=$(sha256_cmd "$TARBALL_PATH")
    if [ "$ACTUAL" != "$EXPECTED" ]; then
        echo "Checksum verification failed!"
        echo "  Expected: $EXPECTED"
        echo "  Actual:   $ACTUAL"
        rm -rf "$TMP"
        exit 1
    fi
    echo "Checksum verified."

    tar xz -C "$TMP" -f "$TARBALL_PATH"

    INSTALL_DIR="/usr/local/bin"
    if [ "$(id -u)" -ne 0 ]; then
        INSTALL_DIR="$HOME/.local/bin"
        mkdir -p "$INSTALL_DIR"
    fi

    mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
    chmod +x "$INSTALL_DIR/$BINARY"
    rm -rf "$TMP"

    echo "$BINARY $VERSION installed to $INSTALL_DIR/$BINARY"

    if [ "$(id -u)" -eq 0 ] && command -v systemctl >/dev/null 2>&1; then
        if [ ! -f /etc/systemd/system/burrow-server.service ]; then
            cat > /etc/systemd/system/burrow-server.service <<'UNIT'
[Unit]
Description=Burrow VPN Server
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/burrow-server run
Restart=on-failure
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
UNIT
            systemctl daemon-reload
            echo "Systemd service installed. Run: systemctl enable --now burrow-server"
        fi
    fi

    if [ ! -f /etc/burrow/burrow-server.json ]; then
        echo ""
        echo "Initialize the server:"
        echo "  $INSTALL_DIR/$BINARY init --password <password> --server <your-ip>"
    fi
}

main "$@"
