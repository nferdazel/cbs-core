package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// ckpn_parameters.go memuat status SEMENTARA/FINAL parameter CKPN beserta pengamannya.
//
// Dasar: docs/KEPUTUSAN-PANEL-RISIKO-CKPN.md butir 1.4–1.5. Nilai PD/LGD yang dipakai
// produksi saat ini DISANDERA dari tarif PPKA/LGD 0,45, jadi BUKAN hasil perhitungan PD
// menurut PA BPR 12.6/12.7. Selama belum diratifikasi Direksi + akuntan (DPS untuk
// BPRS), parameter itu berstatus SEMENTARA dan HANYA untuk internal: dilarang menjadi
// dasar kolom CKPN pada laporan OJK/APOLO. Berkas ini membuat status itu TERLIHAT dan
// menutup ekspornya, bukan menebak angka pengganti.
//
// Tidak ada tabel baru: tanggal mulai dan batas waktu memakai kunci system_config yang
// sudah ada polanya (nilai teks), sehingga tidak perlu migrasi skema.
const (
	// ConfigKeyCKPNParametersStatus menyimpan status parameter: SEMENTARA atau FINAL.
	// Nilai tak dikenal (termasuk kosong) diperlakukan SEMENTARA — gagal-aman, karena
	// salah membaca SEMENTARA sebagai FINAL berarti angka sementara dipakai ke OJK.
	ConfigKeyCKPNParametersStatus = "ckpn.parameters.status"
	// ConfigKeyCKPNParametersSince adalah tanggal (YYYY-MM-DD) parameter sementara
	// mulai dipakai. Dipakai menghitung batas ratifikasi. Kosong berarti belum tercatat.
	ConfigKeyCKPNParametersSince = "ckpn.parameters.temporary_since"
	// ConfigKeyCKPNParametersMonths adalah batas ratifikasi dalam bulan (bawaan 12).
	ConfigKeyCKPNParametersMonths = "ckpn.parameters.ratification_months"
	// ConfigKeyCKPNFloorPPKA adalah saklar lantai wajib PPKA setelah ratifikasi. Selama
	// SEMENTARA lantai selalu ditegakkan tanpa memandang kunci ini (butir 1.6).
	ConfigKeyCKPNFloorPPKA = "ckpn.floor.ppka_enabled"
	// ConfigKeyCKPNEnabled adalah saklar CKPN resmi. Dipakai di sini hanya untuk
	// memutuskan apakah angka parameter sementara benar-benar mengalir ke laporan
	// (karena itu wajib diblokir) atau laporan masih memakai PPKA (sehingga menutup
	// laporannya justru memblokir hal yang benar).
	ConfigKeyCKPNEnabled = "ckpn.enabled"

	// Kunci BUKTI RATIFIKASI parameter CKPN (keputusan panel butir 1). Ratifikasi adalah
	// tindakan manusia bertanda tangan (Direksi + akuntan; DPS untuk BPRS), sehingga
	// status FINAL HANYA boleh disetel bila bukti-bukti ini lengkap. Format bukti
	// sengaja paling sederhana: kunci konfigurasi bernilai teks/tanggal/penanda, tanpa
	// tabel baru. Migrasi 000095 menegakkan syarat ini lewat trigger pada system_config,
	// sehingga jalur SQL langsung pun ditolak bila bukti kurang.
	//
	// Nama pengesah/nomor berita acara adalah DATA BANK, bukan nilai yang boleh dikarang
	// sistem; panel tidak mengisinya. Yang dapat diputuskan mesin hanyalah KELENGKAPAN.
	ConfigKeyCKPNRatificationBANumber   = "ckpn.ratification.ba_number"
	ConfigKeyCKPNRatificationBADate     = "ckpn.ratification.ba_date"
	ConfigKeyCKPNRatificationApprovedBy = "ckpn.ratification.approved_by"
	ConfigKeyCKPNRatificationPDLGDBasis = "ckpn.ratification.pd_lgd_basis"
	// ConfigKeyCKPNRatificationPDLGDFromBank menandai PD/LGD benar-benar dihitung dari
	// data historis bank (PA BPR 12.6/12.7), bukan angka turunan tarif PPKA. Nilai
	// selain true membuat perubahan ke FINAL ditolak.
	ConfigKeyCKPNRatificationPDLGDFromBank = "ckpn.ratification.pd_lgd_from_bank"
)

