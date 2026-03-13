package client

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/FrankFMY/burrow/internal/shared"
)

func generateTestTLSConfig(t *testing.T) *tls.Config {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("x509 key pair: %v", err)
	}

	return &tls.Config{Certificates: []tls.Certificate{cert}}
}

func listenTCP(t *testing.T) (net.Listener, uint16) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	var port uint16
	fmt.Sscanf(portStr, "%d", &port)
	return ln, port
}

func TestDiagDNS(t *testing.T) {
	step := diagDNS()
	if step.Name != "DNS Resolution" {
		t.Errorf("unexpected step name: %s", step.Name)
	}
	if !step.Passed {
		t.Logf("DNS step failed (may be expected in sandboxed env): %s", step.Detail)
	}
}

func TestDiagTCP_Reachable(t *testing.T) {
	ln, port := listenTCP(t)
	defer ln.Close()

	step := diagTCP("127.0.0.1", port)
	if !step.Passed {
		t.Errorf("expected TCP to pass for local listener, got: %s", step.Detail)
	}
	if step.Latency == 0 {
		t.Error("expected non-zero latency")
	}
}

func TestDiagTCP_Unreachable(t *testing.T) {
	step := diagTCP("127.0.0.1", 1)
	if step.Passed {
		t.Error("expected TCP to fail for unreachable port")
	}
	if step.Detail == "" {
		t.Error("expected non-empty detail on failure")
	}
}

func TestDiagTLS_WithServer(t *testing.T) {
	tlsCfg := generateTestTLSConfig(t)
	ln, err := tls.Listen("tcp", "127.0.0.1:0", tlsCfg)
	if err != nil {
		t.Fatalf("tls listen: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// Perform the TLS handshake before closing
			if tlsConn, ok := conn.(*tls.Conn); ok {
				tlsConn.Handshake()
			}
			conn.Close()
		}
	}()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	var port uint16
	fmt.Sscanf(portStr, "%d", &port)

	step := diagTLS("127.0.0.1", port, "localhost")
	if !step.Passed {
		t.Errorf("expected TLS handshake to pass: %s", step.Detail)
	}
	if !strings.Contains(step.Detail, "TLS") {
		t.Errorf("expected detail to mention TLS version: %s", step.Detail)
	}
}

func TestDiagTLS_Unreachable(t *testing.T) {
	step := diagTLS("127.0.0.1", 1, "localhost")
	if step.Passed {
		t.Error("expected TLS to fail for unreachable port")
	}
}

func TestDiagCDN_Reachable(t *testing.T) {
	ln, port := listenTCP(t)
	defer ln.Close()

	step := diagCDN("127.0.0.1", port)
	if !step.Passed {
		t.Errorf("expected CDN reachability to pass: %s", step.Detail)
	}
}

func TestDiagCDN_DefaultPort(t *testing.T) {
	step := diagCDN("127.0.0.1", 0)
	if step.Name != "CDN Reachability" {
		t.Errorf("unexpected step name: %s", step.Name)
	}
}

func TestDiagLatency_Reachable(t *testing.T) {
	ln, port := listenTCP(t)
	defer ln.Close()

	step := diagLatency("127.0.0.1", port)
	if !step.Passed {
		t.Errorf("expected latency to pass for local listener: %s", step.Detail)
	}
	if !strings.Contains(step.Detail, "3/3 probes") {
		t.Errorf("expected 3/3 probes: %s", step.Detail)
	}
}

func TestDiagLatency_Unreachable(t *testing.T) {
	step := diagLatency("127.0.0.1", 1)
	if step.Passed {
		t.Error("expected latency to fail for unreachable port")
	}
}

func TestDiagnose_Full(t *testing.T) {
	ln, port := listenTCP(t)
	defer ln.Close()

	invite := shared.InviteData{
		Server: "127.0.0.1",
		Port:   port,
		SNI:    "localhost",
	}

	result, err := Diagnose(invite)
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}

	if len(result.Steps) != 4 {
		t.Errorf("expected 4 steps without CDN, got %d", len(result.Steps))
	}

	expected := []string{"DNS Resolution", "TCP Connectivity", "TLS Handshake", "Latency (RTT)"}
	for i, e := range expected {
		if i >= len(result.Steps) {
			break
		}
		if result.Steps[i].Name != e {
			t.Errorf("step %d: expected %q, got %q", i, e, result.Steps[i].Name)
		}
	}
}

func TestDiagnose_WithCDN(t *testing.T) {
	ln, port := listenTCP(t)
	defer ln.Close()

	invite := shared.InviteData{
		Server:  "127.0.0.1",
		Port:    port,
		SNI:     "localhost",
		CDNHost: "127.0.0.1",
		CDNPort: port,
	}

	result, err := Diagnose(invite)
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}

	if len(result.Steps) != 5 {
		t.Errorf("expected 5 steps with CDN, got %d", len(result.Steps))
	}

	if result.Steps[3].Name != "CDN Reachability" {
		t.Errorf("expected step 4 to be CDN, got %q", result.Steps[3].Name)
	}
}

func TestDiagResult_AllPassed(t *testing.T) {
	r := &DiagResult{
		Steps: []StepResult{
			{Name: "A", Passed: true},
			{Name: "B", Passed: true},
		},
	}
	if !r.AllPassed() {
		t.Error("expected AllPassed to return true")
	}

	r.Steps = append(r.Steps, StepResult{Name: "C", Passed: false})
	if r.AllPassed() {
		t.Error("expected AllPassed to return false with a failed step")
	}
}

func TestDiagResult_AllPassed_Empty(t *testing.T) {
	r := &DiagResult{}
	if !r.AllPassed() {
		t.Error("expected AllPassed to return true for empty steps")
	}
}

func TestFormatDiagResult(t *testing.T) {
	r := &DiagResult{
		Steps: []StepResult{
			{Name: "DNS Resolution", Passed: true, Detail: "ok"},
			{Name: "TCP Connectivity", Passed: false, Detail: "refused"},
		},
	}

	output := FormatDiagResult(r)

	if !strings.Contains(output, "[PASS] DNS Resolution") {
		t.Error("expected PASS marker for DNS")
	}
	if !strings.Contains(output, "[FAIL] TCP Connectivity") {
		t.Error("expected FAIL marker for TCP")
	}
	if !strings.Contains(output, "1/2 checks passed") {
		t.Errorf("expected summary, got:\n%s", output)
	}
	if !strings.Contains(output, "Connection Diagnostics") {
		t.Error("expected header in output")
	}
}

func TestTLSVersionString(t *testing.T) {
	tests := []struct {
		version uint16
		want    string
	}{
		{tls.VersionTLS10, "1.0"},
		{tls.VersionTLS11, "1.1"},
		{tls.VersionTLS12, "1.2"},
		{tls.VersionTLS13, "1.3"},
		{0x0300, "0x0300"},
	}
	for _, tt := range tests {
		got := tlsVersionString(tt.version)
		if got != tt.want {
			t.Errorf("tlsVersionString(%#x) = %q, want %q", tt.version, got, tt.want)
		}
	}
}
