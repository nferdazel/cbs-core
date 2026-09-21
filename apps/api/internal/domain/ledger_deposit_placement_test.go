package domain

import "testing"

// Penempatan deposito berjangka tidak boleh tertukar dengan setoran tunai teller di
// laporan. Jenis transaksinya harus bernilai tersendiri dan berbeda dari DEPOSIT.
//
// CATATAN: penggantian tipe pada jalur penempatan (deposit_service.go) berada di luar
// perubahan ini karena berkas itu sedang dikerjakan paralel; uji ini menjaga kontrak
// nilainya tetap terpisah.
func TestTxTypeDepositPlacementBerbedaDariSetoranTunai(t *testing.T) {
	if TxTypeDepositPlacement == TxTypeDeposit {
		t.Fatalf("DEPOSIT_PLACEMENT tidak boleh sama dengan DEPOSIT (%q)", TxTypeDeposit)
	}
	if string(TxTypeDepositPlacement) != "DEPOSIT_PLACEMENT" {
		t.Fatalf("nilai jenis transaksi %q, ingin DEPOSIT_PLACEMENT", TxTypeDepositPlacement)
	}
}
