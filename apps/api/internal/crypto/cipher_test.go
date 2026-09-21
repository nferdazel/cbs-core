package crypto

import (
	"encoding/base64"
	"slices"
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

// Nilai indeks harus TIDAK BERUBAH saat kunci indeks terpisah belum dikonfigurasi.
// Nilai emas di bawah dihitung dari formula lama (HMAC-SHA256 master key +
// "blind-index:"/"name-token:") dengan testKey deterministik; bila implementasi
// mengubah formula, test ini gagal dan pencarian NIK/email/nama yang sudah berjalan
// akan terputus.
func TestDefaultIndexValueUnchanged(t *testing.T) {
	c, err := NewCipher("k1", testKey(t), nil)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	if got, want := c.BlindIndex("3201234567890001"), "ipAeq4Frtv+IPub2ywE+Z3wgQeRh8axMLbMzbl3ykF0="; got != want {
		t.Fatalf("blind index berubah: %q, ingin %q", got, want)
	}
	if got, want := c.NameTokenIndex("siti"), "KnNPo9UIYOxszl3bj4F2POZmxpzsMrWRmklU+WGhM6E="; got != want {
		t.Fatalf("token nama berubah: %q, ingin %q", got, want)
	}
	if got := c.IndexKeyVersion(); got != "k1" {
		t.Fatalf("versi kunci indeks bawaan = %q, ingin k1", got)
	}
}

func byteKey(b byte) string {
	key := make([]byte, 32)
	for i := range key {
		key[i] = b
	}
	return base64.StdEncoding.EncodeToString(key)
}

// Kunci indeks terpisah harus mengubah nilai indeks (memisahkan domain kunci), tetapi
// kandidat pencarian tetap memuat nilai versi lama agar data lama tidak hilang.
func TestSeparateIndexKeyProducesDifferentValueWithLegacyCandidate(t *testing.T) {
	const value = "3201234567890001"
	legacy, err := NewCipher("k1", testKey(t), nil)
	if err != nil {
		t.Fatalf("cipher lama: %v", err)
	}
	split, err := NewCipherWithIndexKey("k1", testKey(t), nil, IndexKeyConfig{
		ActiveKeyID: "ik1",
		ActiveKey:   byteKey(7),
	})
	if err != nil {
		t.Fatalf("cipher kunci terpisah: %v", err)
	}

	oldIndex, newIndex := legacy.BlindIndex(value), split.BlindIndex(value)
	if oldIndex == newIndex {
		t.Fatal("kunci indeks terpisah menghasilkan nilai yang sama; pemisahan kunci tidak berlaku")
	}
	candidates := split.BlindIndexCandidates(value)
	if !slices.Contains(candidates, oldIndex) {
		t.Fatalf("kandidat tidak memuat indeks versi lama %q: %v", oldIndex, candidates)
	}
	if !slices.Contains(candidates, newIndex) {
		t.Fatalf("kandidat tidak memuat indeks versi baru %q: %v", newIndex, candidates)
	}
	if got := split.IndexKeyVersion(); got != "ik1" {
		t.Fatalf("versi kunci indeks aktif = %q, ingin ik1", got)
	}
}

// Pesan galat konfigurasi kunci tidak boleh membocorkan nilai kunci. Nilai yang
// diuji sengaja disisipkan agar mudah dideteksi bila muncul di pesan.
func TestCipherConfigErrorsDoNotLeakKeyMaterial(t *testing.T) {
	const secret = "nilai-kunci-yang-tidak-boleh-muncul"

	_, err := NewCipher("k1", "!!!"+secret, nil)
	if err == nil {
		t.Fatal("master key bukan base64 seharusnya ditolak")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("pesan galat membocorkan nilai kunci: %v", err)
	}

	// Base64 valid tetapi ukuran salah; pesan hanya boleh menyebut kelas kesalahan.
	_, err = NewCipherWithIndexKey("k1", testKey(t), nil, IndexKeyConfig{ActiveKey: base64.StdEncoding.EncodeToString([]byte(secret))})
	if err == nil {
		t.Fatal("kunci indeks dengan ukuran salah seharusnya ditolak")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("pesan galat membocorkan nilai kunci: %v", err)
	}

	_, err = NewCipherWithIndexKey("k1", testKey(t), nil, IndexKeyConfig{ActiveKey: "!!!" + secret})
	if err == nil {
		t.Fatal("kunci indeks bukan base64 seharusnya ditolak")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("pesan galat membocorkan nilai kunci: %v", err)
	}
}

// Id kunci indeks yang sama dengan id kunci enkripsi tetapi material berbeda adalah
// salah konfigurasi dan harus ditolak, bukan menimpa kunci diam-diam.
func TestSeparateIndexKeyRejectsIDCollision(t *testing.T) {
	_, err := NewCipherWithIndexKey("k1", testKey(t), nil, IndexKeyConfig{ActiveKeyID: "k1", ActiveKey: byteKey(9)})
	if err == nil {
		t.Fatal("id kunci indeks yang menabrak kunci enkripsi harus ditolak")
	}
}
