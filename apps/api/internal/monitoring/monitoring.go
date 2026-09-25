// Package monitoring mengevaluasi kesehatan operasional bank dari data yang SUDAH ada
// (tanggal bisnis, riwayat langkah EOD, penanda run PPAP, parameter CKPN, dan status
// operasional instalasi). Seluruh fungsi evaluasi bersifat MURNI: masukannya nilai
// biasa dan keluarannya temuan; tidak ada I/O, tidak ada tulis-menulis, dan tidak ada
// perubahan state. Pengambilan data dari repositori dilakukan Collector.
//
// Paket ini sengaja tidak menambah dependensi apa pun. Tujuannya menjawab backlog W10
// "pemantauan & peringatan belum ada": kondisi yang benar-benar tersedia datanya
// dijadikan temuan ber-severity+kode+pesan, bukan tebakan.
package monitoring

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// Severity adalah tingkat kepentingan sebuah temuan.
type Severity string

const (
	// SeverityWarning: perlu tindakan operator tetapi tidak menghentikan operasi.
	SeverityWarning Severity = "WARNING"
	// SeverityCritical: operasi/akuntansi terancam dan harus segera ditangani.
	SeverityCritical Severity = "CRITICAL"
)

// Kode temuan. Nilainya stabil dan dipakai klien untuk memfilter/mengotomasi; pesan
// boleh berubah, kode tidak.
const (
	codeBusinessDateUnavailable = "business_date_unavailable"
	codeBusinessDateLagging     = "business_date_lagging"
	codeBusinessDateAhead       = "business_date_ahead"
	codeBusinessDateInEOD       = "business_date_in_eod"

	codeEODRunUnavailable = "eod_run_unavailable"
	codeEODRunMissing     = "eod_run_missing"
	codeEODStepFailed     = "eod_step_failed"
	codeEODPerCredit      = "eod_per_credit_failures"

	codeCKPNProvisional = "ckpn_parameters_provisional"
	codeCKPNOverdue     = "ckpn_parameters_overdue"
	codeCKPNExport      = "ckpn_ojk_export_blocked"

	codePPAPUnavailable = "ppap_run_unavailable"
	codePPAPMissing     = "ppap_run_missing"
	codePPAPLagging     = "ppap_run_lagging"

	codeOperationalUnavailable = "operational_status_unavailable"
	codeCKPNDisabled           = "ckpn_disabled_operational"
)

// Batas keterlambatan tanggal bisnis dalam hari kalender. Satu hari sudah layak
// diperingatkan (tutup hari biasanya harian); tiga hari dinaikkan menjadi CRITICAL
// karena operasional bank sudah terhenti beberapa hari.
const (
	BusinessDateLagWarningDays  = 1
	BusinessDateLagCriticalDays = 3
)

// Finding adalah satu temuan pemantauan.
type Finding struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

// Summary adalah cacah temuan per tingkat, agar klien tidak perlu menghitung ulang.
type Summary struct {
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
}

// Snapshot adalah hasil satu kali pemantauan.
type Snapshot struct {
	EvaluatedAt time.Time `json:"evaluated_at"`
	Findings    []Finding `json:"findings"`
	Summary     Summary   `json:"summary"`
}

// Data adalah potret bahan evaluasi: nilai biasa tanpa klien database, sehingga
// Evaluate dapat diuji tanpa basis data. Err dibawa apa adanya dan dilaporkan sebagai
// temuan "tidak dapat dibaca" — kegagalan membaca TIDAK boleh disamarkan menjadi
// "aman".
type Data struct {
	Now time.Time

	BusinessDate    *domain.SystemBusinessDate
	BusinessDateErr error

	// EODSteps adalah riwayat langkah pada tanggal bisnis berjalan. Kosong dengan
	// BusinessDate tidak nil berarti tutup hari belum dijalankan.
	EODSteps    []domain.EODStepRunRecord
	EODStepsErr error

	// PPAPKnown=false berarti belum pernah ada run PPAP yang berhasil (bukan error).
	PPAPLastDate time.Time
	PPAPKnown    bool
	PPAPErr      error

	CKPN        domain.CKPNParametersStatus
	CKPNEnabled bool

	// Operational nil berarti status operasional belum/tidak dapat dipastikan.
	Operational    *bool
	OperationalErr error
}

// Evaluate menjalankan seluruh pemeriksaan murni dan mengembalikan temuan terurut
// (CRITICAL lebih dulu, lalu kode) supaya keluaran deterministik.
func Evaluate(data Data) []Finding {
	var findings []Finding
	findings = append(findings, EvaluateBusinessDate(data.Now, data.BusinessDate, data.BusinessDateErr)...)
	findings = append(findings, EvaluateEODRun(data.EODSteps, data.EODStepsErr, data.BusinessDate != nil)...)
	findings = append(findings, EvaluateCKPNParameters(data.CKPN)...)
	findings = append(findings, EvaluatePPAPFreshness(data.BusinessDate, data.PPAPLastDate, data.PPAPKnown, data.PPAPErr, data.Operational)...)
	findings = append(findings, EvaluateCKPNReadiness(data.Operational, data.OperationalErr, data.CKPNEnabled)...)
	sortFindings(findings)
	return findings
}

