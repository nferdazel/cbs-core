package domain

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func cronTime(y int, m time.Month, d, hh, mm int) time.Time {
	return time.Date(y, m, d, hh, mm, 0, 0, BankZone)
}

// Evaluator cron harus menghitung kemunculan berikutnya dengan benar untuk bentuk
// yang benar-benar dipakai bank, dalam zona waktu bank (WIB).
func TestCronNext(t *testing.T) {
	cases := []struct {
		expr  string
		after time.Time
		want  time.Time
	}{
		{"0 23 * * *", cronTime(2026, time.September, 22, 22, 30), cronTime(2026, time.September, 22, 23, 0)},
		// Tepat pada menit jadwal: berikutnya harus HARI BERIKUTNYA, bukan menit itu juga.
		{"0 23 * * *", cronTime(2026, time.September, 22, 23, 0), cronTime(2026, time.September, 23, 23, 0)},
		{"*/15 * * * *", cronTime(2026, time.September, 22, 10, 7), cronTime(2026, time.September, 22, 10, 15)},
		{"0 0 1 1 *", cronTime(2026, time.September, 22, 0, 0), cronTime(2027, time.January, 1, 0, 0)},
		// Jumat 18:00 -> Senin 09:00 (akhir pekan dilewati).
		{"0 9-17 * * 1-5", cronTime(2026, time.September, 25, 18, 0), cronTime(2026, time.September, 28, 9, 0)},
		// 7 = Minggu (Selasa -> Minggu berikutnya).
		{"0 0 * * 7", cronTime(2026, time.September, 22, 0, 0), cronTime(2026, time.September, 27, 0, 0)},
		// Tanggal dan hari keduanya dibatasi: yang lebih dulu menang (Jumat 25 Sep).
		{"0 0 13 * 5", cronTime(2026, time.September, 22, 0, 0), cronTime(2026, time.September, 25, 0, 0)},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			sched, err := ParseCron(tc.expr)
			if err != nil {
				t.Fatalf("ParseCron(%q): %v", tc.expr, err)
			}
			got, err := sched.Next(tc.after)
			if err != nil {
				t.Fatalf("Next: %v", err)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("Next(%s) = %s, mau %s", tc.after.Format(time.RFC3339), got.Format(time.RFC3339), tc.want.Format(time.RFC3339))
			}
		})
	}
}

// Ekspresi tidak sah WAJIB ditolak dengan galat yang dapat dikenali. Diam-diam
// menganggapnya "tidak pernah jatuh tempo" dilarang.
func TestCronRejectsInvalidExpressions(t *testing.T) {
	invalid := []string{
		"",
		"* * * *",
		"* * * * * *",
		"60 * * * *",
		"* 24 * * *",
		"* * 0 * *",
		"* * 32 * *",
		"* * * 13 *",
		"* * * * 8",
		"a * * * *",
		"*/0 * * * *",
		"1/2 * * * *",
		"5-1 * * * *",
		"1,,2 * * * *",
		"1-2-3 * * * *",
	}
	for _, expr := range invalid {
		t.Run(fmt.Sprintf("%q", expr), func(t *testing.T) {
			if _, err := ParseCron(expr); !errors.Is(err, ErrInvalidCron) {
				t.Fatalf("ParseCron(%q) harus menolak dengan ErrInvalidCron, dapat %v", expr, err)
			}
		})
	}
}

// Matches mengikuti aturan tanggal/hari cron standar.
func TestCronMatches(t *testing.T) {
	sched, err := ParseCron("30 8 13 * 5")
	if err != nil {
		t.Fatalf("ParseCron: %v", err)
	}
	// Jumat 25 Sep 2026 08:30 -> cocok lewat hari (Jumat), tanggal bukan 13.
	if !sched.Matches(cronTime(2026, time.September, 25, 8, 30)) {
		t.Fatal("Jumat 08:30 harus cocok lewat aturan hari")
	}
	// Selasa 13 Okt 2026 08:30 -> cocok lewat tanggal.
	if !sched.Matches(cronTime(2026, time.October, 13, 8, 30)) {
		t.Fatal("tanggal 13 08:30 harus cocok lewat aturan tanggal")
	}
	// Menit salah.
	if sched.Matches(cronTime(2026, time.September, 25, 8, 31)) {
		t.Fatal("menit 31 tidak boleh cocok")
	}
}
