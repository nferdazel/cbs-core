package service

import (
	"context"

	"cbs-core/apps/core-api/internal/domain"
)

// CheckRecoveryCapAtSubmission memvalidasi batas akumulasi pemulihan pada SAAT
// PENGAJUAN, sebelum permintaan masuk antrean maker-checker, sehingga pengajuan yang
// pasti ditolak tidak mengisi antrean (temuan E2E N3). Pemeriksaan tetap dijalankan
// ulang saat persetujuan lewat guardRecoveryCap di recoveryTx karena keadaan dapat
// berubah di antara pengajuan dan keputusan.
//
// Metode ini berada di berkas terpisah namun memakai jalur transaksi (txRunner) dan
// kunci idempoten (recoveryKey) yang sudah ada, sehingga tidak ada logika batas kedua.
// Pemeriksaan akses memakai GetLoan yang sama dengan jalur eksekusi: kredit cabang/buku
// lain tetap ditolak lebih dulu, bukan nilai batasnya yang terlihat.
func (s *loanService) CheckRecoveryCapAtSubmission(ctx context.Context, input domain.RecoverWrittenOffLoanInput, actor domain.Actor) error {
	loan, err := s.GetLoan(ctx, input.LoanID, actor)
	if err != nil {
		return err
	}
	return s.txRunner.Run(ctx, func(tx any) error {
		idempotencyKey := s.recoveryKey(ctx, loan.LoanNumber, input.RecoveryAmount, input.IdempotencyKey)
		return s.guardRecoveryCap(ctx, tx, loan, input.RecoveryAmount, idempotencyKey)
	})
}
