package ojkreport

import (
	"context"
	"sort"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// mapping_review.go menyajikan pemetaan COA DRAF sebagai data yang dapat ditinjau bank
// lewat antarmuka, dan menyimpan keputusan bank per baris.
//
// Pemetaan di coa_mapping.go sengaja tetap DRAF: SEOJK No. 16/SEOJK.03/2024 mewajibkan
// setiap BPR punya pedoman konversi sendiri, sehingga hanya bank yang boleh mengesahkan.
// Berkas ini TIDAK mengubah pemetaan maupun status ekspor; ia hanya membaca pemetaan,
// melengkapinya dengan nama akun/pos, dan menyimpan keputusan bank agar dapat diaudit.

// ReviewDecision adalah keputusan bank atas satu baris pemetaan.
// DISETUJUI berarti bank menyetujui usulan; DICATAT berarti bank mencatat keberatan.
type ReviewDecision string

const (
	ReviewApproved ReviewDecision = "DISETUJUI"
	ReviewNoted    ReviewDecision = "DICATAT"
)

// Valid menandai nilai keputusan yang dikenal. Keputusan asing ditolak, bukan disimpan.
func (d ReviewDecision) Valid() bool {
	return d == ReviewApproved || d == ReviewNoted
}

// MappingReview adalah keputusan bank tersimpan untuk satu baris pemetaan yang
// diidentifikasi (form, coa_code). Keputusan ini bertahan di basis data, bukan di kode.
type MappingReview struct {
	Form      string         `json:"form"`
	COACode   string         `json:"coa_code"`
	Decision  ReviewDecision `json:"decision"`
	Note      string         `json:"note"`
	DecidedBy string         `json:"decided_by"`
	// DecidedByID adalah uuid staf pelaku (string kosong bila tidak tersedia).
	DecidedByID string    `json:"decided_by_id,omitempty"`
	DecidedAt   time.Time `json:"decided_at"`
}

// MappingReviewRepository menyimpan dan membaca keputusan bank. Implementasinya ada di
// lapisan basis data; paket ini tetap tidak bergantung pada driver SQL.
type MappingReviewRepository interface {
	ListReviews(ctx context.Context) ([]MappingReview, error)
	UpsertReview(ctx context.Context, review MappingReview) error
}

// MappingReviewRow adalah satu baris pemetaan siap ditampilkan: kode COA beserta namanya,
// pos OJK tujuan, status verifikasi draf, dan keputusan bank bila sudah ada.
type MappingReviewRow struct {
	Form    string `json:"form"`
	COACode string `json:"coa_code"`
	COAName string `json:"coa_name"`
	// COAFound false berarti kode pemetaan tidak ada di bagan akun; antarmuka harus
	// menandainya, bukan menampilkan nama kosong seolah akunnya dikenal.
	COAFound bool   `json:"coa_found"`
	Sandi    string `json:"sandi"`
	PosName  string `json:"pos_name"`
	Sign     int    `json:"sign"`
	// DraftVerified dan DraftNote berasal dari berkas pemetaan dan tidak pernah diubah
	// handler; keputusan bank disimpan terpisah.
	DraftVerified bool   `json:"draft_verified"`
	DraftNote     string `json:"draft_note"`
	// Decision kosong berarti bank belum memutuskan baris ini.
	Decision   ReviewDecision `json:"decision,omitempty"`
	ReviewNote string         `json:"review_note,omitempty"`
	DecidedBy  string         `json:"decided_by,omitempty"`
	DecidedAt  *time.Time     `json:"decided_at,omitempty"`
}

// UnmappedCOA adalah kode COA yang muncul pada laporan sumber tetapi tidak ada di
// pemetaan. Inilah yang membuat ekspor ditolak (ErrIncompleteMapping).
type UnmappedCOA struct {
	Form    string `json:"form"`
	COACode string `json:"coa_code"`
	COAName string `json:"coa_name"`
}

// UnmappedPosition adalah pos OJK yang tidak punya satu pun sumber COA pada pemetaan,
// sehingga laporannya akan selalu nol walau jurnalnya ada.
type UnmappedPosition struct {
	Form    string `json:"form"`
	Sandi   string `json:"sandi"`
	PosName string `json:"pos_name"`
}

// BuildMappingReviewRows melengkapi setiap entri pemetaan dengan nama akun dan keputusan
// bank. Terurut menurut form lalu kode COA agar urutannya stabil di antarmuka.
func BuildMappingReviewRows(entries []MappingEntry, coaNames map[string]string, reviews []MappingReview) []MappingReviewRow {
	byLine := make(map[string]MappingReview, len(reviews))
	for _, rev := range reviews {
		byLine[MappingReviewKey(rev.Form, rev.COACode)] = rev
	}

	rows := make([]MappingReviewRow, 0, len(entries))
	for _, e := range entries {
		name, found := coaNames[e.COACode]
		if e.COACode == "-" {
			// Baris penyeimbang laba/rugi berjalan bukan akun bagan akun; jangan
			// dilaporkan sebagai akun tak dikenal.
			found = true
		}
		row := MappingReviewRow{
			Form:          e.Form,
			COACode:       e.COACode,
			COAName:       name,
			COAFound:      found,
			Sandi:         e.Sandi,
			PosName:       PositionName(e.Form, e.Sandi),
			Sign:          e.Sign,
			DraftVerified: e.Verified,
			DraftNote:     e.Note,
		}
		if rev, ok := byLine[MappingReviewKey(e.Form, e.COACode)]; ok {
			decidedAt := rev.DecidedAt
			row.Decision = rev.Decision
			row.ReviewNote = rev.Note
			row.DecidedBy = rev.DecidedBy
			row.DecidedAt = &decidedAt
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Form != rows[j].Form {
			return rows[i].Form < rows[j].Form
		}
		return rows[i].COACode < rows[j].COACode
	})
	return rows
}

// MappingReviewKey menyatukan identitas satu baris pemetaan. Form ikut menjadi bagian
// kunci karena satu sandi dapat dipakai beberapa form.
func MappingReviewKey(form, coaCode string) string {
	return form + "/" + coaCode
}

// KnownMappingLine menandai pasangan (form, kode COA) yang benar-benar ada pada pemetaan
// draf. Keputusan bank hanya boleh disimpan untuk baris yang ada, bukan kode karangan.
func KnownMappingLine(form, coaCode string) bool {
	for _, e := range COAMappingDraft {
		if e.Form == form && e.COACode == coaCode {
			return true
		}
	}
	return false
}

// PositionName mengembalikan nama pos OJK berdasarkan sandi dan form. Nama diambil dari
// susunan form (Lampiran II SEOJK 16/2024), bukan dikarang.
func PositionName(form, sandi string) string {
	for _, l := range formLinesFor(form) {
		if l.Sandi == sandi && l.Sandi != "" {
			return l.Name
		}
	}
	return ""
}

// formLinesFor memilih susunan baris menurut kode form. Form yang tidak dikenal
// menghasilkan daftar kosong, bukan panik.
func formLinesFor(form string) []formLine {
	switch form {
	case "01.00":
		return form01Lines
	case "02.00":
		return form02Lines
	default:
		return nil
	}
}

// UnmappedPositions mencari pos daun OJK (bukan baris total turunan) yang tidak punya
// satu pun entry pemetaan. Pos seperti ini akan selalu nol pada ekspor.
func UnmappedPositions(entries []MappingEntry) []UnmappedPosition {
	used := make(map[string]bool, len(entries))
	for _, e := range entries {
		used[e.Form+"/"+e.Sandi] = true
	}

	out := make([]UnmappedPosition, 0)
	for _, form := range []string{"01.00", "02.00"} {
		for _, l := range formLinesFor(form) {
			// Baris total dihitung dari pos anak, bukan dari saldo COA langsung;
			// baris tanpa sandi pun bukan pos. Keduanya bukan kekurangan pemetaan.
			if l.Sandi == "" || len(l.TotalFrom) > 0 {
				continue
			}
			if used[form+"/"+l.Sandi] {
				continue
			}
			out = append(out, UnmappedPosition{Form: form, Sandi: l.Sandi, PosName: l.Name})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Form != out[j].Form {
			return out[i].Form < out[j].Form
		}
		return out[i].Sandi < out[j].Sandi
	})
	return out
}

// UnmappedCOACodes mengumpulkan kode COA pada laporan sumber yang tidak ada di pemetaan,
// per form. Ini mencerminkan tepat apa yang akan ditolak ekspor (lihat Builder.collect),
// termasuk akun milik form lain yang memang sengaja dilewati tidak dianggap celah.
//
// "00000" adalah baris selisih jurnal, bukan kekurangan pemetaan; masalahnya
// ketidakseimbangan sumber, jadi tidak dicampur ke daftar ini.
func UnmappedCOACodes(bs *domain.BalanceSheet, is *domain.IncomeStatement, entries []MappingEntry) []UnmappedCOA {
	index := buildMappingIndex(entries)
	type key struct{ form, code string }
	seen := make(map[key]bool)
	out := make([]UnmappedCOA, 0)

	collectRows := func(form string, rows []domain.ReportRow) {
		for _, row := range rows {
			if row.AccountCode == "00000" {
				continue
			}
			// Kode yang sudah ada di pemetaan bukan celah, meskipun entry-nya
			// dipetakan ke form lain: sikap ini sama dengan Builder.collect.
			if _, ok := index[row.AccountCode]; !ok {
				k := key{form, row.AccountCode}
				if !seen[k] {
					seen[k] = true
					out = append(out, UnmappedCOA{Form: form, COACode: row.AccountCode, COAName: row.AccountName})
				}
			}
		}
	}
	if bs != nil {
		collectRows("01.00", bs.Rows)
	}
	if is != nil {
		collectRows("02.00", is.Rows)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Form != out[j].Form {
			return out[i].Form < out[j].Form
		}
		return out[i].COACode < out[j].COACode
	})
	return out
}

// HasImbalance menandai laporan sumber yang masih memuat baris penyeimbang "00000",
// artinya jurnal tidak tie-out. Dipakai antarmuka agar peninjau tahu kelengkapan
// pemetaan belum cukup bila jurnalnya sendiri belum seimbang.
func HasImbalance(bs *domain.BalanceSheet, is *domain.IncomeStatement) bool {
	has := func(rows []domain.ReportRow) bool {
		for _, row := range rows {
			if row.AccountCode == "00000" {
				return true
			}
		}
		return false
	}
	return (bs != nil && has(bs.Rows)) || (is != nil && has(is.Rows))
}
