package domain

import "context"

// Kunci konfigurasi identitas branding aplikasi. Nama PT bukan kunci di sini:
// sumbernya tabel bank_profile (bank_name) agar hanya ada satu sumber kebenaran,
// sama seperti dokumen cetak. Kunci-kunci ini hanya untuk branding yang khas
// aplikasi, bukan identitas badan hukum.
const (
	ConfigKeyBrandingDisplayName = "branding.display_name"
	ConfigKeyBrandingShortName   = "branding.short_name"
	ConfigKeyBrandingDescription = "branding.description"
	ConfigKeyBrandingLogoURL     = "branding.logo_url"
)

// Nilai bawaan branding mereproduksi tampilan sebelum identitas menjadi dinamis.
// Dipakai hanya bila kunci tidak ada/rusak; nilai yang di-seed migrasi 000075
// sama dengan nilai ini sehingga tidak ada perubahan visual tak sengaja.
const (
	DefaultBrandingDisplayName = "CBS Core Backoffice"
	DefaultBrandingShortName   = "CBS Core"
	DefaultBrandingDescription = "Core Banking & Akuntansi Double-Entry"
)

// AppInfo adalah identitas aplikasi yang boleh dibaca TANPA login: dipakai halaman
// login dan metadata/judul tab web. Isinya sengaja dibatasi pada identitas publik;
// jangan menambah data internal (mis. daftar buku, konfigurasi, statistik) ke sini.
type AppInfo struct {
	// CompanyName adalah nama PT dari bank_profile.bank_name. Kosong berarti bank
	// belum mengisi profil; pemanggil tidak boleh mengarang nama.
	CompanyName string `json:"company_name"`
	// DisplayName adalah nama aplikasi yang tampil di header dan judul tab.
	DisplayName string `json:"display_name"`
	// ShortName adalah nama pendek untuk ruang sempit (mis. ikon/tab).
	ShortName string `json:"short_name"`
	// Description adalah keterangan singkat aplikasi.
	Description string `json:"description"`
	// LogoURL adalah URL logo; kosong berarti web memakai ikon bawaannya.
	LogoURL string `json:"logo_url"`
}

// AppInfoService menyusun identitas aplikasi untuk endpoint publik.
type AppInfoService interface {
	Get(ctx context.Context) AppInfo
}
