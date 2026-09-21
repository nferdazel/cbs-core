package postgres

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// Penempatan deposito berjangka harus punya prefix referensi sendiri (DPL) supaya
// nomornya tidak tertukar dengan setoran tunai teller (DEP) di daftar referensi dan
// rekonsiliasi. Prefix ditentukan dari jenis transaksi jurnal.
func TestRefPrefixPenempatanDepositoTerpisahDariSetoran(t *testing.T) {
	if got := refPrefix[domain.TxTypeDepositPlacement]; got != "DPL" {
		t.Fatalf("prefix penempatan deposito = %q, mau DPL", got)
	}
	if got := refPrefix[domain.TxTypeDeposit]; got != "DEP" {
		t.Fatalf("prefix setoran tunai = %q, mau DEP", got)
	}
	if refPrefix[domain.TxTypeDepositPlacement] == refPrefix[domain.TxTypeDeposit] {
		t.Fatal("prefix penempatan deposito tidak boleh sama dengan setoran tunai")
	}
}