// EvaluateBusinessDate memeriksa tanggal bisnis: tertinggal/di depan kalender, dan
// status IN_EOD_PROCESSING (tutup hari sedang berjalan atau terhenti).
func EvaluateBusinessDate(now time.Time, bd *domain.SystemBusinessDate, readErr error) []Finding {
	if readErr != nil {
		return []Finding{{
			Code:     codeBusinessDateUnavailable,
			Severity: SeverityCritical,
			Message:  fmt.Sprintf("tanggal bisnis tidak dapat dibaca: %v; posting dan tutup hari tidak dapat dipastikan", readErr),
		}}
	}
	if bd == nil {
		return []Finding{{
			Code:     codeBusinessDateUnavailable,
			Severity: SeverityCritical,
			Message:  "tanggal bisnis tidak tersedia; tanggal bisnis adalah periode posting dan tidak boleh ditebak",
		}}
	}

	var out []Finding
	lag := calendarDaysBetween(bd.CurrentDate, now)
	switch {
	case lag > 0:
		severity := SeverityWarning
		if lag >= BusinessDateLagCriticalDays {
			severity = SeverityCritical
		}
		out = append(out, Finding{
			Code:     codeBusinessDateLagging,
			Severity: severity,
			Message: fmt.Sprintf("tanggal bisnis %s tertinggal %d hari dari kalender %s; tutup hari belum memajukan periode",
				formatDate(bd.CurrentDate), lag, formatDate(now)),
		})
	case lag < 0:
		out = append(out, Finding{
			Code:     codeBusinessDateAhead,
			Severity: SeverityWarning,
			Message: fmt.Sprintf("tanggal bisnis %s berada %d hari di depan kalender %s; periksa setelan tanggal",
				formatDate(bd.CurrentDate), -lag, formatDate(now)),
		})
	}

	if bd.Status == domain.BusinessDateStatusEOD {
		out = append(out, Finding{
			Code:     codeBusinessDateInEOD,
			Severity: SeverityWarning,
			Message:  "status tanggal bisnis IN_EOD_PROCESSING: tutup hari sedang berjalan atau terhenti; posting ditolak sampai selesai",
		})
	}
	return out
}

// EvaluateEODRun memeriksa riwayat langkah EOD tanggal bisnis berjalan: ada-tidaknya
// run, langkah FAILED, dan kegagalan per-kredit yang tercatat di ringkasan langkah.
//
// businessDateKnown=false (tanggal bisnis tidak terbaca) menonaktifkan pemeriksaan
// "belum ada run", supaya ketiadaan data tidak dilaporkan sebagai tutup hari belum
// dijalankan.
func EvaluateEODRun(steps []domain.EODStepRunRecord, readErr error, businessDateKnown bool) []Finding {
	if readErr != nil {
		return []Finding{{
			Code:     codeEODRunUnavailable,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("riwayat langkah EOD tidak dapat dibaca: %v", readErr),
		}}
	}
	if businessDateKnown && len(steps) == 0 {
		return []Finding{{
			Code:     codeEODRunMissing,
			Severity: SeverityWarning,
			Message:  "belum ada riwayat langkah EOD untuk tanggal bisnis berjalan; tutup hari belum dijalankan",
		}}
	}

	var out []Finding
	for _, step := range steps {
		if step.Status == domain.EODStepFailed {
			msg := fmt.Sprintf("langkah EOD %s gagal pada tanggal bisnis %s", step.StepCode, formatDate(step.BusinessDate))
			if step.Reason != "" {
				msg += ": " + step.Reason
			}
			out = append(out, Finding{Code: codeEODStepFailed, Severity: SeverityCritical, Message: msg})
		}
		if n := summaryCount(step.Summary, "failed"); n > 0 {
			out = append(out, Finding{
				Code:     codeEODPerCredit,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("langkah EOD %s tanggal %s: %d kredit gagal dihitung pada run terakhir", step.StepCode, formatDate(step.BusinessDate), n),
			})
		}
	}
	return out
}

// EvaluateCKPNParameters memeriksa status parameter CKPN dari system_config: masih
// SEMENTARA, melewati batas ratifikasi, atau memblokir ekspor OJK.
func EvaluateCKPNParameters(status domain.CKPNParametersStatus) []Finding {
	var out []Finding
	if status.Sementara {
		out = append(out, Finding{
			Code:     codeCKPNProvisional,
			Severity: SeverityWarning,
			Message:  "parameter CKPN berstatus SEMENTARA: PD/LGD belum diratifikasi Direksi + akuntan (DPS untuk BPRS) dan DILARANG menjadi dasar kolom CKPN laporan OJK/APOLO",
		})
	}
	if status.DeadlinePassed {
		out = append(out, Finding{
			Code:     codeCKPNOverdue,
			Severity: SeverityCritical,
			Message: fmt.Sprintf("parameter CKPN SEMENTARA melewati batas ratifikasi %d bulan (sejak %s, batas %s); wajib diratifikasi atau diperpanjang dengan berita acara",
				status.RatificationMonths, status.TemporarySince, status.Deadline),
		})
	}
	if status.OJKExportBlocked {
		out = append(out, Finding{
			Code:     codeCKPNExport,
			Severity: SeverityCritical,
			Message:  "ekspor laporan OJK/APOLO diblokir karena parameter CKPN SEMENTARA dipakai saat ckpn.enabled menyala; ratifikasi parameter atau matikan CKPN resmi",
		})
	}
	return out
}

