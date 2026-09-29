package postgres

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// TestFirstDuplicatePlacementMenemukanDuplikat memastikan guard invarian penempatan LPS
// menandai penempatan logis yang sama muncul dua kali, sesuai invarian
// docs/CKPN-SIAP-RILIS.md (satu baris per penempatan, as_of diperbarui bukan diduplikasi).
func TestFirstDuplicatePlacementMenemukanDuplikat(t *testing.T) {
	list := []domain.LPSPlacement{
		{COACode: "10200", CounterpartyBank: "BANK A", PlacementType: domain.LPSPlacementDeposito},
		{COACode: "11200", CounterpartyBank: "BANK B", PlacementType: domain.LPSPlacementGiro},
		{COACode: "10200", CounterpartyBank: "BANK A", PlacementType: domain.LPSPlacementDeposito},
	}
	dup, ok := firstDuplicatePlacement(list)
	if !ok {
		t.Fatal("duplikat penempatan logis harus terdeteksi")
	}
	if dup.coaCode != "10200" || dup.counterpartyBank != "BANK A" || dup.placementType != "DEPOSITO" {
		t.Fatalf("kunci duplikat salah: %+v", dup)
	}
}

// TestFirstDuplicatePlacementTidakMenandaiPenempatanBerbeda memastikan dua penempatan
// berbeda di bank yang sama (mis. dua deposito berjangka) TIDAK dianggap duplikat. Inilah
// alasan dedup berbasis kunci logis tidak dipakai untuk menggabungkan baris: penempatan
// berbeda di bank sama adalah dua baris sah.
func TestFirstDuplicatePlacementTidakMenandaiPenempatanBerbeda(t *testing.T) {
	list := []domain.LPSPlacement{
		{COACode: "10200", CounterpartyBank: "BANK A", PlacementType: domain.LPSPlacementDeposito},
		{COACode: "10200", CounterpartyBank: "BANK A", PlacementType: domain.LPSPlacementGiro},
		{COACode: "10200", CounterpartyBank: "BANK B", PlacementType: domain.LPSPlacementDeposito},
	}
	if dup, ok := firstDuplicatePlacement(list); ok {
		t.Fatalf("penempatan berbeda tidak boleh dianggap duplikat: %+v", dup)
	}
}

// TestFirstDuplicatePlacementDaftarKosong memastikan daftar kosong aman (tabel produksi
// saat ini kosong).
func TestFirstDuplicatePlacementDaftarKosong(t *testing.T) {
	if _, ok := firstDuplicatePlacement(nil); ok {
		t.Fatal("daftar kosong tidak boleh menandai duplikat")
	}
}
