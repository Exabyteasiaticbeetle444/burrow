# Burrow

The fastest, most private, and easiest to use VPN & proxy for censorship circumvention.

**Deploy a server in one command. Share access with a link. Connect in one click.**

## What is Burrow?

Burrow is a self-hosted VPN/proxy system designed for people living under internet censorship. It combines military-grade traffic camouflage with dead-simple UX.

- **Undetectable** — VLESS+Reality makes your traffic look like normal HTTPS to any website. DPI cannot distinguish it from legitimate traffic.
- **Fast** — WireGuard for non-censored networks, Hysteria 2 (QUIC) for lossy mobile connections, automatic protocol selection.
- **Simple** — Server deploys in one command. Users connect by scanning a QR code or pasting a link. Zero configuration.
- **Private** — Self-hosted. You control the server. No logs by default. No telemetry. No third parties.

## Quick Start

### Server (on your VPS)

```bash
curl -sL https://get.burrow.sh | sh
```

### Client

```bash
# CLI
burrow connect "burrow://connect/..."

# GUI
burrow gui
```

Or download the desktop app from [Releases](https://github.com/FrankFMY/burrow/releases).

## Protocols

| Protocol | Port | Use Case |
|----------|------|----------|
| VLESS+Reality | 443/TCP | Primary — camouflaged as real HTTPS, undetectable by DPI |
| Hysteria 2 | 8443/UDP | Mobile/lossy networks — QUIC-based, fast handshake |
| Shadowsocks 2022 | 8388/TCP | Proven fallback — AEAD encryption |
| WireGuard | 51820/UDP | Maximum speed — for non-censored environments |

The client automatically selects the best working protocol. If one is blocked, it falls back to the next.

## Features

- **One-command server deploy** with interactive setup
- **QR code / link sharing** — invite friends and family
- **Auto protocol selection** with intelligent fallback
- **Kill switch** — blocks all traffic if VPN disconnects
- **DNS leak prevention** — all DNS through encrypted tunnel
- **Split tunneling** — bypass VPN for specific apps/sites
- **Admin dashboard** — monitor clients, bandwidth, manage invites
- **Cross-platform** — Linux, macOS, Windows

## Architecture

```
Server (VPS)                          Client (your device)
┌─────────────────────┐              ┌─────────────────────┐
│ Management API      │              │ Control UI          │
│ Transport Engine    │◄────────────►│ Tunnel Engine       │
│ Admin Dashboard     │  encrypted   │ Kill Switch         │
│ SQLite DB           │  tunnel      │ DNS Proxy           │
└─────────────────────┘              └─────────────────────┘
```

## Building from Source

```bash
# Prerequisites: Go 1.22+, Bun (for frontend)

# Build everything
make build

# Run server
./dist/burrow-server init
./dist/burrow-server start

# Run client
./dist/burrow connect "burrow://connect/..."
```

## License

Apache License 2.0 — see [LICENSE](LICENSE).

## Author

**Прянишников Артём Алексеевич**
- Email: Pryanishnikovartem@gmail.com
- Telegram: [@FrankFMY](https://t.me/FrankFMY)
- GitHub: [@FrankFMY](https://github.com/FrankFMY)
