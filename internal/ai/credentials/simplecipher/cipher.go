package simplecipher

import "context"

// Cipher is a starter implementation that performs no real encryption.
// Replace with a proper cipher (KMS, envelope encryption, etc.) before production use.
type Cipher struct{}

func New() *Cipher { return &Cipher{} }

func (Cipher) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
    return append([]byte(nil), plaintext...), nil
}

func (Cipher) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
    return append([]byte(nil), ciphertext...), nil
}
