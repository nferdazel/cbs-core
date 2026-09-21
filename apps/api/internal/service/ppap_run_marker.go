package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// cfgPPAPLastRunBusinessDate adalah kunci system_config yang menyimpan tanggal bisnis
// run PPAP terakhir yang berhasil.
//
// ALASAN disimpan di system_config, bukan tabel/kolom baru: penanda ini adalah satu
// fakta global per bank pada satu waktu (bukan per kredit dan bukan riwayat), sehingga
// satu baris konfigurasi sudah cukup. Jalur baca yang sudah ada (SystemConfigRepository)
// dipakai ulang, tidak ada tabel yang perlu di-join, dan migrasi 000052 hanya perlu
// menyemai kuncinya. Yang lebih penting: karena penanda dibaca langsung dari
// repositori (bukan lewat cache SystemConfigService yang berlaku 60 detik), CKPN
// selalu melihat tanggal run PPAP terakhir tanpa risiko tertahan cache.
const cfgPPAPLastRunBusinessDate = "ppap.last_run_business_date"

// layoutTanggalBisnis adalah format tanggal yang disimpan dan dibandingkan. Tanggal
// bisnis tidak membawa jam/zona, jadi perbandingannya dilakukan pada tingkat hari.
const layoutTanggalBisnis = "2006-01-02"

type ppapRunMarker struct {
	repo domain.SystemConfigRepository
}

// NewPPAPRunMarker membuat penanda run PPAP di atas penyimpanan system_config.
func NewPPAPRunMarker(repo domain.SystemConfigRepository) domain.PPAPRunMarker {
	return &ppapRunMarker{repo: repo}
}

// RecordRun menyimpan tanggal bisnis run PPAP. updatedBy adalah staf pelaksana run
// (dari actor), bukan uuid.Nil: system_config.updated_by ber-FK ke staff_users,
// sehingga uuid.Nil ditolak database dan membuat run PPAP yang sudah selesai
// menghitung gagal hanya karena penandanya tidak dapat ditulis.
func (m *ppapRunMarker) RecordRun(ctx context.Context, businessDate time.Time, updatedBy uuid.UUID) error {
	if m == nil || m.repo == nil {
		return nil
	}
	day := time.Date(businessDate.Year(), businessDate.Month(), businessDate.Day(), 0, 0, 0, 0, time.UTC)
	if err := m.repo.Set(ctx, cfgPPAPLastRunBusinessDate, day.Format(layoutTanggalBisnis), updatedBy); err != nil {
		return fmt.Errorf("menyimpan penanda run PPAP: %w", err)
	}
	return nil
}

// LastRunBusinessDate membaca tanggal bisnis run PPAP terakhir. Kunci yang belum ada
// atau kosong berarti belum pernah ada run PPAP yang berhasil dan dikembalikan sebagai
// ok=false, bukan error: pemanggil (CKPN) yang memutuskan menolak. Nilai yang ada
// tetapi tidak dapat diurai adalah penanda rusak dan dikembalikan sebagai error agar
// tidak diperlakukan sebagai "tidak ada".
func (m *ppapRunMarker) LastRunBusinessDate(ctx context.Context) (time.Time, bool, error) {
	if m == nil || m.repo == nil {
		return time.Time{}, false, nil
	}
	raw, err := m.repo.Get(ctx, cfgPPAPLastRunBusinessDate)
	if err != nil {
		// Kunci belum pernah ditulis (mis. database yang belum menjalankan PPAP):
		// bukan kegagalan teknis, cukup "belum ada run".
		return time.Time{}, false, nil
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false, nil
	}
	day, perr := time.Parse(layoutTanggalBisnis, raw)
	if perr != nil {
		return time.Time{}, false, fmt.Errorf("%w: penanda tanggal bisnis run PPAP terakhir %q tidak dapat diurai",
			domain.ErrCKPNStalePPAP, raw)
	}
	return day, true, nil
}

var _ domain.PPAPRunMarker = (*ppapRunMarker)(nil)