const (
	// CKPNParameterStatusSementara menandai parameter belum diratifikasi.
	CKPNParameterStatusSementara = "SEMENTARA"
	// CKPNParameterStatusFinal menandai parameter sudah diratifikasi bank/akuntan.
	CKPNParameterStatusFinal = "FINAL"
	// CKPNRatificationMonthsDefault adalah batas ratifikasi bawaan (butir 1.4.2).
	CKPNRatificationMonthsDefault = 12
)

// CKPNParametersStatus adalah ringkasan status parameter CKPN untuk ditampilkan dan
// diperiksa tanpa menjalankan tutup hari. Seluruh bidang bersifat BACA-SAJA: fungsi
// yang membentuknya tidak menulis konfigurasi dan tidak mengubah angka apa pun.
type CKPNParametersStatus struct {
	// Status adalah SEMENTARA atau FINAL. Nilai tak dikenal dilaporkan sebagai
	// SEMENTARA supaya salah-konfigurasi tidak membuka laporan OJK.
	Status string `json:"status"`
	// Sementara true berarti parameter belum diratifikasi.
	Sementara bool `json:"sementara"`
	// TemporarySince adalah tanggal mulai (YYYY-MM-DD) bila tercatat.
	TemporarySince string `json:"temporary_since,omitempty"`
	// RatificationMonths adalah batas ratifikasi dalam bulan (bawaan 12).
	RatificationMonths int `json:"ratification_months"`
	// Deadline adalah tanggal batas ratifikasi (YYYY-MM-DD) bila tanggal mulai tercatat.
	Deadline string `json:"ratification_deadline,omitempty"`
	// DeadlinePassed true berarti umur parameter melewati batas 12 bulan; peringatan
	// tingkat tinggi wajib muncul (butir 1.4.4).
	DeadlinePassed bool `json:"deadline_passed"`
	// FloorPPKAEnforced true berarti target CKPN dijaga tidak di bawah required_ppap.
	FloorPPKAEnforced bool `json:"floor_ppka_enforced"`
	// OJKExportBlocked true berarti laporan OJK/APOLO menolak angka CKPN ini. Selama
	// SEMENTARA blokir berlaku (butir 1.5).
	OJKExportBlocked bool `json:"ojk_export_blocked"`
	// OJKExportBlockReason menjelaskan sebab dan langkah perbaikan, bukan hanya menolak.
	OJKExportBlockReason string `json:"ojk_export_block_reason,omitempty"`
	// RatificationReady true berarti bukti ratifikasi (nomor/tanggal berita acara,
	// pengesah, dasar PD/LGD dari data bank) sudah lengkap sehingga status FINAL SAH
	// disetel. RatificationMissing menyebut bukti yang kurang — bukan hanya menolak.
	RatificationReady   bool     `json:"ratification_ready"`
	RatificationMissing []string `json:"ratification_missing,omitempty"`
	// EnablementGaps adalah daftar yang masih menahan penyalakan ckpn.enabled pada
	// instalasi ini (keputusan panel butir 2). Kosong berarti tidak ada penahan yang
	// dapat diperiksa mesin. Daftarnya menyebut kunci yang harus diselesaikan, bukan
	// menebak nilainya.
	EnablementGaps []string `json:"enablement_gaps,omitempty"`
	// Warnings adalah peringatan yang wajib ditampilkan (start, EOD, KPMM/PPAP, status).
	Warnings []string `json:"warnings,omitempty"`
}