// EvaluatePPAPFreshness membandingkan tanggal bisnis berjalan dengan penanda run PPAP
// terakhir. Diabaikan bila instalasi jelas belum beroperasi (Operational false) supaya
// pemasangan baru tidak dibanjiri peringatan.
func EvaluatePPAPFreshness(bd *domain.SystemBusinessDate, last time.Time, known bool, readErr error, operational *bool) []Finding {
	if operational != nil && !*operational {
		return nil
	}
	if readErr != nil {
		return []Finding{{
			Code:     codePPAPUnavailable,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("penanda run PPAP terakhir tidak dapat dibaca: %v", readErr),
		}}
	}
	if !known {
		return []Finding{{
			Code:     codePPAPMissing,
			Severity: SeverityWarning,
			Message:  "belum ada run PPAP yang berhasil; perbandingan CKPN tidak memiliki dasar required_ppap pada tanggal bisnis berjalan",
		}}
	}
	if bd != nil && dateOnly(last).Before(dateOnly(bd.CurrentDate)) {
		return []Finding{{
			Code:     codePPAPLagging,
			Severity: SeverityWarning,
			Message: fmt.Sprintf("run PPAP terakhir %s mendahului tanggal bisnis berjalan %s; required_ppap berpotensi basi (perbandingan CKPN dapat ditolak)",
				formatDate(last), formatDate(bd.CurrentDate)),
		}}
	}
	return nil
}

// EvaluateCKPNReadiness memeriksa kewajiban CKPN: instalasi yang sudah beroperasi
// tetapi ckpn.enabled masih mati sedang melanggar kewajiban sejak 1 Januari 2025.
// Ini peringatan; fungsi ini tidak menyalakan CKPN.
func EvaluateCKPNReadiness(operational *bool, readErr error, ckpnEnabled bool) []Finding {
	if readErr != nil {
		return []Finding{{
			Code:     codeOperationalUnavailable,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("status operasional instalasi tidak dapat dibaca: %v", readErr),
		}}
	}
	if operational == nil || !*operational {
		return nil
	}
	if !ckpnEnabled {
		return []Finding{{
			Code:     codeCKPNDisabled,
			Severity: SeverityWarning,
			Message:  "CKPN belum dinyalakan (ckpn.enabled=false) padahal instalasi sudah beroperasi; CKPN SAK EP wajib sejak 1 Januari 2025 (POJK No. 1/2024 / POJK No. 24/2024)",
		}}
	}
	return nil
}

// sortFindings mengurutkan CRITICAL lebih dulu, lalu berdasarkan kode agar keluaran
// stabil dan mudah dibandingkan.
func sortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return findings[i].Severity == SeverityCritical
		}
		return findings[i].Code < findings[j].Code
	})
}

// summaryCount mengambil nilai numerik dari ringkasan langkah. Nilai dari JSONB
// (jalur produksi) berbentuk float64; pada uji dapat berupa int. Tipe lain diabaikan
// karena bukan angka yang dapat ditafsirkan.
func summaryCount(summary map[string]any, key string) int {
	if summary == nil {
		return 0
	}
	switch v := summary[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
	}
	return 0
}

// calendarDaysBetween mengembalikan selisih hari kalender dari `from` ke `to` menurut
// zona waktu bank (WIB), sehingga batas tengah malam tidak bergeser oleh zona UTC.
func calendarDaysBetween(from, to time.Time) int {
	return int(dateOnly(to).Sub(dateOnly(from)).Hours() / 24)
}

// dateOnly memotong sebuah waktu menjadi tanggal kalender di zona bank pada tengah
// malam UTC, agar pengurangan menghasilkan cacah hari bulat.
func dateOnly(t time.Time) time.Time {
	local := t.In(domain.BankZone)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

// formatDate memformat tanggal (tanpa jam) sebagai YYYY-MM-DD menurut zona waktu bank.
// Memakai zona bank, bukan zona server, agar tanggal yang ditampilkan sejalan dengan
// calendarDaysBetween dan tidak tampak kontradiktif pada server UTC dini hari WIB
// (mis. "tanggal bisnis 2026-09-24 tertinggal 1 hari dari kalender 2026-09-24").
func formatDate(t time.Time) string {
	return t.In(domain.BankZone).Format("2006-01-02")
}
