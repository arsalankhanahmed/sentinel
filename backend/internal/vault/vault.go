// Package vault provides envelope-free AES-256-GCM encryption for secrets at rest
// (connection strings, webhook URLs, API tokens). Swap Encrypt/Decrypt for a
// KMS/HashiCorp Vault transit call without touching callers.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

type Vault struct{ aead cipher.AEAD }

func New(masterKey string) (*Vault, error) {
	if masterKey == "" {
		return nil, errors.New("MASTER_KEY is required")
	}
	key, err := base64.StdEncoding.DecodeString(masterKey)
	if err != nil || len(key) != 32 {
		sum := sha256.Sum256([]byte(masterKey)) // passphrase fallback
		key = sum[:]
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Vault{aead: aead}, nil
}

// Encrypt returns nonce||ciphertext.
func (v *Vault) Encrypt(plain []byte) ([]byte, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return v.aead.Seal(nonce, nonce, plain, nil), nil
}

func (v *Vault) Decrypt(blob []byte) ([]byte, error) {
	ns := v.aead.NonceSize()
	if len(blob) < ns {
		return nil, errors.New("vault: ciphertext too short")
	}
	return v.aead.Open(nil, blob[:ns], blob[ns:], nil)
}
