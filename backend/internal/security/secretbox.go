package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

type SecretBox struct {
	aead cipher.AEAD
}

func NewSecretBox(master string) (*SecretBox, error) {
	sum := sha256.Sum256([]byte(master))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &SecretBox{aead: aead}, nil
}

func (s *SecretBox) Encrypt(plain string) (string, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	cipherText := s.aead.Seal(nil, nonce, []byte(plain), nil)
	payload := append(nonce, cipherText...)
	return base64.RawStdEncoding.EncodeToString(payload), nil
}

func (s *SecretBox) Decrypt(payload string) (string, error) {
	raw, err := base64.RawStdEncoding.DecodeString(payload)
	if err != nil {
		return "", err
	}
	if len(raw) < s.aead.NonceSize() {
		return "", errors.New("cipher payload too short")
	}
	nonce, cipherText := raw[:s.aead.NonceSize()], raw[s.aead.NonceSize():]
	plain, err := s.aead.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
