package ojkreport

import "testing"

// TestForm06KolomSandiReferensiOJK membuktikan kolom XVIII (Jenis Debitur/Pihak
// Lawan, Lampiran 02) dan XX (Sektor Ekonomi, Lampiran 05) kini terisi sandi
// referensi OJK per nasabah, dan menulis "-" bila bank belum mengisinya. Keduanya
// tidak lagi terdaftar sebagai kolom belum tersedia.
func TestForm06KolomSandiReferensiOJK(t *testing.T) {
	rows := []LoanRow{
		{
			Status:               "DISBURSED",
			LoanNumber:           "LN-ISI",
			OJKPihakLawanCode:    "860",
			OJKSektorEkonomiCode: "A00000",
		},
		{Status: "DISBURSED", LoanNumber: "LN-KOSONG"},
	}
	sec := buildForm06(rows)

	if got := findCell(t, sec, "LN-ISI", form06SandiJenisDebitur).Value; got != "860" {
		t.Errorf("kolom XVIII = %q, ingin 860", got)
	}
	if got := findCell(t, sec, "LN-ISI", form06SandiSektor).Value; got != "A00000" {
		t.Errorf("kolom XX = %q, ingin A00000", got)
	}
	if got := findCell(t, sec, "LN-KOSONG", form06SandiJenisDebitur).Value; got != "-" {
		t.Errorf("kolom XVIII kosong = %q, ingin -", got)
	}
	if got := findCell(t, sec, "LN-KOSONG", form06SandiSektor).Value; got != "-" {
		t.Errorf("kolom XX kosong = %q, ingin -", got)
	}
	for _, u := range sec.Unavailable {
		if u.Sandi == form06SandiJenisDebitur || u.Sandi == form06SandiSektor {
			t.Errorf("kolom %s masih terdaftar tidak tersedia: %s", u.Sandi, u.Reason)
		}
	}
}
