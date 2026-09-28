package service

import (
	"context"
	"database/sql"

	"cbs-core/apps/core-api/internal/domain"
)

// ckpnPABLActivationService mengelola pengaturan enam kunci ckpn.pabl.* (fraksi PD/LGD
// kolektif, bentuk CKPN aset baik, saklar ckpn.pabl.enabled). Perubahan dan auditnya
// berada dalam SATU transaksi, seperti jalur aktivasi CKPN utama, sehingga audit tidak
// mencatat perubahan yang batal disimpan.
type ckpnPABLActivationService struct {
	repo domain.CKPNPABLActivationStore
	// config adalah konfigurasi CKPN utama, dipakai menilai kesiapan penyalakan
	// ckpn.pabl.enabled (parameter FINAL, ratifikasi, PD/LGD kredit). Boleh nil pada uji.
	config domain.SystemConfigService
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewCKPNPABLActivationService merakit layanan pengaturan CKPN PABL. config dipakai
// membuang cache kunci yang berubah agar nilainya langsung berlaku pada proses yang
// sedang berjalan; boleh nil pada uji tanpa konfigurasi. auditSinks variadik mengikuti
// pola layanan lain: nil berarti audit dilewati tanpa menggagalkan aksi.
func NewCKPNPABLActivationService(db *sql.DB, repo domain.CKPNPABLActivationStore,
	config domain.SystemConfigService, auditSinks ...domain.AuditRepository) domain.CKPNPABLActivationService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &ckpnPABLActivationService{repo: repo, config: config, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// Get mengembalikan nilai terkini beserta penahan penyalakan. Seluruhnya baca-saja.
func (s *ckpnPABLActivationService) Get(ctx context.Context) (*domain.CKPNPABLActivation, error) {
	values, err := s.repo.Get(ctx)
	if err != nil {
		return nil, err
	}
	return buildCKPNPABLActivation(ctx, values, s.config), nil
}

// Update memvalidasi hasil akhir, menyimpan perubahan, dan menulis audit dalam satu
// transaksi. Bidang yang tidak dikirim tidak diubah; bidang yang nilainya sama tidak
// menghasilkan baris/audit (PUT idempoten).
func (s *ckpnPABLActivationService) Update(ctx context.Context, input domain.UpdateCKPNPABLActivationInput, actor domain.Actor) (*domain.CKPNPABLActivation, error) {
	if input.IsEmpty() {
		return nil, domain.ErrCKPNPABLActivationEmpty
	}

	var (
		result      *domain.CKPNPABLActivation
		changedKeys map[string]string
	)
	err := s.runner.Run(ctx, func(tx any) error {
		current, err := s.repo.GetTx(ctx, tx)
		if err != nil {
			return err
		}
		changes := input.Changes(current)
		changedKeys = changes
		if len(changes) == 0 {
			result = buildCKPNPABLActivation(ctx, current, s.config)
			return nil
		}

		merged := make(map[string]string, len(current)+len(changes))
		for _, key := range domain.CKPNPABLActivationKeys() {
			merged[key] = current[key]
		}
		for key, value := range changes {
			merged[key] = value
		}
		if err := domain.ValidateCKPNPABLActivation(ctx, merged, s.config); err != nil {
			return err
		}

		if err := s.repo.SaveTx(ctx, tx, changes, actor.UserID); err != nil {
			return err
		}
		if err := writeAudit(ctx, s.audit, tx, actor, "UPDATE_CKPN_PABL_ACTIVATION", "ckpn_pabl_activation", "1",
			ckpnPABLAuditChanges(current, changes)); err != nil {
			return err
		}
		result = buildCKPNPABLActivation(ctx, merged, s.config)
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Cache proses dibuang SETELAH commit: bila tulis dibatalkan, tidak ada nilai yang
	// perlu dibuang, dan bila berhasil, EOD/endpoint berikutnya membaca nilai baru.
	for key := range changedKeys {
		if s.config != nil {
			s.config.Invalidate(key)
		}
	}
	return result, nil
}

// buildCKPNPABLActivation membentuk snapshot baca-saja dari sekumpulan nilai. Penahan
// penyalakan dinilai atas NILAI TERSIMPAN tanpa memandang saklar, agar pengguna melihat
// sisa pekerjaan walau PABL mati.
func buildCKPNPABLActivation(ctx context.Context, values map[string]string, main domain.SystemConfigService) *domain.CKPNPABLActivation {
	full := make(map[string]string, len(domain.CKPNPABLActivationKeys()))
	for _, key := range domain.CKPNPABLActivationKeys() {
		full[key] = values[key]
	}
	gaps := domain.CKPNPABLEnablementGaps(ctx, full, main)
	return &domain.CKPNPABLActivation{
		Values:          full,
		EnablementReady: len(gaps) == 0,
		EnablementGaps:  gaps,
	}
}

// ckpnPABLAuditChanges membangun catatan audit nilai sebelum -> sesudah per kunci.
func ckpnPABLAuditChanges(before, changes map[string]string) map[string]any {
	out := make(map[string]any, len(changes))
	for key, after := range changes {
		out[key] = map[string]any{"before": before[key], "after": after}
	}
	return out
}
