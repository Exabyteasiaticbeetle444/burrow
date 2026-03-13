package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/FrankFMY/burrow/internal/server/store"
)

func TestFullAPIFlow(t *testing.T) {
	api, _, _ := setupTestAPI(t)
	router := api.Router()

	// 1. Login with correct password
	loginRec := doRequest(t, router, "POST", "/api/auth/login",
		map[string]string{"password": "admin-password"}, "")
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login: got %d, want %d, body: %s", loginRec.Code, http.StatusOK, loginRec.Body.String())
	}

	var loginResp map[string]string
	decodeJSON(t, loginRec, &loginResp)
	token := loginResp["token"]
	if token == "" {
		t.Fatal("login response missing token")
	}

	// Verify cookie is also set
	cookies := loginRec.Result().Cookies()
	var foundTokenCookie bool
	for _, c := range cookies {
		if c.Name == "burrow_token" && c.Value != "" {
			foundTokenCookie = true
		}
	}
	if !foundTokenCookie {
		t.Error("login should set burrow_token cookie")
	}

	// 2. Create invite
	createRec := doRequest(t, router, "POST", "/api/invites",
		map[string]string{"name": "integration-test-client"}, token)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create invite: got %d, want %d, body: %s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}

	var createResp map[string]any
	decodeJSON(t, createRec, &createResp)
	inviteLink, ok := createResp["invite"].(string)
	if !ok || inviteLink == "" {
		t.Fatal("create invite response missing invite link")
	}
	clientData, ok := createResp["client"].(map[string]any)
	if !ok {
		t.Fatal("create invite response missing client data")
	}
	clientID, _ := clientData["id"].(string)
	clientToken, _ := clientData["token"].(string)
	if clientID == "" || clientToken == "" {
		t.Fatal("client data missing id or token")
	}

	// 3. List invites — verify the created invite appears
	listInvRec := doRequest(t, router, "GET", "/api/invites", nil, token)
	if listInvRec.Code != http.StatusOK {
		t.Fatalf("list invites: got %d, want %d", listInvRec.Code, http.StatusOK)
	}

	var clients []store.Client
	decodeJSON(t, listInvRec, &clients)
	var found bool
	for _, c := range clients {
		if c.ID == clientID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("created client %s not found in invite list", clientID)
	}

	// 4. Client connect with the token
	connectRec := doRequest(t, router, "POST", "/api/connect",
		map[string]string{"token": clientToken}, "")
	if connectRec.Code != http.StatusOK {
		t.Fatalf("connect: got %d, want %d, body: %s", connectRec.Code, http.StatusOK, connectRec.Body.String())
	}

	var connectResp map[string]any
	decodeJSON(t, connectRec, &connectResp)
	protocols, ok := connectResp["protocols"].([]any)
	if !ok || len(protocols) == 0 {
		t.Fatal("connect response missing protocols")
	}
	firstProto, ok := protocols[0].(map[string]any)
	if !ok {
		t.Fatal("protocol entry is not an object")
	}
	if firstProto["type"] != "vless" {
		t.Errorf("first protocol type: got %v, want %q", firstProto["type"], "vless")
	}

	// 5. Get client by ID
	getClientRec := doRequest(t, router, "GET", "/api/clients/"+clientID, nil, token)
	if getClientRec.Code != http.StatusOK {
		t.Fatalf("get client: got %d, want %d, body: %s", getClientRec.Code, http.StatusOK, getClientRec.Body.String())
	}

	var gotClient store.Client
	decodeJSON(t, getClientRec, &gotClient)
	if gotClient.Name != "integration-test-client" {
		t.Errorf("client name: got %q, want %q", gotClient.Name, "integration-test-client")
	}

	// 6. Get stats
	statsRec := doRequest(t, router, "GET", "/api/stats", nil, token)
	if statsRec.Code != http.StatusOK {
		t.Fatalf("get stats: got %d, want %d", statsRec.Code, http.StatusOK)
	}

	var stats store.Stats
	decodeJSON(t, statsRec, &stats)
	if stats.TotalClients < 1 {
		t.Errorf("total_clients: got %d, want >= 1", stats.TotalClients)
	}

	// 7. Get config
	configRec := doRequest(t, router, "GET", "/api/config", nil, token)
	if configRec.Code != http.StatusOK {
		t.Fatalf("get config: got %d, want %d", configRec.Code, http.StatusOK)
	}

	var configResp map[string]any
	decodeJSON(t, configRec, &configResp)
	if _, exists := configResp["server_addr"]; !exists {
		t.Error("config response missing server_addr")
	}

	// 8. Revoke client
	revokeRec := doRequest(t, router, "DELETE", "/api/clients/"+clientID, nil, token)
	if revokeRec.Code != http.StatusOK {
		t.Fatalf("revoke client: got %d, want %d, body: %s", revokeRec.Code, http.StatusOK, revokeRec.Body.String())
	}

	// 9. Connect after revoke should fail
	connectAfterRevokeRec := doRequest(t, router, "POST", "/api/connect",
		map[string]string{"token": clientToken}, "")
	if connectAfterRevokeRec.Code != http.StatusUnauthorized {
		t.Errorf("connect after revoke: got %d, want %d", connectAfterRevokeRec.Code, http.StatusUnauthorized)
	}

	// 10. Logout
	logoutRec := doRequest(t, router, "POST", "/api/auth/logout", nil, token)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout: got %d, want %d, body: %s", logoutRec.Code, http.StatusOK, logoutRec.Body.String())
	}

	// 11. Access after logout should fail
	afterLogoutRec := doRequest(t, router, "GET", "/api/clients", nil, token)
	if afterLogoutRec.Code != http.StatusUnauthorized {
		t.Errorf("access after logout: got %d, want %d", afterLogoutRec.Code, http.StatusUnauthorized)
	}
}