// CKPNParametersStatusFromConfig membaca status parameter dari konfigurasi. now dipakai
// menghitung umur parameter. cfg nil (mis. lingkungan tanpa konfigurasi) diperlakukan
// SEMENTARA: gagal-aman, bukan seolah sudah final.
//
// Fungsi ini murni baca-saja. Ia tidak menulis konfigurasi, tidak menyentuh angka
// CKPN, dan tidak memblokir perhitungan bayangan.
func CKPNParametersStatusFromConfig(ctx context.Context, cfg SystemConfigService, now time.Time) CKPNParametersStatus {
	out := CKPNParametersStatus{
		Status:             CKPNParameterStatusSementara,
		Sementara:          true,
		RatificationMonths: CKPNRatificationMonthsDefault,
		FloorPPKAEnforced:  true,
	}

	statusRaw := CKPNParameterStatusSementara
	if cfg != nil {
		statusRaw = strings.ToUpper(strings.TrimSpace(cfg.GetString(ctx, ConfigKeyCKPNParametersStatus, CKPNParameterStatusSementara)))
	}
	switch statusRaw {
	case CKPNParameterStatusFinal:
		out.Status = CKPNParameterStatusFinal
		out.Sementara = false
	case CKPNParameterStatusSementara, "":
		out.Status = CKPNParameterStatusSementara
		out.Sementara = true
	default:
		// Nilai asing: jangan menebak sah. Perlakukan SEMENTARA agar tidak ada ekspor
		// yang lolos hanya karena salah tulis.
		out.Status = CKPNParameterStatusSementara
		out.Sementara = true
		out.Warnings = append(out.Warnings, "nilai "+ConfigKeyCKPNParametersStatus+" tidak dikenal; diperlakukan SEMENTARA (gagal-aman). Nilai sah: SEMENTARA atau FINAL.")
	}

	if cfg != nil {
		if v := cfg.GetInt(ctx, ConfigKeyCKPNParametersMonths, CKPNRatificationMonthsDefault); v > 0 {
			out.RatificationMonths = v
		}
		out.TemporarySince = strings.TrimSpace(cfg.GetString(ctx, ConfigKeyCKPNParametersSince, ""))
		// Lantai PPKA: selama SEMENTARA wajib; setelah FINAL menjadi pilihan bank
		// (bawaan true). Bank tidak boleh mematikannya selama SEMENTARA.
		out.FloorPPKAEnforced = out.Sementara || cfg.GetBool(ctx, ConfigKeyCKPNFloorPPKA, true)
	}

	// Bukti ratifikasi dan daftar penahan penyalakan diperiksa mesin (butir 1 & 2).
	out.RatificationReady, out.RatificationMissing = CKPNRatificationReadiness(ctx, cfg, now)
	out.EnablementGaps = ckpnEnablementGaps(ctx, cfg)

	if out.Sementara {
		out.Warnings = append(out.Warnings,
			"PARAMETER SEMENTARA — belum disetujui bank/akuntan. Angka CKPN ini HANYA untuk internal (mode bayangan/laporan manajemen) dan DILARANG dipakai sebagai dasar kolom CKPN laporan OJK/APOLO. Ratifikasi Direksi + akuntan (DPS untuk BPRS), lalu setel "+ConfigKeyCKPNParametersStatus+"=FINAL.")
		if len(out.RatificationMissing) > 0 {
			out.Warnings = append(out.Warnings,
				"bukti ratifikasi parameter CKPN belum lengkap; perubahan ke "+ConfigKeyCKPNParametersStatus+"=FINAL akan DITOLAK sampai dilengkapi: "+strings.Join(out.RatificationMissing, "; ")+".")
		}
		// Blokir ekspor hanya berlaku bila angka sementara ini benar-benar mengalir ke
		// laporan, yaitu saat CKPN resmi menyala. Selama ckpn.enabled masih mati, laporan
		// OJK memuat angka PPKA (bukan CKPN dari PD/LGD sementara), sehingga memblokirnya
		// akan menutup laporan yang sehat tanpa menutup risiko yang dimaksud panel.
		if cfg == nil || cfg.GetBool(ctx, ConfigKeyCKPNEnabled, false) {
			out.OJKExportBlocked = true
			out.OJKExportBlockReason = "parameter CKPN berstatus SEMENTARA (" + ConfigKeyCKPNParametersStatus + ") dan CKPN resmi sudah menyala: angka PD/LGD belum diratifikasi Direksi + akuntan (DPS untuk BPRS), sehingga laporan OJK/APOLO tidak boleh memakainya. Untuk mengirim laporan OJK: (1) hitung PD menurut PA BPR 12.6 dan LGD menurut 12.7 dari data historis bank (atau pakai excel parameter resmi OJK), (2) ratifikasi Direksi + akuntan (DPS untuk BPRS), (3) setel " + ConfigKeyCKPNParametersStatus + "=FINAL. Selama belum ada pengganti, laporan OJK memakai PPKA pada kolom CKPN dengan pengungkapan selisih kepada OJK."
		} else {
			out.Warnings = append(out.Warnings,
				"ekspor OJK belum diblokir karena "+ConfigKeyCKPNEnabled+" masih mati: laporan OJK saat ini memuat angka PPKA, bukan CKPN dari parameter sementara. Blokir akan berlaku otomatis begitu CKPN resmi dinyalakan sebelum ratifikasi.")
		}
	}

	// Batas 12 bulan sejak tanggal yang dicatat sistem. Tanggal kosong bukan alasan
	// mengarang batas; ia dilaporkan supaya operator mengisinya.
	if out.Sementara {
		if out.TemporarySince == "" {
			out.Warnings = append(out.Warnings, "tanggal mulai parameter sementara ("+ConfigKeyCKPNParametersSince+") belum tercatat; umur parameter dan batas ratifikasi "+itoa(out.RatificationMonths)+" bulan belum dapat dihitung.")
		} else if t, err := time.Parse("2006-01-02", out.TemporarySince); err != nil {
			out.Warnings = append(out.Warnings, "tanggal mulai parameter sementara ("+ConfigKeyCKPNParametersSince+") bukan format YYYY-MM-DD: "+out.TemporarySince+"; batas ratifikasi tidak dapat dihitung.")
		} else {
			deadline := t.AddDate(0, out.RatificationMonths, 0)
			out.Deadline = deadline.Format("2006-01-02")
			if !now.Before(deadline) {
				out.DeadlinePassed = true
				out.Warnings = append(out.Warnings,
					"PERINGATAN TINGKAT TINGGI: parameter CKPN SEMENTARA sudah melewati batas ratifikasi "+itoa(out.RatificationMonths)+" bulan (sejak "+out.TemporarySince+", batas "+out.Deadline+"). Wajib diratifikasi Direksi + akuntan (DPS untuk BPRS) atau diperpanjang dengan berita acara bertanggal; laporan OJK tetap DILARANG memakai angka ini.")
			}
		}
	}
	return out
}

