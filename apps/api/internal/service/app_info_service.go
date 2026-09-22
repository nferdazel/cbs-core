package service

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// appInfoCacheTTL adalah masa berlaku cache identitas aplikasi. Identitas dibaca
// halaman login dan metadata web pada setiap permintaan, jadi hasil rakitannya
// di-cache agar tidak menjadi satu query bank_profile tiap kali. Nilai yang sama
// juga di-cache per kunci oleh SystemConfigService; cache di sini menambah
// pembacaan bank_profile ke dalam satu hasil yang siap dikirim.
const appInfoCacheTTL = 60 * time.Second

type appInfoService struct {
	bankProfiles domain.BankProfileRepository
	config       domain.SystemConfigService
	mu           sync.Mutex
	cached       *domain.AppInfo
	expiresAt    time.Time
}

// NewAppInfoService membangun penyusun identitas aplikasi. Repositori bank_profile
// boleh nil (mis. uji): nama PT lalu kosong, bukan dikarang.
func NewAppInfoService(
	bankProfiles domain.BankProfileRepository,
	config domain.SystemConfigService,
) domain.AppInfoService {
	return &appInfoService{bankProfiles: bankProfiles, config: config}
}

// Get mengembalikan identitas aplikasi, memakai cache TTL agar tidak membaca
// database tiap permintaan. Kegagalan membaca bank_profile TIDAK menggagalkan
// endpoint publik: nama PT dibiarkan kosong dan dicatat sebagai peringatan, karena
// halaman login harus tetap dapat tampil walau profil bank belum diisi.
func (s *appInfoService) Get(ctx context.Context) domain.AppInfo {
	s.mu.Lock()
	if s.cached != nil && time.Now().Before(s.expiresAt) {
		info := *s.cached
		s.mu.Unlock()
		return info
	}
	s.mu.Unlock()

	info := s.build(ctx)

	s.mu.Lock()
	s.cached = &info
	s.expiresAt = time.Now().Add(appInfoCacheTTL)
	s.mu.Unlock()
	return info
}

func (s *appInfoService) build(ctx context.Context) domain.AppInfo {
	info := domain.AppInfo{
		DisplayName: s.getConfig(ctx, domain.ConfigKeyBrandingDisplayName, domain.DefaultBrandingDisplayName),
		ShortName:   s.getConfig(ctx, domain.ConfigKeyBrandingShortName, domain.DefaultBrandingShortName),
		Description: s.getConfig(ctx, domain.ConfigKeyBrandingDescription, domain.DefaultBrandingDescription),
		LogoURL:     s.getConfig(ctx, domain.ConfigKeyBrandingLogoURL, ""),
	}

	if s.bankProfiles != nil {
		profile, err := s.bankProfiles.Get(ctx)
		switch {
		case err != nil:
			slog.WarnContext(ctx, "gagal membaca bank_profile untuk identitas aplikasi; nama PT dikosongkan", "error", err)
		case profile == nil || strings.TrimSpace(profile.Name) == "":
			// Profil kosong tetap tidak menggagalkan endpoint publik (halaman login
			// harus tampil), tetapi WAJIB berisik di log: inilah gejala instalasi
			// yang belum mengisi identitas, dan perbaikannya bukan lewat SQL.
			slog.WarnContext(ctx,
				"profil bank belum diisi (bank_profile.bank_name kosong); perusahaan pada identitas aplikasi dan dokumen tampil kosong. Isi lewat Pengaturan > Identitas Bank (PUT /api/v1/system/bank-profile).",
				"sumber", "bank_profile")
		case profile != nil:
			info.CompanyName = profile.Name
		}
	}
	return normalizeAppInfo(info)
}

// getConfig membaca kunci branding dengan fallback nilai bawaan. SystemConfigService
// mengembalikan fallback untuk kunci yang hilang/rusak, sehingga instalasi yang
// belum di-provision tetap menampilkan identitas lama.
func (s *appInfoService) getConfig(ctx context.Context, key, fallback string) string {
	if s.config == nil {
		return fallback
	}
	return s.config.GetString(ctx, key, fallback)
}

// normalizeAppInfo merapikan spasi dan mengisi nilai kosong dengan bawaan agar
// judul tab atau header tidak pernah kosong.
func normalizeAppInfo(info domain.AppInfo) domain.AppInfo {
	info.CompanyName = strings.TrimSpace(info.CompanyName)
	info.DisplayName = strings.TrimSpace(info.DisplayName)
	info.ShortName = strings.TrimSpace(info.ShortName)
	info.Description = strings.TrimSpace(info.Description)
	info.LogoURL = strings.TrimSpace(info.LogoURL)
	if info.DisplayName == "" {
		info.DisplayName = domain.DefaultBrandingDisplayName
	}
	if info.ShortName == "" {
		info.ShortName = domain.DefaultBrandingShortName
	}
	if info.Description == "" {
		info.Description = domain.DefaultBrandingDescription
	}
	return info
}
