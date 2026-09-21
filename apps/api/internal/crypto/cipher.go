// Package crypto menyediakan enkripsi field-level untuk data pribadi nasabah.
//
// Model: envelope encryption. Setiap nilai dienkripsi dengan data key acak 32 byte,
// dan data key itu sendiri dienkripsi dengan master key. Hasilnya satu blob yang
// membawa key id, sehingga rotasi master key tidak memerlukan dekripsi ulang seluruh
// tabel sekaligus.
//
// Pemisahan kunci indeks (migrasi 000048)
//
// Enkripsi dan pembentukan indeks pencarian semula memakai master key yang sama.
// Sejak dukungan kunci indeks terpisah, blind index dan token nama dapat memakai
// kunci sendiri (ENCRYPTION_INDEX_KEY) tanpa mengubah kunci enkripsi. Alasannya:
// kunci indeks deterministik — kalau bocor, kamus nilai bisa dicocokkan untuk
// menebak NIK/email; dan rotasi kunci enkripsi tidak boleh mematikan indeks.
//
// Kompatibilitas: bila kunci indeks terpisah TIDAK dikonfigurasi, kunci indeks
// adalah master key enkripsi dengan id yang sama, sehingga nilai indeks untuk
// masukan yang sama persis seperti sebelumnya. Baris lama karena itu tetap
// ditemukan. Untuk pencarian lintas versi, BlindIndexCandidates/
// NameTokenIndexCandidates menghitung indeks memakai SEMUA versi kunci indeks yang
// dikenal (kunci enkripsi aktif/lama + kunci indeks terpisah), lalu repository
// mencocokkan dengan `= ANY(...)`; baris versi lama dan baru menyatu dalam satu
// hasil.
//
// Rotasi aktual TIDAK dilakukan otomatis oleh kode ini. Prosedurnya tertulis di
// paket ini dan di migrasi 000048; baris baru selalu mencatat versi kuncinya di
// kolom index_key_version, sehingga rotasi bisa dikerjakan bertahap dan idempoten
// kapan pun tanpa menulis ulang data secara diam-diam.
package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	versionPrefix = "v1"
	masterKeySize = 32
	dataKeySize   = 32
	// defaultKeyID dipakai bila id kunci enkripsi tidak diisi, agar cocok dengan
	// data lama yang selalu ditulis dengan id "k1".
	defaultKeyID = "k1"
	// defaultIndexKeyID dipakai bila kunci indeks terpisah diisi tanpa id.
	defaultIndexKeyID = "ik1"
)

var (
	ErrMissingMasterKey = errors.New("master key enkripsi tidak diset")
	ErrInvalidKeySize   = errors.New("ukuran master key enkripsi harus 32 byte")
	// ErrInvalidKeyEncoding sengaja tidak memuat nilai kunci. Pesan galat konfigurasi
	// kunci tidak boleh membocorkan kunci, dan nilai kunci tidak pernah ditulis ke log.
	ErrInvalidKeyEncoding = errors.New("kunci harus base64 valid")
	ErrMalformedBlob      = errors.New("format data terenkripsi tidak valid")
	ErrUnknownKeyID       = errors.New("key id tidak dikenal")
)

// IndexKeyConfig memuat kunci terpisah untuk pembentukan indeks pencarian
// (blind index dan token nama). Bila ActiveKey kosong, Cipher memakai master key
// enkripsi sebagai kunci indeks — persis perilaku sebelum pemisahan kunci,
// sehingga nilai indeks lama tidak berubah dan pencarian yang sudah berjalan tetap
// cocok.
type IndexKeyConfig struct {
	// ActiveKeyID adalah versi kunci indeks yang ditulis ke baris baru.
	// Bila kosong dan ActiveKey diisi, dipakai "ik1".
	ActiveKeyID string
	// ActiveKey adalah kunci indeks aktif, base64 dari 32 byte. Kosong berarti
	// memakai master key enkripsi (mode kompatibel).
	ActiveKey string
	// PreviousKeys memetakan versi kunci indeks lama ke kunci base64-nya agar
	// pencarian lintas versi tetap menemukan baris yang diindeks dengan kunci lama.
	// Kunci enkripsi (aktif dan lama) selalu ikut sebagai kandidat.
	PreviousKeys map[string]string
}

