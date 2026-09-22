package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// BankZone adalah zona waktu bank (WIB, UTC+7) tanpa ketergantungan tzdata sistem.
// Indonesia tidak memakai daylight saving sehingga offset tetap aman. Ekspresi cron
// pemicu EOD dievaluasi di zona ini supaya jadwal "23:00" berarti 23:00 waktu bank,
// bukan 23:00 UTC.
var BankZone = time.FixedZone("WIB", 7*60*60)

// ErrInvalidCron menandai ekspresi jadwal yang tidak dapat ditafsirkan. Pemanggil
// WAJIB memperlakukannya sebagai galat yang terlihat, bukan sebagai "tidak pernah
// jatuh tempo": ekspresi yang salah ketik tidak boleh diam-diam mematikan EOD.
var ErrInvalidCron = errors.New("ekspresi cron tidak valid")

// CronSchedule adalah jadwal cron lima bagian (menit jam tanggal bulan hari) yang
// sudah divalidasi. Subset yang didukung sengaja dibatasi pada bentuk yang benar-benar
// dipakai bank: '*', angka tunggal, rentang 'a-b', daftar 'a,b', dan langkah '*/n'
// atau 'a-b/n'. Nama (JAN/MON) dan detik tidak didukung agar tidak ada ekspresi yang
// "tampak sah" tetapi ditafsirkan berbeda dari cron sistem operasi.
type CronSchedule struct {
	minute cronField
	hour   cronField
	dom    cronField
	month  cronField
	dow    cronField
}

// cronField adalah satu bagian cron yang sudah diurai.
type cronField struct {
	allowed map[int]bool
	// wildcard hanya TRUE untuk '*' persis. Dipakai aturan standar
	// tanggal-vs-hari: bila keduanya dibatasi, salah satu cocok sudah cukup.
	wildcard bool
}

// ParseCron memvalidasi dan mengurai ekspresi cron lima bagian. Kesalahan menyebut
// bagian mana yang tidak sah agar operator dapat memperbaikinya tanpa membaca kode.
func ParseCron(expr string) (CronSchedule, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return CronSchedule{}, fmt.Errorf("%w: butuh 5 bagian (menit jam tanggal bulan hari), ekspresi %q punya %d",
			ErrInvalidCron, expr, len(fields))
	}

	var s CronSchedule
	var err error
	if s.minute, err = parseCronField(fields[0], 0, 59); err != nil {
		return CronSchedule{}, fmt.Errorf("%w pada bagian menit %q: %v", ErrInvalidCron, fields[0], err)
	}
	if s.hour, err = parseCronField(fields[1], 0, 23); err != nil {
		return CronSchedule{}, fmt.Errorf("%w pada bagian jam %q: %v", ErrInvalidCron, fields[1], err)
	}
	if s.dom, err = parseCronField(fields[2], 1, 31); err != nil {
		return CronSchedule{}, fmt.Errorf("%w pada bagian tanggal %q: %v", ErrInvalidCron, fields[2], err)
	}
	if s.month, err = parseCronField(fields[3], 1, 12); err != nil {
		return CronSchedule{}, fmt.Errorf("%w pada bagian bulan %q: %v", ErrInvalidCron, fields[3], err)
	}
	if s.dow, err = parseCronField(fields[4], 0, 7); err != nil {
		return CronSchedule{}, fmt.Errorf("%w pada bagian hari %q: %v", ErrInvalidCron, fields[4], err)
	}
	// 7 = Minggu, sama dengan 0 (kebiasaan cron standar).
	if s.dow.allowed[7] {
		s.dow.allowed[0] = true
	}
	return s, nil
}

// parseCronField mengurai satu bagian menjadi himpunan nilai yang diizinkan.
func parseCronField(spec string, min, max int) (cronField, error) {
	f := cronField{allowed: map[int]bool{}}
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return f, errors.New("bagian kosong")
	}
	if spec == "*" {
		f.wildcard = true
		for v := min; v <= max; v++ {
			f.allowed[v] = true
		}
		return f, nil
	}

	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return f, errors.New("ada bagian kosong di dalam daftar")
		}

		step := 1
		base := part
		if i := strings.IndexByte(part, '/'); i >= 0 {
			base = part[:i]
			raw := part[i+1:]
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 {
				return f, fmt.Errorf("langkah %q tidak sah (harus bilangan >= 1)", raw)
			}
			step = n
		}

		var lo, hi int
		switch {
		case base == "*":
			lo, hi = min, max
		case strings.Contains(base, "-"):
			a, b, _ := strings.Cut(base, "-")
			if strings.Contains(b, "-") {
				return f, fmt.Errorf("rentang %q tidak sah", base)
			}
			av, err := strconv.Atoi(a)
			if err != nil {
				return f, fmt.Errorf("angka %q tidak sah", a)
			}
			bv, err := strconv.Atoi(b)
			if err != nil {
				return f, fmt.Errorf("angka %q tidak sah", b)
			}
			lo, hi = av, bv
		default:
			if step != 1 {
				// Cron standar hanya mengizinkan langkah setelah '*' atau rentang;
				// 'a/n' ditolak agar tidak ditafsirkan berbeda.
				return f, fmt.Errorf("langkah hanya boleh mengikuti '*' atau rentang, bukan %q", part)
			}
			v, err := strconv.Atoi(base)
			if err != nil {
				return f, fmt.Errorf("angka %q tidak sah", base)
			}
			lo, hi = v, v
		}

		if lo < min || hi > max || lo > hi {
			return f, fmt.Errorf("nilai %q di luar rentang %d-%d", part, min, max)
		}
		for v := lo; v <= hi; v += step {
			f.allowed[v] = true
		}
	}
	return f, nil
}

// Matches melaporkan apakah waktu t cocok dengan jadwal. Aturan tanggal/hari mengikuti
// cron standar: bila tanggal dan hari keduanya dibatasi (bukan '*'), kecocokan salah
// satu sudah cukup.
func (s CronSchedule) Matches(t time.Time) bool {
	t = t.In(BankZone)
	if !s.minute.allowed[t.Minute()] || !s.hour.allowed[t.Hour()] ||
		!s.month.allowed[int(t.Month())] {
		return false
	}

	domMatch := s.dom.allowed[t.Day()]
	dowMatch := s.dow.allowed[int(t.Weekday())]
	switch {
	case s.dom.wildcard && s.dow.wildcard:
		return true
	case s.dom.wildcard:
		return dowMatch
	case s.dow.wildcard:
		return domMatch
	default:
		return domMatch || dowMatch
	}
}

// Next mengembalikan kemunculan pertama yang cocok SETELAH after, dalam BankZone.
// Pencarian menit demi menit dibatasi 10 tahun; ekspresi yang lolos ParseCron selalu
// punya kemunculan, jadi batas ini hanya jaring pengaman agar tidak pernah menggantung.
func (s CronSchedule) Next(after time.Time) (time.Time, error) {
	t := after.In(BankZone)
	t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, BankZone).Add(time.Minute)
	limit := t.AddDate(10, 0, 0)
	for t.Before(limit) {
		if s.Matches(t) {
			return t, nil
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}, fmt.Errorf("%w: tidak ada jadwal berikutnya dalam 10 tahun", ErrInvalidCron)
}
