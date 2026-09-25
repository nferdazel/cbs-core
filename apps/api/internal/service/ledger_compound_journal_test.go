package service

import (
	"context"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

func compoundLine(account, direction string, amount int64) domain.CustomJournalLineInput {
	return domain.CustomJournalLineInput{
		AccountNumber: account,
		Direction:     domain.EntryDirection(direction),
		Amount:        decimal.NewFromInt(amount),
	}
}

// Jurnal majemuk memerlukan minimal dua baris; satu baris tidak dapat seimbang.
func TestPostCompoundJournalServiceTolakBarisKurangDua(t *testing.T) {
	posting := &stubPostingSvc{}
	svc := &ledgerService{posting: posting}

	_, err := svc.PostCompoundJournal(context.Background(), domain.CustomJournalRequest{
		Lines: []domain.CustomJournalLineInput{compoundLine("10101", "DEBIT", 1000)},
	})
	if err == nil {
		t.Fatal("satu baris harus ditolak")
	}
	if posting.calls != 0 {
		t.Fatal("jurnal tidak boleh diposting saat baris tidak valid")
	}
}

// Nominal nol/negatif ditolak sebelum posting engine menyentuh saldo.
func TestPostCompoundJournalServiceTolakNominalTidakPositif(t *testing.T) {
	posting := &stubPostingSvc{}
	svc := &ledgerService{posting: posting}

	_, err := svc.PostCompoundJournal(context.Background(), domain.CustomJournalRequest{
		Lines: []domain.CustomJournalLineInput{
			compoundLine("10101", "DEBIT", 0),
			compoundLine("40100", "CREDIT", 0),
		},
	})
	if err == nil {
		t.Fatal("nominal nol harus ditolak")
	}
	if posting.calls != 0 {
		t.Fatal("jurnal tidak boleh diposting saat nominal tidak positif")
	}
}

// Jurnal yang sah diteruskan apa adanya ke posting engine.
func TestPostCompoundJournalServiceMeneruskanKePosting(t *testing.T) {
	posting := &stubPostingSvc{}
	svc := &ledgerService{posting: posting}

	entry, err := svc.PostCompoundJournal(context.Background(), domain.CustomJournalRequest{
		TransactionType: domain.TxTypeAdjustment,
		Description:     "koreksi manual",
		IdempotencyKey:  "IDEM-1",
		CreatedBy:       "admin01",
		Lines: []domain.CustomJournalLineInput{
			compoundLine("10101", "DEBIT", 1000),
			compoundLine("40100", "CREDIT", 1000),
		},
	})
	if err != nil {
		t.Fatalf("PostCompoundJournal: %v", err)
	}
	if entry == nil || posting.calls != 1 {
		t.Fatalf("posting dipanggil %d kali, ingin 1", posting.calls)
	}
	if posting.last.CreatedBy != "admin01" || posting.last.IdempotencyKey != "IDEM-1" {
		t.Fatalf("permintaan tidak diteruskan apa adanya: %+v", posting.last)
	}
	if len(posting.last.Lines) != 2 {
		t.Fatalf("baris diteruskan = %d, ingin 2", len(posting.last.Lines))
	}
}
