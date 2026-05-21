package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
)

type AESGCM struct {
	aead cipher.AEAD
}

func NewAESGCM(hexKey string) (*AESGCM, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("AES-256-GCM key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AESGCM{aead: aead}, nil
}

func (c *AESGCM) EncryptString(plaintext string) ([]byte, []byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return c.aead.Seal(nil, nonce, []byte(plaintext), nil), nonce, nil
}

func (c *AESGCM) DecryptString(ciphertext, nonce []byte) (string, error) {
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
