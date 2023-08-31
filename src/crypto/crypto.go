package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"io"
)

type crypto struct {
	key []byte
}

type Crypto interface {
	Encrypt(value string) (string, error)
	Decrypt(encValue string) ([]byte, error)
}

func NewCrypto(key []byte) Crypto {
	return crypto{key}
}

func (c crypto) Encrypt(value string) (string, error) {
	cip, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(cip)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	return hex.EncodeToString(gcm.Seal(nonce, nonce, []byte(value), nil)), nil
}

func (c crypto) Decrypt(encValue string) ([]byte, error) {
	encByte, err := hex.DecodeString(encValue)
	if err != nil {
		return []byte{}, err
	}

	cip, err := aes.NewCipher(c.key)
	if err != nil {
		return []byte{}, err
	}

	gcm, err := cipher.NewGCM(cip)
	if err != nil {
		return []byte{}, err
	}

	nonceSize := gcm.NonceSize()
	if len(encValue) < nonceSize {
		return []byte{}, err
	}

	nonce, encByte := encByte[:nonceSize], encByte[nonceSize:]
	decValue, err := gcm.Open(nil, nonce, encByte, nil)
	if err != nil {
		return []byte{}, err
	}
	return decValue, nil
}