// CKPNRatificationReadiness memeriksa KELENGKAPAN bukti ratifikasi parameter CKPN
// (keputusan panel butir 1). Ia mengembalikan ready=true hanya bila seluruh bukti
// tersedia, beserta daftar yang kurang supaya bank tahu persis apa yang harus diisi.
//
// Ini BUKAN ratifikasi: sistem tidak dapat dan tidak boleh meratifikasi atas nama bank.
// Yang diperiksa mesin hanya kelengkapan/format — kebenaran isi tetap tanggung jawab
// Direksi + akuntan (dan DPS untuk BPRS) yang menandatangani berita acara.
//
// cfg nil diperlakukan belum lengkap (gagal-aman), bukan siap.
func CKPNRatificationReadiness(ctx context.Context, cfg SystemConfigService, now time.Time) (bool, []string) {
	if cfg == nil {
		return false, []string{"konfigurasi tidak terbaca (SystemConfigService nil); bukti ratifikasi tidak dapat diverifikasi"}
	}
	var missing []string

	requireText := func(key, label string) {
		if strings.TrimSpace(cfg.GetString(ctx, key, "")) == "" {
			missing = append(missing, key+" ("+label+")")
		}
	}
	requireText(ConfigKeyCKPNRatificationBANumber, "nomor berita acara ratifikasi Direksi + akuntan/DPS")
	requireText(ConfigKeyCKPNRatificationApprovedBy, "nama dan jabatan pengesah")
	requireText(ConfigKeyCKPNRatificationPDLGDBasis, "dasar perhitungan PD (PA BPR 12.6) dan LGD (12.7) dari data historis bank")

	if !cfg.GetBool(ctx, ConfigKeyCKPNRatificationPDLGDFromBank, false) {
		missing = append(missing, ConfigKeyCKPNRatificationPDLGDFromBank+"=true (PD/LGD dihitung dari data historis bank, bukan turunan tarif PPKA)")
	}

	rawDate := strings.TrimSpace(cfg.GetString(ctx, ConfigKeyCKPNRatificationBADate, ""))
	switch {
	case rawDate == "":
		missing = append(missing, ConfigKeyCKPNRatificationBADate+" (tanggal berita acara ratifikasi, YYYY-MM-DD)")
	default:
		t, err := time.Parse("2006-01-02", rawDate)
		if err != nil {
			missing = append(missing, ConfigKeyCKPNRatificationBADate+" bukan format YYYY-MM-DD: "+rawDate)
		} else if t.After(tanggalSaja(now)) {
			missing = append(missing, ConfigKeyCKPNRatificationBADate+" tidak boleh di masa depan: "+rawDate)
		}
	}
	return len(missing) == 0, missing
}

