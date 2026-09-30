package ojkreport

import (
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// form00_19.go merakit Form 00.19 "STRUKTUR ORGANISASI BPR" sebagai DOKUMEN CETAK
// (HTML), bukan tabel angka.
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 00.19 "STRUKTUR ORGANISASI BPR": PDF #page 298 (hlm. tercetak 246). Terdaftar
//     di daftar Laporan Gabungan PDF #page 60 (hlm. 8).
//   - Isi peraturan: "Struktur organisasi BPR disampaikan oleh BPR dalam bentuk portable
//     document format (.pdf) kepada Otoritas Jasa Keuangan. Struktur organisasi BPR yang
//     dilaporkan mencakup susunan hierarki seluruh jaringan kantor yang dimiliki oleh BPR,
//     divisi atau satuan kerja, dan nama pegawai tetap atau tidak tetap BPR."
//
// KEPUTUSAN BENTUK BERKAS (disetujui pemilik 29 Sep 2026):
//   - Repo tidak memiliki generator PDF dan tidak menambah dependency baru. Dokumen
//     dirakit sebagai HTML siap cetak memakai pola dokumen yang sudah ada
//     (document_service.go: HTML + CSP dokumen); bank mencetak/menyimpan sebagai PDF dari
//     browser. Ini konsisten dengan cara dokumen cetak lain di sistem ini dikirim.
//
// BATAS SUMBER — jangan diisi tebakan:
//   - Hierarki kantor memakai bank_offices (migrasi 000112) apa adanya: nama, jenis,
//     sandi, alamat, kota, status. Sistem TIDAK mengarang relasi induk-anak antar kantor
//     yang tidak tersimpan di data.
//   - Nama pegawai pada direksi/dewan komisaris/pejabat eksekutif memakai bank_management
//     (migrasi 000112) per kategori. NIK tidak disimpan (keputusan privasi) dan karena itu
//     tidak dicetak.
//   - "Divisi atau satuan kerja" dibaca dari bank_work_units (migrasi 000131). Bila bank
//     belum mengisi, bagian itu ditandai belum diisi di dalam dokumen, bukan dicetak
//     kosong seolah lengkap dan bukan diisi contoh.
//   - Bila kelembagaan sama sekali kosong, dokumen tetap dirakit dengan peringatan bahwa
//     bank belum mengisi data — bukan digagalkan dan bukan diisi contoh.

// form00_19CategoryOrder menentukan urutan kategori manajemen pada dokumen. Kategori di
// luar daftar ini tetap dicetak, diurutkan menurut nilai sandinya, agar tidak ada data
// yang hilang hanya karena urutan tidak dikenal.
var form00_19CategoryOrder = []string{"KOMISARIS", "DIREKSI", "PEJABAT_EKSEKUTIF"}

// form00_19CategoryLabel memberi judul bagian yang terbaca untuk kategori manajemen.
func form00_19CategoryLabel(category string) string {
	switch strings.ToUpper(strings.TrimSpace(category)) {
	case "KOMISARIS":
		return "Dewan Komisaris"
	case "DIREKSI":
		return "Direksi"
	case "PEJABAT_EKSEKUTIF":
		return "Pejabat Eksekutif"
	default:
		// Kategori tak dikenal dicetak apa adanya, bukan diterjemahkan menjadi judul
		// yang mungkin salah.
		return strings.TrimSpace(category)
	}
}

// BuildForm00_19Document merakit dokumen HTML Form 00.19 dari laporan kelembagaan.
// Fungsi ini murni (tanpa basis data) sehingga dapat diuji langsung.
func BuildForm00_19Document(report domain.KelembagaanReport, bankName string) string {
	esc := html.EscapeString
	namaBank := strings.TrimSpace(bankName)
	if namaBank == "" {
		// Identitas bank tidak boleh dikarang di kode dokumen.
		namaBank = "PROFIL BANK BELUM DIKONFIGURASI"
	}

	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"id\">\n<head>\n")
	b.WriteString("<meta charset=\"UTF-8\">\n")
	b.WriteString("<title>STRUKTUR ORGANISASI BPR</title>\n")
	b.WriteString(`<style>
        body { font-family: Arial, Helvetica, sans-serif; width: 210mm; padding: 15mm; color: #0f172a; background: #fff; }
        h1 { font-size: 16px; text-align: center; margin: 0 0 2px; letter-spacing: 1px; }
        h2 { font-size: 13px; text-align: center; margin: 0 0 2px; font-weight: normal; color: #334155; }
        h3 { font-size: 12px; margin: 18px 0 6px; text-transform: uppercase; letter-spacing: .5px; border-bottom: 1px solid #94a3b8; padding-bottom: 3px; }
        .meta { text-align: center; font-size: 11px; color: #475569; margin-bottom: 14px; }
        table { width: 100%; border-collapse: collapse; font-size: 11px; }
        th, td { border: 1px solid #94a3b8; padding: 4px 6px; text-align: left; vertical-align: top; }
        th { background: #e2e8f0; }
        .empty { font-size: 11px; font-style: italic; color: #475569; margin: 4px 0; }
        .warn { border: 1px solid #b45309; background: #fffbeb; color: #7c2d12; font-size: 11px; padding: 8px 10px; margin: 10px 0; border-radius: 4px; }
        .sign { margin-top: 28px; font-size: 11px; }
        @media print { body { padding: 10mm; } }
    </style>
</head>
<body>
`)
	fmt.Fprintf(&b, "<h1>%s</h1>\n", esc(namaBank))
	b.WriteString("<h2>STRUKTUR ORGANISASI BPR</h2>\n")
	b.WriteString("<div class=\"meta\">Form 00.19 &mdash; Lampiran II SEOJK No. 16/SEOJK.03/2024</div>\n")
	if !report.AsOf.IsZero() {
		fmt.Fprintf(&b, "<div class=\"meta\">Posisi: %s</div>\n", esc(report.AsOf.Format("02-01-2006")))
	}

	// Peringatan sumber: apa yang belum dimodelkan harus terlihat oleh pembaca dokumen,
	// bukan disembunyikan.
	b.WriteString("<div class=\"warn\"><strong>Catatan kelengkapan dokumen.</strong> ")
	b.WriteString("Susunan hierarki mengikuti data jaringan kantor, pejabat, dan divisi/satuan kerja yang bank isi. ")
	b.WriteString("Lengkapi bagian yang masih kosong secara manual sebelum dokumen dikirim ke OJK.")
	b.WriteString("</div>\n")
	for _, w := range report.Warnings {
		fmt.Fprintf(&b, "<div class=\"warn\">%s</div>\n", esc(w))
	}

	// Jaringan kantor.
	b.WriteString("<h3>Jaringan Kantor</h3>\n")
	if len(report.Offices) == 0 {
		b.WriteString("<p class=\"empty\">Belum ada kantor yang diisi bank (register jaringan kantor kosong).</p>\n")
	} else {
		b.WriteString("<table>\n<thead><tr>")
		for _, h := range []string{"Sandi", "Jenis", "Nama Kantor", "Alamat", "Kota", "Status"} {
			fmt.Fprintf(&b, "<th>%s</th>", h)
		}
		b.WriteString("</tr></thead>\n<tbody>\n")
		for _, office := range form00_19SortedOffices(report.Offices) {
			fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
				esc(form00_19OrDash(office.Code)),
				esc(form00_19OrDash(office.OfficeType)),
				esc(form00_19OrDash(office.Name)),
				esc(form00_19OrDash(office.Address)),
				esc(form00_19OrDash(office.City)),
				esc(form00_19OrDash(office.Status)))
		}
		b.WriteString("</tbody>\n</table>\n")
	}

	// Pejabat: satu bagian per kategori, urutan tetap agar dokumen stabil.
	grouped := map[string][]domain.BankManagement{}
	var categories []string
	for _, m := range report.Management {
		key := strings.ToUpper(strings.TrimSpace(m.Category))
		if _, ok := grouped[key]; !ok {
			categories = append(categories, key)
		}
		grouped[key] = append(grouped[key], m)
	}
	sort.SliceStable(categories, func(i, j int) bool {
		return form00_19CategoryRank(categories[i]) < form00_19CategoryRank(categories[j])
	})

	b.WriteString("<h3>Susunan Pengurus</h3>\n")
	if len(report.Management) == 0 {
		b.WriteString("<p class=\"empty\">Belum ada pejabat yang diisi bank (register manajemen kosong).</p>\n")
	}
	for _, category := range categories {
		fmt.Fprintf(&b, "<h3>%s</h3>\n", esc(form00_19CategoryLabel(category)))
		b.WriteString("<table>\n<thead><tr>")
		for _, h := range []string{"Nama", "Jabatan", "Sandi Jabatan OJK", "Tanggal Mulai", "Status"} {
			fmt.Fprintf(&b, "<th>%s</th>", h)
		}
		b.WriteString("</tr></thead>\n<tbody>\n")
		for _, m := range grouped[category] {
			fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
				esc(form00_19OrDash(m.Name)),
				esc(form00_19OrDash(m.Position)),
				esc(form00_19OrDash(m.OJKPositionCode)),
				esc(form00_19Date(m.StartedAt)),
				esc(form00_19OrDash(m.Status)))
		}
		b.WriteString("</tbody>\n</table>\n")
	}

	// Divisi atau satuan kerja (sumber: bank_work_units, migrasi 000131).
	b.WriteString("<h3>Divisi atau Satuan Kerja</h3>\n")
	if len(report.WorkUnits) == 0 {
		b.WriteString("<p class=\"empty\">Belum ada divisi/satuan kerja yang diisi bank (register kosong). ")
		b.WriteString("Bagian ini harus dilengkapi manual sebelum dokumen dikirim ke OJK.</p>\n")
	} else {
		b.WriteString("<table>\n<thead><tr>")
		for _, h := range []string{"Kode", "Jenis", "Nama Unit", "Unit Induk", "Kepala Unit", "Jumlah Pegawai"} {
			fmt.Fprintf(&b, "<th>%s</th>", h)
		}
		b.WriteString("</tr></thead>\n<tbody>\n")
		for _, u := range form00_19SortedWorkUnits(report.WorkUnits) {
			fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
				esc(form00_19OrDash(u.Code)),
				esc(form00_19OrDash(u.Jenis)),
				esc(form00_19OrDash(u.Nama)),
				esc(form00_19OrDash(u.ParentCode)),
				esc(form00_19OrDash(u.KepalaUnit)),
				esc(form00_19JumlahPegawai(u.JumlahPegawai)))
		}
		b.WriteString("</tbody>\n</table>\n")
	}

	b.WriteString("<div class=\"sign\">Dicetak dari CBS Core untuk dilengkapi dan disimpan sebagai PDF sebelum disampaikan ke OJK.</div>\n")
	b.WriteString("</body>\n</html>\n")
	return b.String()
}

