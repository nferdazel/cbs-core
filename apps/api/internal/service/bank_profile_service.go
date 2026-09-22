package service

import (
	"context"
	"database/sql"

	"cbs-core/apps/core-api/internal/domain"
)

type bankProfileService struct {
	repo      domain.BankProfileStore
	auditRepo domain.AuditRepository
	// txRunner membungkus perubahan profil dan auditnya dalam SATU transaksi.
	// Lewat interface agar dapat diganti stub pada uji unit tanpa database.
	txRunner ppapTxRunner
}

// NewBankProfileService merakit layanan profil bank. auditRepo variadik mengikuti
// pola layanan lain: pemanggil lama tanpa audit tetap dapat memakai konstruktor
// ini, dan auditRepo nil berarti penulisan audit dilewati tanpa menggagalkan aksi.
func NewBankProfileService(db *sql.DB, repo domain.BankProfileStore, auditSinks ...domain.AuditRepository) domain.BankProfileService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &bankProfileService{repo: repo, auditRepo: auditRepo, txRunner: sqlPPAPTxRunner{db: db}}
}

// Get mengembalikan profil bank. Baris yang belum ada dinormalkan menjadi profil
// kosong (bukan nil) supaya pemanggil web tidak perlu menangani dua bentuk.
func (s *bankProfileService) Get(ctx context.Context) (*domain.BankProfile, error) {
	profile, err := s.repo.Get(ctx)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return &domain.BankProfile{}, nil
	}
	return profile, nil
}

// Update mengubah identitas bank. Hanya bidang yang dikirim yang diubah; hasil
// akhir divalidasi (nama wajib, panjang wajar) sebelum disimpan. Setiap perubahan
// mencatat nilai sebelum -> sesudah dalam transaksi yang sama dengan penulisannya,
// sehingga audit tidak dapat mencatat perubahan yang batal disimpan.
func (s *bankProfileService) Update(ctx context.Context, input domain.UpdateBankProfileInput, actor domain.Actor) (*domain.BankProfile, error) {
	if input.IsEmpty() {
		return nil, domain.ErrBankProfileEmpty
	}

	var updated *domain.BankProfile
	err := s.txRunner.Run(ctx, func(tx any) error {
		current, err := s.repo.GetForUpdate(ctx, tx)
		if err != nil {
			return err
		}
		before := domain.BankProfile{}
		if current != nil {
			before = *current
		}

		// Bekerja pada salinan: kegagalan validasi tidak boleh meninggalkan objek
		// yang dikembalikan repositori dalam keadaan separuh berubah.
		candidate := before
		applyBankProfileUpdate(&candidate, input)
		if err := domain.ValidateBankProfile(&candidate); err != nil {
			return err
		}

		changes := bankProfileChanges(before, candidate)
		if len(changes) == 0 {
			// Tidak ada yang benar-benar berubah: PUT idempoten tetap sukses tanpa
			// menulis baris maupun audit yang bisa disalahbaca sebagai perubahan.
			updated = &candidate
			return nil
		}

		if err := s.repo.UpdateTx(ctx, tx, &candidate); err != nil {
			return err
		}
		if err := writeAudit(ctx, s.auditRepo, tx, actor, "UPDATE_BANK_PROFILE", "bank_profile", "1", changes); err != nil {
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

// applyBankProfileUpdate menyalin bidang yang dikirim ke profil kandidat.
func applyBankProfileUpdate(p *domain.BankProfile, in domain.UpdateBankProfileInput) {
	if in.Name != nil {
		p.Name = *in.Name
	}
	if in.Address != nil {
		p.Address = *in.Address
	}
	if in.City != nil {
		p.City = *in.City
	}
	if in.Phone != nil {
		p.Phone = *in.Phone
	}
	if in.NPWP != nil {
		p.NPWP = *in.NPWP
	}
}

// bankProfileChanges membangun catatan audit nilai sebelum -> sesudah untuk setiap
// bidang identitas yang benar-benar berubah.
func bankProfileChanges(before, after domain.BankProfile) map[string]any {
	changes := map[string]any{}
	add := func(field, b, a string) {
		if b != a {
			changes[field] = map[string]any{"before": b, "after": a}
		}
	}
	add("name", before.Name, after.Name)
	add("address", before.Address, after.Address)
	add("city", before.City, after.City)
	add("phone", before.Phone, after.Phone)
	add("npwp", before.NPWP, after.NPWP)
	return changes
}