func TestUnauthenticatedAccess(t *testing.T) {
	api, _, db := setupTestAPI(t)
	router := api.Router()

	protectedEndpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/clients"},
		{"GET", "/api/invites"},
		{"POST", "/api/invites"},
		{"GET", "/api/stats"},
		{"GET", "/api/config"},
		{"GET", "/api/logs"},
		{"GET", "/api/health/detailed"},
		{"POST", "/api/rotate-keys"},
	}

	for _, ep := range protectedEndpoints {
		rec := doRequest(t, router, ep.method, ep.path, nil, "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without auth: got %d, want %d", ep.method, ep.path, rec.Code, http.StatusUnauthorized)
		}
	}

	// Public: GET /health
	healthRec := doRequest(t, router, "GET", "/health", nil, "")
	if healthRec.Code != http.StatusOK {
		t.Errorf("GET /health: got %d, want %d", healthRec.Code, http.StatusOK)
	}

	// Public: POST /api/connect with a valid client token
	client := &store.Client{
		ID:    "unauth-test-client",
		Name:  "Unauth Test",
		Token: "unauth-connect-token",
	}
	client.CreatedAt = client.CreatedAt.UTC()
	if err := db.CreateClient(t.Context(), client); err != nil {
		t.Fatalf("create client: %v", err)
	}

	connectRec := doRequest(t, router, "POST", "/api/connect",
		map[string]string{"token": "unauth-connect-token"}, "")
	if connectRec.Code != http.StatusOK {
		t.Errorf("POST /api/connect with valid token: got %d, want %d, body: %s",
			connectRec.Code, http.StatusOK, connectRec.Body.String())
	}

	var connectResp map[string]any
	if err := json.NewDecoder(connectRec.Body).Decode(&connectResp); err != nil {
		t.Fatalf("decode connect response: %v", err)
	}
	if connectResp["client_id"] != "unauth-test-client" {
		t.Errorf("connect client_id: got %v, want %q", connectResp["client_id"], "unauth-test-client")
	}
}
