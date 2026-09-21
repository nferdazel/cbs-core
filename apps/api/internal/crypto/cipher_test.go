package crypto

import (
	"encoding/base64"
	"strings"
	"testing"
)

func testKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	c, err := NewCipher("k1", testKey(t), nil)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}

	plaintext := "3201234567890001"
	blob, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if strings.Contains(blob, plaintext) {
		t.Fatal("blob memuat plaintext")
	}

	got, err := c.Decrypt(blob)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got != plaintext {
		t.Fatalf("hasil dekripsi %q, ingin %q", got, plaintext)
	}
}

func TestEncryptUsesFreshDataKey(t *testing.T) {
	c, _ := NewCipher("k1", testKey(t), nil)

	a, _ := c.Encrypt("sama")
	b, _ := c.Encrypt("sama")
	if a == b {
		t.Fatal("dua enkripsi nilai sama menghasilkan blob identik; data key tidak acak")
	}
}

func TestDecryptRejectsTamperedBlob(t *testing.T) {
	c, _ := NewCipher("k1", testKey(t), nil)
	blob, _ := c.Encrypt("3201234567890001")

	parts := strings.Split(blob, ":")
	raw, _ := base64.StdEncoding.DecodeString(parts[4])
	raw[0] ^= 0xFF
	parts[4] = base64.StdEncoding.EncodeToString(raw)

	if _, err := c.Decrypt(strings.Join(parts, ":")); err == nil {
		t.Fatal("diharapkan gagal pada ciphertext yang dimodifikasi")
	}
}

func TestDecryptRejectsWrongKey(t *testing.T) {
	c1, _ := NewCipher("k1", testKey(t), nil)
	blob, _ := c1.Encrypt("rahasia")

	otherKey := make([]byte, 32)
	for i := range otherKey {
		otherKey[i] = byte(200 - i)
	}
	c2, _ := NewCipher("k1", base64.StdEncoding.EncodeToString(otherKey), nil)

	if _, err := c2.Decrypt(blob); err == nil {
		t.Fatal("diharapkan gagal saat didekripsi dengan master key berbeda")
	}
}

func TestRotationDecryptsOldData(t *testing.T) {
	oldKey := testKey(t)
	cOld, _ := NewCipher("k1", oldKey, nil)
	blob, _ := cOld.Encrypt("data lama")

	// Master key baru, kunci lama tetap tersedia untuk dekripsi.
	newKey := make([]byte, 32)
	for i := range newKey {
		newKey[i] = byte(99 - i)
	}
	cNew, err := NewCipher("k2", base64.StdEncoding.EncodeToString(newKey), map[string]string{"k1": oldKey})
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}

	got, err := cNew.Decrypt(blob)
	if err != nil {
		t.Fatalf("dekripsi data lama setelah rotasi: %v", err)
	}
	if got != "data lama" {
		t.Fatalf("hasil %q", got)
	}
}

func TestBlindIndexDeterministic(t *testing.T) {
	c, _ := NewCipher("k1", testKey(t), nil)
	if c.BlindIndex("3201234567890001") != c.BlindIndex("3201234567890001") {
		t.Fatal("blind index tidak deterministik")
	}
	if c.BlindIndex("3201234567890001") == c.BlindIndex("3201234567890002") {
		t.Fatal("nilai berbeda menghasilkan blind index sama")
	}
}

// Indeks token nama wajib deterministik supaya pencarian stabil, tetapi harus
// terpisah domain dari blind index nilai utuh: token "siti" tidak boleh pernah
// sama dengan BlindIndex("SITI"), agar satu indeks tidak bocor ke konteks lain.
func TestNameTokenIndexSeparateDomain(t *testing.T) {
	c, _ := NewCipher("k1", testKey(t), nil)
	if c.NameTokenIndex("siti") != c.NameTokenIndex("siti") {
		t.Fatal("indeks token nama tidak deterministik")
	}
	if c.NameTokenIndex("siti") == c.NameTokenIndex("rahayu") {
		t.Fatal("token berbeda menghasilkan indeks sama")
	}
	if c.NameTokenIndex("siti") == c.BlindIndex("siti") {
		t.Fatal("token nama tidak boleh berbagi indeks dengan blind index nilai utuh")
	}
}

func TestNewCipherRejectsBadKey(t *testing.T) {
	if _, err := NewCipher("k1", "", nil); err == nil {
		t.Fatal("master key kosong seharusnya ditolak")
	}
	short := base64.StdEncoding.EncodeToString([]byte("terlalu pendek"))
	if _, err := NewCipher("k1", short, nil); err == nil {
		t.Fatal("master key dengan ukuran salah seharusnya ditolak")
	}
}
