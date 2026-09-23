package service

import (
	"context"
	"database/sql"

	"cbs-core/apps/core-api/internal/domain"
)

// ojkProfileService mengelola identitas Form 00.00 yang disimpan sebagai kunci
// system_config ojk.*. Perubahan dan auditnya berada dalam SATU transaksi, seperti
// profil bank, sehingga audit tidak mencatat perubahan yang batal disimpan.
type ojkProfileService struct {
	repo      domain.OJKProfileStore
	auditRepo domain.AuditRepository
	txRunner  ppapTxRunner
}

// NewOJKProfileService merakit layanan profil OJK. auditRepo variadik mengikuti pola
// layanan lain: pemanggil lama tanpa audit tetap dapat memakai konstruktor ini, dan
// auditRepo nil berarti penulisan audit dilewati tanpa menggagalkan aksi.
func NewOJKProfileService(db *sql.DB, repo domain.OJKProfileStore, auditSinks ...domain.AuditRepository) domain.OJKProfileService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &ojkProfileService{repo: repo, auditRepo: auditRepo, txRunner: sqlPPAPTxRunner{db: db}}
}

// Get mengembalikan profil OJK. Repositori selalu memberi objek non-nil; pengaman
// nil tetap ada agar pemanggil web tidak menangani dua bentuk.
func (s *ojkProfileService) Get(ctx context.Context) (*domain.OJKProfile, error) {
	profile, err := s.repo.Get(ctx)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return &domain.OJKProfile{}, nil
	}
	return profile, nil
}

// Update mengubah identitas OJK. Hanya bidang yang dikirim yang diubah; hasil akhir
// divalidasi sebelum disimpan. Setiap perubahan mencatat nilai sebelum -> sesudah
// dalam transaksi yang sama dengan penulisannya.
func (s *ojkProfileService) Update(ctx context.Context, input domain.UpdateOJKProfileInput, actor domain.Actor) (*domain.OJKProfile, error) {
	if input.IsEmpty() {
		return nil, domain.ErrOJKProfileEmpty
	}

	var updated *domain.OJKProfile
	err := s.txRunner.Run(ctx, func(tx any) error {
		current, err := s.repo.GetTx(ctx, tx)
		if err != nil {
			return err
		}
		if current == nil {
			current = &domain.OJKProfile{}
		}
		before := *current

		// Bekerja pada salinan: kegagalan validasi tidak boleh meninggalkan objek
		// yang dikembalikan repositori dalam keadaan separuh berubah.
		candidate := before
		input.Apply(&candidate)
		if err := domain.ValidateOJKProfile(&candidate); err != nil {
			return err
		}

		changes := domain.OJKProfileChanges(&before, &candidate)
		if len(changes) == 0 {
			// Tidak ada yang benar-benar berubah: PUT idempoten tetap sukses tanpa
			// menulis baris maupun audit yang bisa disalahbaca sebagai perubahan.
			updated = &candidate
			return nil
		}

		if err := s.repo.SaveTx(ctx, tx, &candidate, actor.UserID); err != nil {
			return err
		}
		if err := writeAudit(ctx, s.auditRepo, tx, actor, "UPDATE_OJK_PROFILE", "ojk_profile", "1", changes); err != nil {
			return err
		}
		updated = &candidate
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}