// ckpnEnablementGaps mengembalikan daftar yang menahan penyalakan ckpn.enabled pada
// instalasi ini (keputusan panel butir 2). Ia baca-saja dan tidak menyalakan apa pun.
// Daftarnya dapat diperiksa mesin: parameter terisi sah, sudah diratifikasi, dan akun
// syariah terpetakan bila instalasi melayani buku syariah.
//
// Bila ckpn.enabled sudah menyala, tidak ada penahan (kosong). Peringatan kesiapan
// instalasi (jurnal/tanggal bisnis) tetap milik service.CKPNReadinessWarnings.
func ckpnEnablementGaps(ctx context.Context, cfg SystemConfigService) []string {
	if cfg == nil {
		return []string{"konfigurasi tidak terbaca (SystemConfigService nil); penahan penyalakan CKPN tidak dapat diperiksa"}
	}
	if cfg.GetBool(ctx, ConfigKeyCKPNEnabled, false) {
		return nil
	}
	var gaps []string

	status := strings.ToUpper(strings.TrimSpace(cfg.GetString(ctx, ConfigKeyCKPNParametersStatus, CKPNParameterStatusSementara)))
	if status != CKPNParameterStatusFinal {
		if ready, missing := CKPNRatificationReadiness(ctx, cfg, time.Now()); ready {
			gaps = append(gaps, "parameter CKPN belum FINAL padahal bukti ratifikasi lengkap; setel "+ConfigKeyCKPNParametersStatus+"=FINAL")
		} else {
			gaps = append(gaps, "parameter CKPN belum diratifikasi (status "+ConfigKeyCKPNParametersStatus+" bukan FINAL); bukti kurang: "+strings.Join(missing, "; "))
		}
	}

	// PD per golongan dan LGD wajib berupa fraksi 0..1 yang benar-benar terisi.
	for i := 1; i <= 5; i++ {
		key := fmt.Sprintf("ckpn.pd_frac.gol_%d", i)
		if !validCKPNFraction(ctx, cfg, key) {
			gaps = append(gaps, key+" belum diisi atau bukan fraksi 0..1 (satuan FRAKSI, bukan persen)")
		}
	}
	if !validCKPNFraction(ctx, cfg, "ckpn.lgd_frac") {
		gaps = append(gaps, "ckpn.lgd_frac belum diisi atau bukan fraksi 0..1 (satuan FRAKSI, bukan persen)")
	}

	// Instalasi yang melayani buku syariah wajib memetakan akun CKPN syariah; bila
	// kosong, jurnal CKPN pembiayaan jatuh ke akun konvensional.
	scope := ParseInstitutionBookScope(cfg.GetString(ctx, ConfigKeyInstitutionBookScope, string(ScopeDual)))
	if scope.AllowsBook(BookSyariah) {
		for _, key := range []string{"ckpn.coa.expense.syariah", "ckpn.coa.reserve.syariah"} {
			if strings.TrimSpace(cfg.GetString(ctx, key, "")) == "" {
				gaps = append(gaps, key+" belum dipetakan ke akun CKPN syariah")
			}
		}
	}
	return gaps
}

// validCKPNFraction melaporkan apakah kunci terisi angka dan berada pada rentang 0..1.
// Kunci kosong dan nilai di luar rentang sama-sama dianggap belum sah.
func validCKPNFraction(ctx context.Context, cfg SystemConfigService, key string) bool {
	raw := strings.TrimSpace(cfg.GetString(ctx, key, ""))
	if raw == "" {
		return false
	}
	d, err := decimal.NewFromString(raw)
	if err != nil {
		return false
	}
	return d.GreaterThanOrEqual(decimal.Zero) && d.LessThanOrEqual(decimal.NewFromInt(1))
}

// itoa menghindari impor strconv hanya untuk satu bilangan; nilai selalu positif.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
