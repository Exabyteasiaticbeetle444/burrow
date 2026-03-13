package shared

import (
	"encoding/base64"
	"testing"
)

func TestGenerateRealityKeyPair(t *testing.T) {
	kp, err := GenerateRealityKeyPair()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	privBytes, err := base64.RawURLEncoding.DecodeString(kp.PrivateKey)
	if err != nil {
		t.Fatalf("decode private key: %v", err)
	}
	if len(privBytes) != 32 {
		t.Errorf("private key length: got %d, want 32", len(privBytes))
	}

	pubBytes, err := base64.RawURLEncoding.DecodeString(kp.PublicKey)
	if err != nil {
		t.Fatalf("decode public key: %v", err)
	}
	if len(pubBytes) != 32 {
		t.Errorf("public key length: got %d, want 32", len(pubBytes))
	}

	kp2, _ := GenerateRealityKeyPair()
	if kp.PrivateKey == kp2.PrivateKey {
		t.Error("two generated key pairs should differ")
	}
}

func TestGenerateShortID(t *testing.T) {
	id, err := GenerateShortID()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(id) != 8 {
		t.Errorf("short id length: got %d, want 8 hex chars", len(id))
	}

	id2, _ := GenerateShortID()
	if id == id2 {
		t.Error("two generated short IDs should differ")
	}
}
