# Changelog

## [0.2.0] - 2026-03-13

### Added
- **VPN mode (TUN)** — routes all system traffic through VPN, no proxy setup needed
- **Kill switch** — blocks all internet if VPN drops (Linux/macOS/Windows)
- **Auto-reconnect** — exponential backoff, up to 10 attempts, cancel anytime
- **Live speed stats** — real-time upload/download speed with total traffic counters
- **Server ping** — TCP latency measurement with color-coded badges
- **Server switching** — switch servers while connected without manual disconnect
- **Desktop notifications** — system notifications on connect and disconnect
- **Dynamic system tray** — menu reflects connection state, tooltip shows status
- **Auto-connect** — automatic connection on app launch
- **Deep links** — `burrow://connect/...` URLs to add servers from browser
- **Onboarding wizard** — first-run flow guides new users through setup
- **Localization** — English, Russian, Chinese (auto-detected from system locale)
- **Persistent preferences** — settings saved with visual confirmation
- **Error localization** — daemon errors translated to user's language
- **Admin auto-refresh** — dashboard stats update every 5 seconds
- **Inline confirmations** — no browser `confirm()` dialogs in admin dashboard
- **Server ping endpoint** — `GET /api/servers/:name/ping` in client daemon API

### Fixed
- Silent error swallowing in client store — now shows daemon connection errors
- Race condition in reconnect loop — ghost tunnel after disconnect
- Mutex unlock/relock gap in reconnect cancel path
- CORS restricted to localhost/Tauri origins (was wildcard `*`)
- Deep link scheme corrected to `burrow://connect/` to match Go invite format
- Preference save before connect no longer silently swallowed
- Server switch attempts reconnect to previous server on failure
- Tray polling thread now exits cleanly on app shutdown
- No false "Connection failed" notification on app quit
- Clipboard API properly awaited with error handling
- JSON encode errors logged in Go daemon

### Changed
- Version bumped to 0.2.0 across all packages
- Go version requirement updated to 1.26+
- Release workflow updated to Go 1.26
- Admin dashboard uses typed TypeScript interfaces (no `any`)

## [0.1.0] - 2026-03-10

### Added
- Initial release
- VLESS+Reality protocol with sing-box engine
- Hysteria2 and Shadowsocks 2022 fallback protocols
- Server with admin dashboard, invite system, SQLite storage
- Desktop client with Tauri 2
- Docker deployment support
- CI/CD with GitHub Actions
