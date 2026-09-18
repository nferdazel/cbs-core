// Package crypto menyediakan enkripsi field-level untuk data pribadi nasabah.
//
// Model: envelope encryption. Setiap nilai dienkripsi dengan data key acak 32 byte,
// dan data key itu sendiri dienkripsi dengan master key. Hasilnya satu blob yang
// membawa key id, sehingga rotasi master key tidak memerlukan dekripsi ulang seluruh
// tabel sekaligus.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const (
	versionPrefix = "v1"
	masterKeySize = 32
	dataKeySize   = 32
)

var (
	ErrMissingMasterKey = errors.New("master key enkripsi tidak diset")
	ErrInvalidKeySize   = errors.New("ukuran master key enkripsi harus 32 byte")
	ErrMalformedBlob    = errors.New("format data terenkripsi tidak valid")
	ErrUnknownKeyID     = errors.New("key id tidak dikenal")
)

// Cipher mengenkripsi dan mendekripsi nilai serta membangkitkan blind index.
type Cipher struct {
	keyID     string
	masterKey []byte
	// keys memungkinkan dekripsi data lama saat master key dirotasi.
	keys map[string][]byte
}

// NewCipher membangun Cipher dari master key base64 dan id kunci aktifnya.
// keys tambahan untuk dekripsi versi lama bersifat opsional.
func NewCipher(activeKeyID, masterKeyB64 string, previousKeys map[string]string) (*Cipher, error) {
	if masterKeyB64 == "" {
		return nil, ErrMissingMasterKey
	}
	key, err := base64.StdEncoding.DecodeString(masterKeyB64)
	if err != nil {
		return nil, fmt.Errorf("master key bukan base64 valid: %w", err)
	}
	if len(key) != masterKeySize {
		return nil, ErrInvalidKeySize
	}
	if activeKeyID == "" {
		activeKeyID = "k1"
	}

	c := &Cipher{
		keyID:     activeKeyID,
		masterKey: key,
		keys:      map[string][]byte{activeKeyID: key},
	}
	for id, b64 := range previousKeys {
		prev, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("previous key %s bukan base64 valid: %w", id, err)
		}
		if len(prev) != masterKeySize {
			return nil, fmt.Errorf("previous key %s: %w", id, ErrInvalidKeySize)
		}
		c.keys[id] = prev
	}
	return c, nil
}

// Encrypt mengenkripsi plaintext dan mengembalikan blob siap simpan.
// Format: v1:keyID:base64(dataKeyTerbungkus):base64(nonce):base64(ciphertext)
func (c *Cipher) Encrypt(plaintext string) (string, error) {
	dataKey := make([]byte, dataKeySize)
	if _, err := rand.Read(dataKey); err != nil {
		return "", err
	}

	wrappedKey, err := seal(c.masterKey, dataKey, []byte(c.keyID))
	if err != nil {
		return "", err
	}

	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	block, err := aes.NewCipher(dataKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	return strings.Join([]string{
		versionPrefix,
		c.keyID,
		base64.StdEncoding.EncodeToString(wrappedKey),
		base64.StdEncoding.EncodeToString(nonce),
		base64.StdEncoding.EncodeToString(ciphertext),
	}, ":"), nil
}

// Decrypt membuka blob yang dihasilkan Encrypt.
func (c *Cipher) Decrypt(blob string) (string, error) {
	parts := strings.Split(blob, ":")
	if len(parts) != 5 || parts[0] != versionPrefix {
		return "", ErrMalformedBlob
	}
	keyID := parts[1]
	master, ok := c.keys[keyID]
	if !ok {
		return "", ErrUnknownKeyID
	}

	wrappedKey, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", ErrMalformedBlob
	}
	nonce, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil {
		return "", ErrMalformedBlob
	}
	ciphertext, err := base64.StdEncoding.DecodeString(parts[4])
	if err != nil {
		return "", ErrMalformedBlob
	}

	dataKey, err := open(master, wrappedKey, []byte(keyID))
	if err != nil {
		return "", ErrMalformedBlob
	}

	block, err := aes.NewCipher(dataKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrMalformedBlob
	}
	return string(plaintext), nil
}

// BlindIndex menghasilkan indeks pencarian deterministik untuk nilai yang terenkripsi.
// Dipakai agar kolom seperti NIK tetap bisa dicari tanpa membuka enkripsi. Nilai index
// tidak dapat dibalik menjadi plaintext.
func (c *Cipher) BlindIndex(value string) string {
	mac := hmac.New(sha256.New, c.masterKey)
	mac.Write([]byte("blind-index:"))
	mac.Write([]byte(strings.TrimSpace(strings.ToUpper(value))))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func seal(key, plaintext, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, aad), nil
}

func open(key, sealed, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, ErrMalformedBlob
	}
	nonce, ciphertext := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, aad)
}
