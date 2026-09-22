package service_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
	"github.com/shopspring/decimal"
)

// stubAppInfoConfig adalah SystemConfigService in-memory berbasis string. Dipakai
// uji identitas aplikasi: kunci yang tidak ada jatuh ke fallback yang diberikan.
type stubAppInfoConfig struct {
	values map[string]string
}

func (s *stubAppInfoConfig) GetString(_ context.Context, key, fallback string) string {
	if v, ok := s.values[key]; ok {
		return v
	}
	return fallback
}
func (s *stubAppInfoConfig) GetDecimal(context.Context, string, decimal.Decimal) decimal.Decimal {
	return decimal.Zero
}
func (s *stubAppInfoConfig) GetInt(context.Context, string, int) int    { return 0 }
func (s *stubAppInfoConfig) GetBool(context.Context, string, bool) bool { return false }
func (s *stubAppInfoConfig) Invalidate(string)                          {}

var _ domain.SystemConfigService = (*stubAppInfoConfig)(nil)

// TestAppInfoServiceMengambilDariSumber membuktikan identitas datang dari
// bank_profile (nama PT) dan kunci branding system_config, bukan literal kode.
func TestAppInfoServiceMengambilDariSumber(t *testing.T) {
	bank := &stubBankProfileRepo{profile: &domain.BankProfile{Name: "BPR Contoh Sejahtera"}}
	config := &stubAppInfoConfig{values: map[string]string{
		domain.ConfigKeyBrandingDisplayName: "Aplikasi Bank Contoh",
		domain.ConfigKeyBrandingShortName:   "ABC",
		domain.ConfigKeyBrandingDescription: "Deskripsi Bank Contoh",
		domain.ConfigKeyBrandingLogoURL:     "https://contoh.local/logo.png",
	}}
	svc := service.NewAppInfoService(bank, config)

	got := svc.Get(context.Background())
	if got.CompanyName != "BPR Contoh Sejahtera" {
		t.Errorf("company_name = %q, mau dari bank_profile", got.CompanyName)
	}
	if got.DisplayName != "Aplikasi Bank Contoh" {
		t.Errorf("display_name = %q, mau dari konfigurasi", got.DisplayName)
	}
	if got.ShortName != "ABC" {
		t.Errorf("short_name = %q, mau dari konfigurasi", got.ShortName)
	}
	if got.Description != "Deskripsi Bank Contoh" {
		t.Errorf("description = %q, mau dari konfigurasi", got.Description)
	}
	if got.LogoURL != "https://contoh.local/logo.png" {
		t.Errorf("logo_url = %q, mau dari konfigurasi", got.LogoURL)
	}
}

// TestAppInfoServiceBawaanReproduksiTampilanLama membuktikan kunci yang belum ada
// (instalasi belum di-provision) menghasilkan identitas yang sama dengan tampilan
// sebelum dinamis: "CBS Core Backoffice" / "Core Banking & Akuntansi Double-Entry".
func TestAppInfoServiceBawaanReproduksiTampilanLama(t *testing.T) {
	svc := service.NewAppInfoService(&stubBankProfileRepo{}, &stubAppInfoConfig{values: map[string]string{}})

	got := svc.Get(context.Background())
	if got.DisplayName != domain.DefaultBrandingDisplayName {
		t.Errorf("display_name bawaan = %q, mau %q", got.DisplayName, domain.DefaultBrandingDisplayName)
	}
	if got.ShortName != domain.DefaultBrandingShortName {
		t.Errorf("short_name bawaan = %q, mau %q", got.ShortName, domain.DefaultBrandingShortName)
	}
	if got.Description != domain.DefaultBrandingDescription {
		t.Errorf("description bawaan = %q, mau %q", got.Description, domain.DefaultBrandingDescription)
	}
	if got.CompanyName != "" {
		t.Errorf("company_name = %q, mau kosong bila profil bank belum diisi", got.CompanyName)
	}
}

