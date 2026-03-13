# Wave 1: Foundation — Make Everything Actually Work

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the 5 verified real problems: bandwidth tracking, Tauri updater security, install script integrity, test coverage gaps, and missing integration tests.

**Architecture:** Transport-level traffic accounting via sing-box ClashAPI TrafficManager already exists in client (`tunnel.go:Stats()`). Server needs equivalent: periodic sync of per-client bytes to SQLite. Tauri updater needs a signing keypair generated and pubkey set. Install script needs SHA256 verification from GoReleaser checksums.txt. Test coverage targets the 4 untested critical subsystems.

**Tech Stack:** Go 1.26, sing-box v1.13, SQLite, Tauri 2 (Rust), shell scripting, vitest

---

## Chunk 1: Bandwidth Tracking (Transport-Level)

### Task 1: Server-side per-client traffic accounting

The problem: `UpdateClient` exists in store but is never called. `bytes_up`/`bytes_down` are always 0. The bandwidth limit check in `handleConnect` is therefore useless.

**Files:**
- Modify: `internal/server/transport.go` — add traffic stats collection
- Modify: `internal/server/server.go` — add periodic sync goroutine
- Modify: `internal/server/store/sqlite.go` — verify UpdateClient works
- Test: `internal/server/store/sqlite_test.go` — add UpdateClient test
- Test: `internal/server/api_test.go` — add bandwidth limit enforcement test

- [ ] **Step 1: Write test for UpdateClient in store**

```go
func TestUpdateClient(t *testing.T) {
    s := setupTestStore(t)
    ctx := context.Background()

    client := &Client{
        ID: "test-update", Name: "update-test", Token: "tok-update",
        CreatedAt: time.Now().UTC(),
    }
    if err := s.CreateClient(ctx, client); err != nil {
        t.Fatal(err)
    }

    client.BytesUp = 1024
    client.BytesDown = 2048
    client.LastProtocol = "vless-reality"
    if err := s.UpdateClient(ctx, client); err != nil {
        t.Fatal(err)
    }

    got, err := s.GetClient(ctx, "test-update")
    if err != nil {
        t.Fatal(err)
    }
    if got.BytesUp != 1024 || got.BytesDown != 2048 {
        t.Errorf("bytes: got up=%d down=%d, want up=1024 down=2048", got.BytesUp, got.BytesDown)
    }
    if got.LastProtocol != "vless-reality" {
        t.Errorf("protocol: got %q, want vless-reality", got.LastProtocol)
    }
}
```

- [ ] **Step 2: Run test to verify it passes (UpdateClient already implemented)**

Run: `cd /home/user/projects/burrow && go test ./internal/server/store/ -run TestUpdateClient -v`
Expected: PASS (the function exists, just never called from production code)

- [ ] **Step 3: Write bandwidth limit enforcement test in api_test.go**

Add test that creates a client with BandwidthLimit=1000, updates BytesUp+BytesDown to 1000, then verifies handleConnect returns 403.

- [ ] **Step 4: Run test to verify it fails (bytes are never updated)**

Run: `go test ./internal/server/ -run TestBandwidthLimitEnforced -v`
Expected: Verify the test logic works with manual UpdateClient call in test setup

- [ ] **Step 5: Add traffic sync goroutine to server.go**

In `server.go`, after transport starts, launch a goroutine that every 30 seconds:
1. Gets active client tokens from transport (via sing-box connection tracking)
2. For each, calls `store.UpdateClient` with current bytes_up/bytes_down

Since sing-box doesn't expose per-user traffic (it uses UUID-based VLESS auth), we need to track at the inbound level. The practical approach: use the existing `CloseConnection` store method to record bytes when connections end, and add a `RecordTraffic(ctx, token string, bytesUp, bytesDown int64)` method that atomically increments counters.

- [ ] **Step 6: Implement RecordTraffic in store**

```go
func (s *SQLiteStore) RecordTraffic(ctx context.Context, token string, bytesUp, bytesDown int64) error {
    _, err := s.db.ExecContext(ctx,
        `UPDATE clients SET bytes_up = bytes_up + ?, bytes_down = bytes_down + ?, last_connected_at = datetime('now') WHERE token = ? AND revoked = 0`,
        bytesUp, bytesDown, token)
    return err
}
```

- [ ] **Step 7: Write test for RecordTraffic**

- [ ] **Step 8: Run all store tests**

Run: `go test ./internal/server/store/ -v`
Expected: All pass

- [ ] **Step 9: Commit**

```
feat: add per-client traffic accounting

RecordTraffic atomically increments bytes_up/bytes_down in SQLite.
Bandwidth limit enforcement now works because counters are updated.
```

### Task 2: Tauri updater signing keypair

**Files:**
- Modify: `web/client/src-tauri/tauri.conf.json` — set pubkey
- Create: `scripts/generate-tauri-keys.sh` — key generation helper
- Modify: `.github/workflows/release.yml` — use signing key from secret

- [ ] **Step 1: Generate Tauri signing keypair**

Run: `cd /home/user/projects/burrow/web/client && npx @tauri-apps/cli signer generate -w ../.. 2>&1` or equivalent to get a keypair.

If tauri CLI not available, generate manually: the pubkey goes in tauri.conf.json, private key goes in GitHub secret `TAURI_SIGNING_PRIVATE_KEY`.

- [ ] **Step 2: Set pubkey in tauri.conf.json**

Replace `"pubkey": ""` with the generated public key.

- [ ] **Step 3: Update release.yml to use signing key**

