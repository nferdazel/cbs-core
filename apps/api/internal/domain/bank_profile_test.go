package domain

import (
	"errors"
	"testing"
)

// Profil bank tanpa nama harus ditolak: nama adalah identitas minimum yang dipakai
// dokumen dan endpoint /app-info. Inilah inti W16 — profil kosong tidak boleh
// tersimpan sebagai "berhasil".
func TestValidateBankProfileTolakNamaKosong(t *testing.T) {
	for _, name := range []string{"", "   ", "\t"} {
		p := &BankProfile{Name: name, City: "Jakarta"}
		if err := ValidateBankProfile(p); !errors.Is(err, ErrBankProfileNameRequired) {
			t.Fatalf("nama %q: err = %v, ingin ErrBankProfileNameRequired", name, err)
		}
	}
}

// Nama terlalu pendek atau terlalu panjang bukan identitas badan hukum yang wajar.
func TestValidateBankProfileTolakNamaTidakWajar(t *testing.T) {
	if err := ValidateBankProfile(&BankProfile{Name: "AB"}); !errors.Is(err, ErrBankProfileNameTooShort) {
		t.Fatalf("nama 2 huruf: err = %v, ingin ErrBankProfileNameTooShort", err)
	}
	long := make([]rune, BankProfileNameMaxLen+1)
	for i := range long {
		long[i] = 'A'
	}
	if err := ValidateBankProfile(&BankProfile{Name: string(long)}); !errors.Is(err, ErrBankProfileFieldTooLong) {
		t.Fatalf("nama %d huruf: err = %v, ingin ErrBankProfileFieldTooLong", len(long), err)
	}
}

// Karakter yang tidak mungkin ada pada NPWP/telepon ditolak dengan pesan jelas,
// sedangkan format yang wajar diterima dan spasi tepi dirapikan.
func TestValidateBankProfileKarakterIdentitas(t *testing.T) {
	ok := &BankProfile{Name: "  BPR Syariah Contoh  ", NPWP: "01.234.567.8-901.000", Phone: "+62 (21) 555-1234"}
	if err := ValidateBankProfile(ok); err != nil {
		t.Fatalf("identitas wajar ditolak: %v", err)
	}
	if ok.Name != "BPR Syariah Contoh" || ok.NPWP != "01.234.567.8-901.000" {
		t.Fatalf("profil tidak dirapikan: %+v", ok)
	}

	if err := ValidateBankProfile(&BankProfile{Name: "Bank Contoh", NPWP: "abc!"}); !errors.Is(err, ErrBankProfileNPWPInvalid) {
		t.Fatalf("NPWP tidak sah: err = %v, ingin ErrBankProfileNPWPInvalid", err)
	}
	if err := ValidateBankProfile(&BankProfile{Name: "Bank Contoh", Phone: "0812#999"}); !errors.Is(err, ErrBankProfilePhoneInvalid) {
		t.Fatalf("telepon tidak sah: err = %v, ingin ErrBankProfilePhoneInvalid", err)
	}
}
