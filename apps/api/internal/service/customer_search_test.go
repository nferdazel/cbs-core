package service

import (
	"encoding/base64"
	"reflect"
	"testing"

	"cbs-core/apps/core-api/internal/crypto"
	"cbs-core/apps/core-api/internal/domain"
)

func searchTestCipher(t *testing.T) *crypto.Cipher {
	t.Helper()
	c, err := crypto.NewCipher("k1", base64.StdEncoding.EncodeToString(make([]byte, 32)), nil)
	if err != nil {
		t.Fatalf("membangun cipher: %v", err)
	}
	return c
}

// Hanya kata kunci berbentuk NIK (tepat 16 digit) yang boleh dihitung blind
// index-nya. Bentuk lain harus dilewati, bukan dipaksa menjadi hash yang tidak akan
// pernah cocok dengan data mana pun.
func TestIsNIK(t *testing.T) {
	cases := []struct {
		term string
		want bool
	}{
		{"3201234567890001", true},
		{"0000000000000000", true},
		{"320123456789000", false},   // 15 digit
		{"32012345678900012", false}, // 17 digit
		{"320123456789000a", false},  // ada huruf
		{"", false},
		{"CIF-0001", false},
		{"budi", false},
	}

	for _, tc := range cases {
		if got := isNIK(tc.term); got != tc.want {
			t.Fatalf("isNIK(%q) = %v, ingin %v", tc.term, got, tc.want)
		}
	}
}

// Kata kunci NIK dicari lewat blind index SAJA. Nomor CIF tidak pernah berbentuk 16
// digit, sehingga bila filter CIF ikut diisi dan digabung dengan AND, seluruh hasil
// pasti tersaring habis — pencarian NIK selalu kosong.
func TestSearchQuery_ForNIKUsesBlindIndexOnly(t *testing.T) {
	cipher := searchTestCipher(t)
	svc := &customerService{cipher: cipher}

	const nik = "3201234567890001"
	q := svc.searchQuery(nik)

	if q.CIF != "" {
		t.Fatalf("kata kunci NIK tidak boleh ikut menyaring nomor CIF, dapat %q", q.CIF)
	}
	if want := cipher.BlindIndex(nik); q.IDCardIndex != want {
		t.Fatalf("blind index NIK = %q, ingin %q", q.IDCardIndex, want)
	}
}

// Kata kunci non-NIK dicari sebagai awalan nomor CIF, tanpa menyentuh blind index.
func TestSearchQuery_ForCIFUsesPrefixOnly(t *testing.T) {
	svc := &customerService{cipher: searchTestCipher(t)}

	q := svc.searchQuery("  CIF000000123  ")
	if q.CIF != "CIF000000123" {
		t.Fatalf("CIF harus dipangkas spasi, dapat %q", q.CIF)
	}
	if q.IDCardIndex != "" {
		t.Fatalf("bukan NIK tidak boleh jadi blind index, dapat %q", q.IDCardIndex)
	}

	if empty := svc.searchQuery("   "); !reflect.DeepEqual(empty, domain.CustomerQuery{}) {
		t.Fatalf("kata kunci kosong harus menghasilkan filter kosong, dapat %+v", empty)
	}
}

// Tanpa cipher, kata kunci NIK tidak boleh menghasilkan blind index (akan panik bila
// dipaksa); pencarian jatuh ke awalan CIF apa adanya.
func TestSearchQuery_WithoutCipherFallsBackToCIF(t *testing.T) {
	svc := &customerService{}

	q := svc.searchQuery("3201234567890001")
	if q.IDCardIndex != "" {
		t.Fatalf("tanpa cipher tidak boleh ada blind index, dapat %q", q.IDCardIndex)
	}
	if q.CIF != "3201234567890001" {
		t.Fatalf("jatuh ke CIF apa adanya, dapat %q", q.CIF)
	}
}

// Kata kunci nama dipecah menjadi token dan diterjemahkan ke blind index tiap kata.
// CIF dan NIK tidak boleh ikut diisi: menggabungkannya dengan AND akan menyaring
// habis hasilnya.
func TestSearchQuery_ForNameUsesTokenIndexes(t *testing.T) {
	cipher := searchTestCipher(t)
	svc := &customerService{cipher: cipher}

	q := svc.searchQuery("  Siti  Rahayu ")
	if q.CIF != "" || q.IDCardIndex != "" {
		t.Fatalf("kata kunci nama tidak boleh menyaring CIF/NIK, dapat %+v", q)
	}
	want := []string{cipher.NameTokenIndex("siti"), cipher.NameTokenIndex("rahayu")}
	if !reflect.DeepEqual(q.NameTokenIndexes, want) {
		t.Fatalf("token nama = %v, ingin %v", q.NameTokenIndexes, want)
	}
}

// Gelar tidak membedakan nasabah, jadi kueri bergelar harus menghasilkan token yang
// sama dengan nama telanjangnya.
func TestSearchQuery_NameWithTitleMatchesPlainName(t *testing.T) {
	cipher := searchTestCipher(t)
	svc := &customerService{cipher: cipher}

	withTitle := svc.searchQuery("Hj. Siti")
	plain := svc.searchQuery("siti")
	if !reflect.DeepEqual(withTitle, plain) {
		t.Fatalf("kueri bergelar = %+v, ingin sama dengan %+v", withTitle, plain)
	}
}

// Kueri yang hanya berisi tanda baca atau gelar tidak menghasilkan token apa pun,
// sehingga tidak menyaring dan tidak salah mencocokkan seluruh nasabah.
func TestSearchQuery_PunctuationOnlyFindsNothing(t *testing.T) {
	svc := &customerService{cipher: searchTestCipher(t)}

	for _, term := range []string{"!!! ???", "---", "Hj."} {
		if q := svc.searchQuery(term); !reflect.DeepEqual(q, domain.CustomerQuery{}) {
			t.Fatalf("kueri %q harus tanpa filter, dapat %+v", term, q)
		}
	}
}

// Jalur tulis wajib mengisi token nama, jika tidak pencarian nama tidak akan pernah
// menemukan nasabah baru meski sudah dibuat.
func TestEncryptInput_IncludesNameTokens(t *testing.T) {
	cipher := searchTestCipher(t)
	svc := &customerService{cipher: cipher}

	record, err := svc.encryptInput(domain.CreateCustomerInput{FullName: "Hj. Siti Rahayu"})
	if err != nil {
		t.Fatalf("encryptInput: %v", err)
	}
	want := []string{cipher.NameTokenIndex("siti"), cipher.NameTokenIndex("rahayu")}
	if !reflect.DeepEqual(record.NameTokenIndexes, want) {
		t.Fatalf("token nama = %v, ingin %v", record.NameTokenIndexes, want)
	}
}

// Nama kosong/tidak sah tidak boleh menyisipkan token sampah.
func TestEncryptInput_NoTokensForInvalidName(t *testing.T) {
	svc := &customerService{cipher: searchTestCipher(t)}

	for _, name := range []string{"", "   ", "!!!", "A."} {
		record, err := svc.encryptInput(domain.CreateCustomerInput{FullName: name})
		if err != nil {
			t.Fatalf("encryptInput(%q): %v", name, err)
		}
		if len(record.NameTokenIndexes) != 0 {
			t.Fatalf("nama %q menghasilkan token sampah %v", name, record.NameTokenIndexes)
		}
	}
}