Add `TAURI_SIGNING_PRIVATE_KEY: ${{ secrets.TAURI_SIGNING_PRIVATE_KEY }}` to the Tauri build step env.

- [ ] **Step 4: Create helper script**

`scripts/generate-tauri-keys.sh` with instructions for regenerating keys.

- [ ] **Step 5: Commit**

```
security: add Tauri update signing keypair

Updates are now signed with Ed25519. The public key is in tauri.conf.json,
private key must be set as TAURI_SIGNING_PRIVATE_KEY GitHub secret.
```

### Task 3: Install script checksum verification

**Files:**
- Modify: `scripts/install.sh` — add SHA256 verification

- [ ] **Step 1: Read current install.sh**

- [ ] **Step 2: Add checksum verification**

After downloading the tarball, also download `checksums.txt` from the same release. Verify the tarball's SHA256 matches before extracting.

```bash
CHECKSUMS_URL="https://github.com/$REPO/releases/download/$VERSION/checksums.txt"
curl -sL "$CHECKSUMS_URL" -o "$TMP/checksums.txt"
EXPECTED=$(grep "${BINARY}_${OS}_${ARCH}.tar.gz" "$TMP/checksums.txt" | awk '{print $1}')
curl -sL "$URL" -o "$TMP/archive.tar.gz"
ACTUAL=$(sha256sum "$TMP/archive.tar.gz" | awk '{print $1}')
if [ "$EXPECTED" != "$ACTUAL" ]; then
    echo "ERROR: checksum mismatch"
    exit 1
fi
tar xz -C "$TMP" -f "$TMP/archive.tar.gz"
```

- [ ] **Step 3: Test the script locally**

- [ ] **Step 4: Commit**

```
security: verify SHA256 checksum in install script

Downloads checksums.txt from release and verifies tarball integrity
before extraction. Prevents MITM attacks on binary downloads.
```

## Chunk 2: Test Coverage

### Task 4: Test untested critical paths

**Files:**
- Modify: `internal/server/store/sqlite_test.go` — UpdateClient, RevokeClient edge cases
- Modify: `internal/server/api_test.go` — handleLogout, handleGetClient, handleHealthDetailed, handleGetLogs, login rate limiter, bandwidth limit
- Modify: `internal/client/daemon_test.go` — handleConnect when connected (409), handleDisconnect while reconnecting

- [ ] **Step 1: Add store edge case tests**

- RevokeClient on already-revoked client → ErrNotFound
- CreateClient with duplicate ID → error
- GetClientConnections with limit=0, limit=-1, limit=1001

- [ ] **Step 2: Run store tests**

Run: `go test ./internal/server/store/ -v`

- [ ] **Step 3: Add API endpoint tests**

- handleLogout → 200, verify token is blocked (ValidateToken returns error)
- handleGetClient found → 200 with client data
- handleGetClient not found → 404
- handleHealthDetailed → 200 with expected fields
- handleGetLogs → 200, verify returns log entries
- handleGetLogs with limit=-5 → returns default 100
- handleGetLogs with limit=999 → capped to 500
- Login rate limiter: 5 wrong attempts → 429 on 6th
- handleCreateInvite with bandwidth_limit=-1 → 400

- [ ] **Step 4: Run API tests**

Run: `go test ./internal/server/ -v -run "TestLogout|TestGetClient|TestHealthDetailed|TestGetLogs|TestRateLimit|TestBandwidth"`

- [ ] **Step 5: Add daemon edge case tests**

- handleConnect when already connected → 409
- handleDisconnect when not connected → 200 "not connected"

- [ ] **Step 6: Run daemon tests**

Run: `go test ./internal/client/ -v -run "TestHandle"`

- [ ] **Step 7: Commit**

```
test: add coverage for untested critical paths

Covers: UpdateClient, RevokeClient edge cases, all untested API
endpoints (logout, getClient, healthDetailed, getLogs, rate limiter),
daemon conflict handling.
```

### Task 5: Integration test — server + client connect

**Files:**
- Create: `internal/integration_test.go` — E2E test

- [ ] **Step 1: Design the integration test**

The test starts a real server (with in-memory SQLite), creates a client via API, then verifies that `/api/connect` returns valid protocol config. This doesn't require a live sing-box instance — it tests the full API flow.

```go
//go:build integration

func TestServerClientFlow(t *testing.T) {
    // 1. Generate config
    // 2. Create server with NewAPI
    // 3. Login → get JWT
    // 4. Create invite → get token
    // 5. POST /api/connect with token → verify response has protocols
    // 6. Verify client appears in /api/clients list
    // 7. Revoke client
    // 8. POST /api/connect again → expect 401
}
```

- [ ] **Step 2: Implement**

- [ ] **Step 3: Run**

Run: `go test ./internal/ -tags integration -run TestServerClientFlow -v`

- [ ] **Step 4: Commit**

```
test: add server-client integration test

Full API flow: login → create invite → connect → verify → revoke → reject.
Run with -tags integration.
```

## Chunk 3: Documentation & Release

### Task 6: Update CHANGELOG, bump version, release

- [ ] **Step 1: Update CHANGELOG.md with v0.5.1 section**

Security fixes and improvements from this session.

- [ ] **Step 2: Bump version to 0.5.1 in all files**

- web/client/src-tauri/tauri.conf.json
- web/client/package.json
- web/admin/package.json
- Cargo.toml

- [ ] **Step 3: Commit all changes**

- [ ] **Step 4: Tag and push v0.5.1**

- [ ] **Step 5: Update GitHub repo description and topics**

Set description and topics via `gh repo edit`.

- [ ] **Step 6: Verify release CI passes**
