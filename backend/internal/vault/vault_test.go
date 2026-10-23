package vault

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	v, err := New("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("sqlserver://sa:S3cr3t!@db01:1433?database=ERP")
	enc, err := v.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(enc, []byte("S3cr3t")) {
		t.Fatal("ciphertext leaks plaintext")
	}
	dec, err := v.Decrypt(enc)
	if err != nil || !bytes.Equal(dec, plain) {
		t.Fatalf("decrypt mismatch: %s %v", dec, err)
	}
}

func TestNonceIsRandom(t *testing.T) {
	v, _ := New("k")
	a, _ := v.Encrypt([]byte("same"))
	b, _ := v.Encrypt([]byte("same"))
	if bytes.Equal(a, b) {
		t.Fatal("two encryptions of the same plaintext must differ")
	}
}

func TestTamperDetected(t *testing.T) {
	v, _ := New("k")
	enc, _ := v.Encrypt([]byte("secret"))
	enc[len(enc)-1] ^= 0x01
	if _, err := v.Decrypt(enc); err == nil {
		t.Fatal("tampered ciphertext must fail authentication")
	}
}

func TestWrongKeyFails(t *testing.T) {
	a, _ := New("key-a")
	b, _ := New("key-b")
	enc, _ := a.Encrypt([]byte("secret"))
	if _, err := b.Decrypt(enc); err == nil {
		t.Fatal("decrypt with wrong key must fail")
	}
}

func TestBase64Key(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	v, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	enc, _ := v.Encrypt([]byte("x"))
	if out, err := v.Decrypt(enc); err != nil || string(out) != "x" {
		t.Fatal("base64 key round trip failed")
	}
}

func TestEmptyKeyRejected(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("empty key must be rejected")
	}
}

func TestShortCiphertext(t *testing.T) {
	v, _ := New("k")
	if _, err := v.Decrypt([]byte{1, 2}); err == nil {
		t.Fatal("short ciphertext must fail")
	}
}
