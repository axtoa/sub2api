package cdkshop

import (
	"testing"
	"time"
)

func TestCDKShopTokenIsShortLivedAndAudienceBound(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	now := time.Unix(1_800_000_000, 0)
	token, err := SignToken(secret, 17, "admin", now)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := VerifyToken(secret, token, now.Add(4*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != 17 || claims.Role != "admin" || claims.Audience != "cdkshop" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if _, err := VerifyToken([]byte("another independent secret with enough bytes"), token, now); err == nil {
		t.Fatal("token verified with another service secret")
	}
	if _, err := VerifyToken(secret, token, now.Add(6*time.Minute)); err == nil {
		t.Fatal("expired token verified")
	}
}

func TestCryptoEncryptsAndFingerprintsCDK(t *testing.T) {
	c, err := NewCrypto("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	if err != nil {
		t.Fatal(err)
	}
	a, err := c.Encrypt("SECRET-CODE")
	if err != nil {
		t.Fatal(err)
	}
	b, err := c.Encrypt("SECRET-CODE")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("ciphertext must use a fresh nonce")
	}
	plain, err := c.Decrypt(a)
	if err != nil || plain != "SECRET-CODE" {
		t.Fatalf("decrypt = %q, %v", plain, err)
	}
	fingerprint := c.Fingerprint(plain)
	if fingerprint != c.Fingerprint("SECRET-CODE") {
		t.Fatal("fingerprint is not stable")
	}
}
