package service

import (
	"context"
	"database/sql"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// ckpnActivationService mengelola pengaturan aktivasi CKPN (parameter PD/LGD, akun
// syariah, status SEMENTARA/FINAL, bukti ratifikasi, mode bayangan, saklar ckpn.enabled).
// Perubahan dan auditnya berada dalam SATU transaksi, seperti profil bank/OJK, sehingga
// audit tidak mencatat perubahan yang batal disimpan.
type ckpnActivationService struct {
	repo   domain.CKPNActivationStore
	config domain.SystemConfigService
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewCKPNActivationService merakit layanan pengaturan aktivasi CKPN. config dipakai
// membuang cache kunci yang berubah agar nilainya langsung berlaku pada proses yang
// sedang berjalan; boleh nil pada uji tanpa konfigurasi. auditRepo variadik mengikuti
// pola layanan lain: nil berarti audit dilewati tanpa menggagalkan aksi.
func NewCKPNActivationService(db *sql.DB, repo domain.CKPNActivationStore,
	config domain.SystemConfigService, auditSinks ...domain.AuditRepository) domain.CKPNActivationService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &ckpnActivationService{repo: repo, config: config, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// Get mengembalikan nilai terkini beserta status parameter dan penahan penyalakan.
// Seluruhnya baca-saja: tidak menulis konfigurasi dan tidak menjalankan EOD.
func (s *ckpnActivationService) Get(ctx context.Context) (*domain.CKPNActivation, error) {
	values, err := s.repo.Get(ctx)
	if err != nil {
		return nil, err
	}
	return buildCKPNActivation(ctx, values, time.Now()), nil
}

// Update memvalidasi hasil akhir, menyimpan perubahan, dan menulis audit dalam satu
// transaksi. Bidang yang tidak dikirim tidak diubah; bidang yang nilainya sama tidak
// menghasilkan baris/audit (PUT idempoten).
func (s *ckpnActivationService) Update(ctx context.Context, input domain.UpdateCKPNActivationInput, actor domain.Actor) (*domain.CKPNActivation, error) {
	if input.IsEmpty() {
		return nil, domain.ErrCKPNActivationEmpty
	}

	now := time.Now()
	var (
		result      *domain.CKPNActivation
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
			result = buildCKPNActivation(ctx, current, now)
			return nil
		}

		merged := make(map[string]string, len(current)+len(changes))
		for _, key := range domain.CKPNActivationKeys() {
			merged[key] = current[key]
		}
		for key, value := range changes {
			merged[key] = value
		}
		if err := domain.ValidateCKPNActivation(merged, now); err != nil {
			return err
		}

		if err := s.repo.SaveTx(ctx, tx, changes, actor.UserID); err != nil {
			return err
		}
		if err := writeAudit(ctx, s.audit, tx, actor, "UPDATE_CKPN_ACTIVATION", "ckpn_activation", "1",
			ckpnActivationAuditChanges(current, changes)); err != nil {
			return err
		}
		result = buildCKPNActivation(ctx, merged, now)
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

// buildCKPNActivation membentuk snapshot baca-saja dari sekumpulan nilai. Status dihitung
// lewat fungsi domain yang sama dengan start/EOD sehingga angka yang dilihat pengguna
// tidak pernah berbeda dari yang dipakai mesin. Penahan penyalakan dinilai atas NILAI
// TERSIMPAN tanpa memandang saklar, agar pengguna melihat sisa pekerjaan walau CKPN mati.
func buildCKPNActivation(ctx context.Context, values map[string]string, now time.Time) *domain.CKPNActivation {
	full := make(map[string]string, len(values))
	for _, key := range domain.CKPNActivationKeys() {
		full[key] = values[key]
	}
	snapshot := domain.NewConfigSnapshot(full)
	status := domain.CKPNParametersStatusFromConfig(ctx, snapshot, now)
	gaps := domain.CKPNEnablementGapsForCandidate(ctx, snapshot)
	return &domain.CKPNActivation{
		Values:          full,
		Status:          status,
		EnablementReady: len(gaps) == 0,
		EnablementGaps:  gaps,
	}
}

// ckpnActivationAuditChanges membangun catatan audit nilai sebelum -> sesudah per kunci.
func ckpnActivationAuditChanges(before, changes map[string]string) map[string]any {
	out := make(map[string]any, len(changes))
	for key, after := range changes {
		out[key] = map[string]any{"before": before[key], "after": after}
	}
	return out
}