// form00_19SortedWorkUnits mengurutkan divisi/satuan kerja: urutan lebih dulu, lalu kode.
// Urutan deterministik supaya dokumen tidak berubah-ubah antar ekspor.
func form00_19SortedWorkUnits(units []domain.BankWorkUnit) []domain.BankWorkUnit {
	out := append([]domain.BankWorkUnit(nil), units...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Urutan != out[j].Urutan {
			return out[i].Urutan < out[j].Urutan
		}
		return out[i].Code < out[j].Code
	})
	return out
}

// form00_19JumlahPegawai menulis jumlah pegawai; nil berarti belum diisi (bukan nol).
func form00_19JumlahPegawai(n *int) string {
	if n == nil {
		return "-"
	}
	return strconv.Itoa(*n)
}

// form00_19SortedOffices mengurutkan kantor: sandi lebih dulu (kosong di akhir), lalu nama.
// Urutan deterministik supaya dokumen tidak berubah-ubah antar ekspor.
func form00_19SortedOffices(offices []domain.BankOffice) []domain.BankOffice {
	out := append([]domain.BankOffice(nil), offices...)
	sort.SliceStable(out, func(i, j int) bool {
		ci, cj := strings.TrimSpace(out[i].Code), strings.TrimSpace(out[j].Code)
		if (ci == "") != (cj == "") {
			return ci != ""
		}
		if ci != cj {
			return ci < cj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// form00_19CategoryRank memberi urutan kategori yang dikenal; kategori tak dikenal
// diletakkan setelah yang dikenal tanpa dibuang.
func form00_19CategoryRank(category string) int {
	for i, known := range form00_19CategoryOrder {
		if category == known {
			return i
		}
	}
	return len(form00_19CategoryOrder)
}

// form00_19OrDash menulis nilai yang belum diisi sebagai "-", bukan dikosongkan.
func form00_19OrDash(s string) string {
	if trimmed := strings.TrimSpace(s); trimmed != "" {
		return trimmed
	}
	return "-"
}

// form00_19Date menulis tanggal yang boleh kosong sebagai TT-BB-TTTT.
func form00_19Date(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "-"
	}
	return t.Format("02-01-2006")
}
