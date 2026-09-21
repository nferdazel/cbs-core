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
			if err := write("%s\n", strings.Join([]string{
				section.Form, line.Sandi, line.Name, FormatRupiah(line.Amount),
			}, ColumnSeparator)); err != nil {
				return err
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