// Cipher mengenkripsi dan mendekripsi nilai serta membangkitkan blind index.
type Cipher struct {
	keyID     string
	masterKey []byte
	// keys memungkinkan dekripsi data lama saat master key dirotasi.
	keys map[string][]byte

	// indexKeyID adalah versi kunci indeks aktif yang dicatat pada baris baru.
	indexKeyID string
	// indexKeys memetakan versi kunci ke material HMAC-nya. Isinya: kunci enkripsi
	// (aktif + lama) sebagai kandidat warisan, ditambah kunci indeks terpisah bila
	// dikonfigurasi.
	indexKeys map[string][]byte
	// indexKeyOrder menjaga urutan kandidat deterministik tanpa memengaruhi hasil.
	indexKeyOrder []string
}

// NewCipher membangun Cipher dari master key base64 dan id kunci aktifnya, dengan
// kunci indeks mengikuti master key (perilaku lama). keys tambahan untuk dekripsi
// versi lama bersifat opsional.
func NewCipher(activeKeyID, masterKeyB64 string, previousKeys map[string]string) (*Cipher, error) {
	return NewCipherWithIndexKey(activeKeyID, masterKeyB64, previousKeys, IndexKeyConfig{})
}

// NewCipherWithIndexKey membangun Cipher dengan dukungan kunci indeks terpisah.
// Menambah kunci tidak mengubah nilai indeks: kandidat hanya memperluas pencarian.
//
// Idempotensi/keamanan: nilai kunci tidak pernah ditulis ke log atau pesan galat;
// fungsi ini hanya mengembalikan galat bila kunci bukan base64/berukuran salah.
func NewCipherWithIndexKey(activeKeyID, masterKeyB64 string, previousKeys map[string]string, idx IndexKeyConfig) (*Cipher, error) {
	if masterKeyB64 == "" {
		return nil, ErrMissingMasterKey
	}
	key, err := decodeKey(masterKeyB64)
	if err != nil {
		return nil, fmt.Errorf("master key enkripsi: %w", err)
	}
	if activeKeyID == "" {
		activeKeyID = defaultKeyID
	}

	c := &Cipher{
		keyID:     activeKeyID,
		masterKey: key,
		keys:      map[string][]byte{activeKeyID: key},
		indexKeys: map[string][]byte{},
	}
	// Kunci enkripsi selalu menjadi kandidat kunci indeks: baris lama diindeks
	// dengan master key sebelum pemisahan kunci ada, dan dulu versi indeksnya tidak
	// disimpan. Tanpa kandidat ini, baris lama tidak akan pernah ditemukan lagi saat
	// kunci indeks terpisah diaktifkan.
	if err := c.addIndexKey(activeKeyID, key); err != nil {
		return nil, err
	}
	for _, id := range sortedKeyIDs(previousKeys) {
		prev, err := decodeKey(previousKeys[id])
		if err != nil {
			return nil, fmt.Errorf("kunci lama %s: %w", id, err)
		}
		c.keys[id] = prev
		if err := c.addIndexKey(id, prev); err != nil {
			return nil, err
		}
	}

	if idx.ActiveKey != "" {
		if idx.ActiveKeyID == "" {
			idx.ActiveKeyID = defaultIndexKeyID
		}
		decoded, err := decodeKey(idx.ActiveKey)
		if err != nil {
			return nil, fmt.Errorf("kunci indeks: %w", err)
		}
		if err := c.addIndexKey(idx.ActiveKeyID, decoded); err != nil {
			return nil, err
		}
		c.indexKeyID = idx.ActiveKeyID
	} else {
		// Mode kompatibel: kunci indeks = kunci enkripsi aktif.
		c.indexKeyID = activeKeyID
	}
	for _, id := range sortedKeyIDs(idx.PreviousKeys) {
		decoded, err := decodeKey(idx.PreviousKeys[id])
		if err != nil {
			return nil, fmt.Errorf("kunci indeks lama %s: %w", id, err)
		}
		if err := c.addIndexKey(id, decoded); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// addIndexKey mendaftarkan satu versi kunci indeks. Id yang sama dengan material
// berbeda ditolak karena berarti konfigurasi menimpa kunci yang sudah ada;
// pesannya hanya menyebut id, bukan nilai kunci.
func (c *Cipher) addIndexKey(id string, key []byte) error {
	if existing, ok := c.indexKeys[id]; ok {
		if !bytes.Equal(existing, key) {
			return fmt.Errorf("versi kunci indeks %q dikonfigurasi dengan kunci berbeda", id)
		}
		return nil
	}
	c.indexKeys[id] = key
	c.indexKeyOrder = append(c.indexKeyOrder, id)
	return nil
}

// decodeKey memvalidasi kunci base64 32 byte. Pesan galat tidak pernah memuat nilai
// kunci, hanya kelas kesalahannya.
func decodeKey(b64 string) ([]byte, error) {
	if b64 == "" {
		return nil, ErrMissingMasterKey
	}
	key, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, ErrInvalidKeyEncoding
	}
	if len(key) != masterKeySize {
		return nil, ErrInvalidKeySize
	}
	return key, nil
}

// sortedKeyIDs mengembalikan id kunci terurut agar kandidat indeks deterministik
// dan mudah diuji; peta Go tidak menjamin urutan iterasi.
func sortedKeyIDs(keys map[string]string) []string {
	ids := make([]string, 0, len(keys))
	for id := range keys {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// IndexKeyVersion mengembalikan versi kunci indeks aktif. Nilai ini ditulis ke
// kolom index_key_version pada baris baru, sehingga rotasi berikutnya tahu baris
// mana yang masih memakai kunci lama.
func (c *Cipher) IndexKeyVersion() string {
	return c.indexKeyID
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
//
// Fungsi ini memakai kunci indeks AKTIF. Bila kunci indeks terpisah tidak
// dikonfigurasi, kuncinya adalah master key enkripsi dan hasilnya identik dengan
// sebelum migrasi 000048.
func (c *Cipher) BlindIndex(value string) string {
	mac := hmac.New(sha256.New, c.indexKeys[c.indexKeyID])
	mac.Write([]byte("blind-index:"))
	mac.Write([]byte(strings.TrimSpace(strings.ToUpper(value))))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// NameTokenIndex menghasilkan blind index untuk SATU kata nama. Mekanismenya sama
// dengan BlindIndex (HMAC-SHA256 dengan kunci indeks aktif, hasil base64), hanya
// berbeda awalan domain ("name-token:") agar indeks token nama tidak pernah
// bertabrakan dengan blind index nilai utuh seperti NIK/email. Pemisahan domain ini
// penting: memakai ulang indeks yang sama untuk konteks berbeda membuat satu nilai
// dapat dicocokkan di tempat yang tidak dimaksudkan.
//
// Token sudah dinormalkan pemanggil (huruf kecil, tanpa gelar) lewat
// domain.NormalizeNameTokens; fungsi ini mengulang trim/lower sebagai pengaman.
func (c *Cipher) NameTokenIndex(token string) string {
	mac := hmac.New(sha256.New, c.indexKeys[c.indexKeyID])
	mac.Write([]byte("name-token:"))
	mac.Write([]byte(strings.ToLower(strings.TrimSpace(token))))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// BlindIndexCandidates mengembalikan blind index nilai untuk SEMUA versi kunci
// indeks yang dikenal. Inilah yang menyatukan hasil pencarian lintas versi: baris
// lama (kunci enkripsi) dan baris baru (kunci indeks terpisah) sama-sama tercakup.
// Nilai untuk versi aktif selalu ada, dan duplikat dibuang agar query ringkas.
func (c *Cipher) BlindIndexCandidates(value string) []string {
	normalized := strings.TrimSpace(strings.ToUpper(value))
	return c.indexCandidates("blind-index:", normalized)
}

// NameTokenIndexCandidates mengembalikan indeks satu kata nama untuk semua versi
// kunci indeks yang dikenal. Pemanggil menggabungkannya per kata dengan AND.
func (c *Cipher) NameTokenIndexCandidates(token string) []string {
	normalized := strings.ToLower(strings.TrimSpace(token))
	return c.indexCandidates("name-token:", normalized)
}

func (c *Cipher) indexCandidates(domainPrefix, normalized string) []string {
	out := make([]string, 0, len(c.indexKeyOrder))
	seen := make(map[string]struct{}, len(c.indexKeyOrder))
	for _, id := range c.indexKeyOrder {
		mac := hmac.New(sha256.New, c.indexKeys[id])
		mac.Write([]byte(domainPrefix))
		mac.Write([]byte(normalized))
		index := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		if _, ok := seen[index]; ok {
			continue
		}
		seen[index] = struct{}{}
		out = append(out, index)
	}
	return out
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
