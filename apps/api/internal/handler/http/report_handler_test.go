package http

import (
	"testing"
	"time"
)

// TestBusinessDayPakaiZonaBank membuktikan default tanggal laporan mengikuti
// TANGGAL BISNIS bank (WIB), bukan UTC. Pada 23:38 UTC tanggal 22 Sep, WIB sudah
// 23 Sep dini hari; laporan harus menyebut 23 Sep agar tidak tampak kosong.
// Dengan perhitungan UTC lama (reportToday memakai time.Now().UTC()), nilai yang
// diharapkan di sini adalah 22 Sep sehingga uji ini gagal — bukti uji bisa gagal.
func TestBusinessDayPakaiZonaBank(t *testing.T) {
	now := time.Date(2026, time.September, 22, 23, 38, 0, 0, time.UTC)

	today, firstOfMonth := businessDay(now)

	if want := time.Date(2026, time.September, 23, 0, 0, 0, 0, time.UTC); !today.Equal(want) {
		t.Fatalf("today = %s, ingin %s (tanggal WIB)", today, want)
	}
	if want := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC); !firstOfMonth.Equal(want) {
		t.Fatalf("firstOfMonth = %s, ingin %s", firstOfMonth, want)
	}
}
