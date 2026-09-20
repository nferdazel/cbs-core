package domain

import "github.com/shopspring/decimal"

// PaymentAllocation adalah pembagian satu pembayaran kredit ke denda, bunga/margin,
// dan pokok, beserta sisa yang tidak dapat dialokasikan.
type PaymentAllocation struct {
	Penalty   decimal.Decimal
	Profit    decimal.Decimal
	Principal decimal.Decimal
	Unapplied decimal.Decimal
}

// Applied adalah jumlah yang benar-benar terserap ke kewajiban.
func (a PaymentAllocation) Applied() decimal.Decimal {
	return a.Penalty.Add(a.Profit).Add(a.Principal)
}

// AllocateLoanPayment membagi pembayaran dengan urutan denda, lalu bunga/margin,
// terakhir pokok.
//
// Urutan ini bukan pilihan gaya: denda adalah kewajiban yang jatuh tempo lebih dulu
// dan paling berisiko tidak tertagih, sedangkan bunga yang sudah diakru sudah menjadi
// piutang dan pokok menentukan penghentian akrual. Membayar pokok lebih dulu akan
// membuat kredit tampak sehat sementara dendanya menumpuk.
//
// amount adalah uang yang diterima; penaltyDue, profitDue, dan principalDue adalah
// kewajiban yang belum dibayar. Sisa setelah seluruh kewajiban ditutup dikembalikan
// di Unapplied supaya pemanggil tidak diam-diam menelan kelebihan pembayaran.
func AllocateLoanPayment(amount, penaltyDue, profitDue, principalDue decimal.Decimal) PaymentAllocation {
	remaining := amount
	take := func(due decimal.Decimal) decimal.Decimal {
		if !due.IsPositive() {
			return decimal.Zero
		}
		if remaining.LessThan(due) {
			taken := remaining
			remaining = decimal.Zero
			return taken
		}
		remaining = remaining.Sub(due)
		return due
	}

	allocation := PaymentAllocation{}
	allocation.Penalty = take(penaltyDue)
	allocation.Profit = take(profitDue)
	allocation.Principal = take(principalDue)

	if remaining.IsPositive() {
		allocation.Unapplied = remaining
	}
	return allocation
}
