package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

// Sisa pembulatan harus terserap ke rekening terakhir: jumlah alokasi seluruh rekening
// sama persis dengan RoundToRupiah(laba yang didistribusikan menurut nisbah).
func TestMudharabahAllocate_SisaTerserapRekeningTerakhir(t *testing.T) {
	// Kasus yang dulu menyimpang: pool=1.000.000, nisbah=0.5, tiga saldo sama (3,3,3).
	pool := decimal.NewFromInt(1_000_000)
	nisbah := decimal.NewFromFloat(0.5)
	inputs := []MudharabahAllocationInput{
		{Nisbah: nisbah, AverageBalance: decimal.NewFromInt(3)},
		{Nisbah: nisbah, AverageBalance: decimal.NewFromInt(3)},
		{Nisbah: nisbah, AverageBalance: decimal.NewFromInt(3)},
	}
	total := decimal.NewFromInt(9)

	got := MudharabahAllocate(pool, total, inputs)

	target := RoundToRupiah(pool.Mul(nisbah)) // 500.000
	sum := decimal.Zero
	for _, v := range got {
		sum = sum.Add(v)
	}
	if !sum.Equal(target) {
		t.Fatalf("jumlah alokasi %s != target %s (inputs=%v)", sum, target, got)
	}
	// Rekening terakhir menyerap sisa; dua pertama dibulatkan biasa.
	if !got[0].Equal(got[1]) {
		t.Fatalf("dua rekening pertama harus sama, dapat %s dan %s", got[0], got[1])
	}
}

// Nisbah berbeda per rekening tetap menghormati proporsi dan target total.
func TestMudharabahAllocate_NisbahBerbeda(t *testing.T) {
	pool := decimal.NewFromInt(2_000_000)
	inputs := []MudharabahAllocationInput{
		{Nisbah: decimal.NewFromFloat(0.6), AverageBalance: decimal.NewFromInt(1_000)},
		{Nisbah: decimal.NewFromFloat(0.4), AverageBalance: decimal.NewFromInt(1_000)},
		{Nisbah: decimal.NewFromFloat(0.5), AverageBalance: decimal.NewFromInt(500)},
	}
	total := decimal.NewFromInt(2_500)

	got := MudharabahAllocate(pool, total, inputs)

	target := decimal.Zero
	sum := decimal.Zero
	for i, in := range inputs {
		target = target.Add(in.Nisbah.Mul(in.AverageBalance))
		sum = sum.Add(got[i])
	}
	target = RoundToRupiah(pool.Mul(target).Div(total))

	if !sum.Equal(target) {
		t.Fatalf("jumlah alokasi %s != target %s", sum, target)
	}
	// Rekening 1 (porsi lebih besar) harus menerima lebih banyak dari rekening 3.
	if got[0].LessThanOrEqual(got[2]) {
		t.Fatalf("porsi 0.6x1000 harus lebih besar dari 0.5x500: %s vs %s", got[0], got[2])
	}
}

// Input kosong atau total nol mengembalikan nol sepanjang input (bukan panic).
func TestMudharabahAllocate_KosongAman(t *testing.T) {
	if got := MudharabahAllocate(decimal.NewFromInt(1), decimal.Zero, []MudharabahAllocationInput{{Nisbah: decimal.NewFromInt(1), AverageBalance: decimal.NewFromInt(1)}}); !got[0].IsZero() {
		t.Fatalf("total nol harus menghasilkan nol, dapat %s", got[0])
	}
	if got := MudharabahAllocate(decimal.NewFromInt(1), decimal.NewFromInt(1), nil); len(got) != 0 {
		t.Fatalf("input nil harus menghasilkan slice kosong, dapat %v", got)
	}
}

// Rekening tanpa porsi (nisbah nol) tidak menerima apa pun walau berada paling akhir.
func TestMudharabahAllocate_PenerimaSisaLewatiPorsiNol(t *testing.T) {
	pool := decimal.NewFromInt(1_000_000)
	inputs := []MudharabahAllocationInput{
		{Nisbah: decimal.NewFromFloat(0.5), AverageBalance: decimal.NewFromInt(3)},
		{Nisbah: decimal.Zero, AverageBalance: decimal.NewFromInt(0)},
	}
	total := decimal.NewFromInt(3)

	got := MudharabahAllocate(pool, total, inputs)
	if !got[1].IsZero() {
		t.Fatalf("rekening tanpa porsi harus nol, dapat %s", got[1])
	}
	if !got[0].Equal(RoundToRupiah(pool.Mul(decimal.NewFromFloat(0.5)))) {
		t.Fatalf("rekening 0 harus menerima target penuh, dapat %s", got[0])
	}
}