// TestAppInfoServiceKunciKosongPakaiBawaan menjaga judul tab/header tidak pernah
// kosong walau operator sengaja mengosongkan kunci branding.
func TestAppInfoServiceKunciKosongPakaiBawaan(t *testing.T) {
	config := &stubAppInfoConfig{values: map[string]string{
		domain.ConfigKeyBrandingDisplayName: "   ",
		domain.ConfigKeyBrandingDescription: "",
	}}
	svc := service.NewAppInfoService(&stubBankProfileRepo{}, config)

	got := svc.Get(context.Background())
	if got.DisplayName != domain.DefaultBrandingDisplayName {
		t.Errorf("display_name kosong = %q, mau bawaan", got.DisplayName)
	}
	if got.Description != domain.DefaultBrandingDescription {
		t.Errorf("description kosong = %q, mau bawaan", got.Description)
	}
}

// TestAppInfoServiceCacheMenghematPembacaan membuktikan hasil di-cache: perubahan
// konfigurasi tidak terlihat pada instance yang sama sampai TTL habis, sedangkan
// instance baru (perilaku setelah TTL/restart) membaca nilai terbaru.
func TestAppInfoServiceCacheMenghematPembacaan(t *testing.T) {
	bank := &stubBankProfileRepo{profile: &domain.BankProfile{Name: "Bank Lama"}}
	config := &stubAppInfoConfig{values: map[string]string{
		domain.ConfigKeyBrandingDisplayName: "Nama Lama",
	}}
	svc := service.NewAppInfoService(bank, config)

	if got := svc.Get(context.Background()).DisplayName; got != "Nama Lama" {
		t.Fatalf("pembacaan pertama = %q, mau Nama Lama", got)
	}

	// Ubah sumber di belakang service: instance yang sama harus tetap memakai cache.
	config.values[domain.ConfigKeyBrandingDisplayName] = "Nama Baru"
	bank.profile.Name = "Bank Baru"
	cached := svc.Get(context.Background())
	if cached.DisplayName != "Nama Lama" || cached.CompanyName != "Bank Lama" {
		t.Fatalf("instance lama membaca ulang (%+v), cache tidak bekerja", cached)
	}

	// Instance baru membaca nilai terbaru: tidak ada yang tertahan permanen.
	fresh := service.NewAppInfoService(bank, config).Get(context.Background())
	if fresh.DisplayName != "Nama Baru" || fresh.CompanyName != "Bank Baru" {
		t.Fatalf("instance baru = %+v, mau Nama Baru/Bank Baru", fresh)
	}
}

// TestAppInfoServicePeringatanProfilKosong memastikan profil yang belum diisi
// tetap membuat endpoint berjalan (nama PT kosong, tidak panik) TETAPI meninggalkan
// peringatan jelas di log agar instalasi tanpa identitas tidak lolos tanpa terlihat.
func TestAppInfoServicePeringatanProfilKosong(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(old)

	svc := service.NewAppInfoService(&stubBankProfileRepo{}, &stubAppInfoConfig{values: map[string]string{}})
	got := svc.Get(context.Background())

	if got.CompanyName != "" {
		t.Fatalf("company_name = %q, mau kosong saat profil belum diisi", got.CompanyName)
	}
	logged := buf.String()
	if !strings.Contains(logged, "bank_profile") || !strings.Contains(logged, "belum diisi") {
		t.Fatalf("peringatan profil kosong tidak muncul di log: %q", logged)
	}
}

// TestAppInfoServiceBankProfileErrorTidakMenggagalkanEndpoint memastikan kegagalan
// membaca bank_profile hanya mengosongkan nama PT, bukan membuat endpoint gagal.
func TestAppInfoServiceBankProfileErrorTidakMenggagalkanEndpoint(t *testing.T) {
	bank := &stubBankProfileRepo{err: fmt.Errorf("database tidak dapat dihubungi")}
	svc := service.NewAppInfoService(bank, &stubAppInfoConfig{values: map[string]string{}})

	got := svc.Get(context.Background())
	if got.DisplayName != domain.DefaultBrandingDisplayName {
		t.Errorf("display_name = %q, mau tetap bawaan", got.DisplayName)
	}
	if got.CompanyName != "" {
		t.Errorf("company_name = %q, mau kosong saat pembacaan gagal", got.CompanyName)
	}
}
