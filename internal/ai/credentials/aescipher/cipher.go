package aescipher

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

var (
	ErrInvalidKeyLength = errors.New("aescipher: key must be 16, 24, or 32 bytes")
	nonceSize           = 12
)

// Cipher provides AES-GCM based encryption for provider credentials.
type Cipher struct {
	key []byte
}

// New constructs a Cipher using the supplied AES key.
func New(key []byte) (*Cipher, error) {
	if !validKeyLength(len(key)) {
		return nil, ErrInvalidKeyLength
	}
	buf := make([]byte, len(key))
	copy(buf, key)
	return &Cipher{key: buf}, nil
}

// NewFromBase64 creates a cipher from a base64 encoded key.
func NewFromBase64(encoded string) (*Cipher, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	return New(raw)
}

func (c *Cipher) Encrypt(_ context.Context, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, len(nonce)+len(ciphertext))
	copy(out, nonce)
	copy(out[len(nonce):], ciphertext)
	return out, nil
}

func (c *Cipher) Decrypt(_ context.Context, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < nonceSize {
		return nil, errors.New("aescipher: ciphertext too short")
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := ciphertext[:nonceSize]
	data := ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, data, nil)
}

func validKeyLength(length int) bool {
	switch length {
	case 16, 24, 32:
		return true
	default:
		return false
	}
}
