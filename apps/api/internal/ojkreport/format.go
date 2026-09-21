package ojkreport

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// ColumnSeparator adalah pemisah kolom berkas teks. Dipilih "|" karena mudah
// dibaca mata dan jarang muncul di nama pos.
const ColumnSeparator = "|"

// FormatRupiah menulis nominal dalam rupiah penuh tanpa desimal. Nilai nol tetap
// ditulis "0" (bukan dikosongkan) agar pos tanpa nilai terlihat jelas di berkas.
func FormatRupiah(d decimal.Decimal) string {
	return d.Round(0).StringFixed(0)
}

// FormatPersen menulis nilai rasio dalam persen dengan dua desimal. Dua desimal
// dipakai agar pembulatan rupiah penuh tidak menghapus ketelitian rasio.
func FormatPersen(d decimal.Decimal) string {
	return d.Round(2).StringFixed(2)
}

// WriteText menulis bundle sebagai berkas teks yang dapat diperiksa manusia
// sebelum dikirim ke APOLO. Format: FORM|SANDI|NAMA POS|JUMLAH.
func WriteText(w io.Writer, b *Bundle) error {
	write := func(format string, args ...any) error {
		_, err := fmt.Fprintf(w, format, args...)
		return err
	}

	metadata := []string{
		"# EKSPOR LAPORAN OJK (APOLO) - FONDASI, BUKAN BERKAS KIRIM SIAP PAKAI",
		fmt.Sprintf("# Periode          : %s", b.Period.Format("2006-01")),
		fmt.Sprintf("# Akhir periode    : %s", b.PeriodEnd.Format("2006-01-02")),
		fmt.Sprintf("# Tenggat          : %s (koreksi s/d %s)",
			b.Deadline.Format("2006-01-02"), b.CorrectionDeadline.Format("2006-01-02")),
		fmt.Sprintf("# Buku (book)      : %s", emptyAsDash(b.Book)),
		fmt.Sprintf("# Dibuat           : %s", b.GeneratedAt.Format(time.RFC3339)),
		fmt.Sprintf("# Status pemetaan  : %s (lihat coa_mapping.go)", b.MappingStatus),
		fmt.Sprintf("# Kolom            : FORM%sSANDI%sNAMA POS%sJUMLAH (rupiah penuh, tanpa desimal)",
			ColumnSeparator, ColumnSeparator, ColumnSeparator),
		`# Baris form 00.08 diisi dalam persen 2 desimal; "-" berarti tidak tersedia (bukan nol).`,
	}
	for _, line := range metadata {
		if err := write("%s\n", line); err != nil {
			return err
		}
	}

	if err := write("%s\n", strings.Join([]string{"FORM", "SANDI", "NAMA POS", "JUMLAH"}, ColumnSeparator)); err != nil {
		return err
	}

	for _, section := range b.Sections {
		if err := write("# FORM %s - %s\n", section.Form, section.Name); err != nil {
			return err
		}
		for _, line := range section.Lines {
			nilai := FormatRupiah(line.Amount)
			if line.Percent {
				if line.UnavailableReason != "" {
					// "-" menandakan tidak tersedia, bukan nilai nol.
					nilai = "-"
				} else {
					nilai = FormatPersen(line.Amount)
				}
			}
			if err := write("%s\n", strings.Join([]string{
				section.Form, line.Sandi, line.Name, nilai,
			}, ColumnSeparator)); err != nil {
				return err
			}
			if line.UnavailableReason != "" {
				if err := write("# %s|%s TIDAK TERSEDIA: %s\n",
					section.Form, line.Sandi, line.UnavailableReason); err != nil {
					return err
				}
			}
		}
	}

	for _, f := range b.SkippedForms {
		if err := write("# FORM %s TIDAK DIBANGUN: %s\n", f.Form, f.UnavailableReason); err != nil {
			return err
		}
	}
	return nil
}

func emptyAsDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
