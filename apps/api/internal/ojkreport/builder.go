package ojkreport

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Source adalah sumber angka yang sudah ada. Interface sengaja sempit: modul
// hanya bergantung pada dua laporan journal-based yang sudah dihitung layanan
// laporan, sehingga tidak ada rumus akuntansi yang dihitung ulang di sini.
type Source interface {
	GetBalanceSheet(ctx context.Context, asOf time.Time, book string) (*domain.BalanceSheet, error)
	GetIncomeStatement(ctx context.Context, from, to time.Time, book string) (*domain.IncomeStatement, error)
}

var (
	// ErrIncompleteMapping menandai pemetaan COA yang tidak lengkap. Ekspor harus
	// gagal, bukan menghasilkan laporan bolong tanpa disadari.
	ErrIncompleteMapping = errors.New("pemetaan COA ke pos laporan OJK tidak lengkap")
	// ErrInvalidMapping menandai berkas pemetaan yang tidak sah (mis. kode ganda).
	ErrInvalidMapping = errors.New("pemetaan COA ke pos laporan OJK tidak sah")
	// ErrUnbalancedSource menandai laporan sumber yang masih punya baris penyeimbang,
	// artinya jurnal tidak tie-out dan angka tidak layak dikirim ke OJK.
	ErrUnbalancedSource = errors.New("laporan sumber belum seimbang; ekspor OJK dibatalkan")
)

// IncompleteMappingError merinci kode COA yang belum dipetakan per form.
type IncompleteMappingError struct {
	// Missing memetakan kode form -> kode COA yang belum dipetakan.
	Missing map[string][]string
}

func (e *IncompleteMappingError) Error() string {
	forms := make([]string, 0, len(e.Missing))
	for f := range e.Missing {
		forms = append(forms, f)
	}
	sort.Strings(forms)
	var b strings.Builder
	b.WriteString(ErrIncompleteMapping.Error())
	for _, f := range forms {
		codes := append([]string(nil), e.Missing[f]...)
		sort.Strings(codes)
		fmt.Fprintf(&b, "; form %s: %s", f, strings.Join(codes, ", "))
	}
	return b.String()
}

func (e *IncompleteMappingError) Unwrap() error { return ErrIncompleteMapping }

// Line adalah satu baris form yang siap ditulis. Amount dalam rupiah penuh.
type Line struct {
	Sandi  string
	Name   string
	Level  int
	Amount decimal.Decimal
}

// Section adalah satu form lengkap beserta baris-barisnya.
type Section struct {
	Form  string
	Name  string
	Lines []Line
}

// Bundle adalah hasil ekspor laporan bulanan yang siap diperiksa manusia.
type Bundle struct {
	Period             time.Time
	PeriodEnd          time.Time
	Book               string
	GeneratedAt        time.Time
	Deadline           time.Time
	CorrectionDeadline time.Time
	MappingStatus      string
	Sections           []Section
	// SkippedForms adalah form yang belum dapat dibangun beserta alasannya.
	SkippedForms []OJKFormDefinition
}

// Builder menyusun laporan bulanan dari sumber angka yang ada dan pemetaan COA.
type Builder struct {
	source  Source
	mapping []MappingEntry
	now     func() time.Time
}

// NewBuilder membuat builder dengan pemetaan bawaan (COAMappingDraft).
func NewBuilder(source Source) *Builder {
	return NewBuilderWithMapping(source, COAMappingDraft)
}

// NewBuilderWithMapping membuat builder dengan pemetaan yang disuntikkan. Dipakai
// pengujian untuk memastikan pemetaan tidak lengkap ditolak dengan jelas.
func NewBuilderWithMapping(source Source, mapping []MappingEntry) *Builder {
	return &Builder{source: source, mapping: mapping, now: time.Now}
}

// MonthEnd mengembalikan hari terakhir bulan dari tanggal mana pun (UTC, awal hari).
func MonthEnd(t time.Time) time.Time {
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	return first.AddDate(0, 1, -1)
}

// GenerateMonthly menyusun form 01.00 dan 02.00 untuk periode bulan tertentu.
// Laporan laba rugi disajikan year-to-date (1 Januari s/d akhir periode), sesuai
// praktik laporan berkala OJK.
func (b *Builder) GenerateMonthly(ctx context.Context, period time.Time, book string) (*Bundle, error) {
	if b.source == nil {
		return nil, errors.New("sumber laporan OJK belum dikonfigurasi")
	}
	if dup := DuplicateCOACodes(b.mapping); len(dup) > 0 {
		return nil, fmt.Errorf("%w: kode COA ganda: %s", ErrInvalidMapping, strings.Join(dup, ", "))
	}

	periodStart := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := MonthEnd(period)
	yearStart := time.Date(period.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)

	bs, err := b.source.GetBalanceSheet(ctx, periodEnd, book)
	if err != nil {
		return nil, err
	}
	is, err := b.source.GetIncomeStatement(ctx, yearStart, periodEnd, book)
	if err != nil {
		return nil, err
	}

	index := buildMappingIndex(b.mapping)
	amounts01, gaps01, err := collect(bs.Rows, "01.00", index)
	if err != nil {
		return nil, err
	}
	amounts02, gaps02, err := collect(is.Rows, "02.00", index)
	if err != nil {
		return nil, err
	}

	if missing := mergeGaps("01.00", gaps01, "02.00", gaps02); len(missing) > 0 {
		return nil, &IncompleteMappingError{Missing: missing}
	}

	deriveTotals(form01Lines, amounts01)
	deriveTotals(form02Lines, amounts02)

	def := reportByCode("LAPORAN_BULANAN_BPR")
	return &Bundle{
		Period:             periodStart,
		PeriodEnd:          periodEnd,
		Book:               book,
		GeneratedAt:        b.now().UTC(),
		Deadline:           def.Deadline(periodStart),
		CorrectionDeadline: def.CorrectionDeadline(periodStart),
		MappingStatus:      MappingStatus,
		Sections: []Section{
			{Form: "01.00", Name: formName("01.00"), Lines: renderLines(form01Lines, amounts01)},
			{Form: "02.00", Name: formName("02.00"), Lines: renderLines(form02Lines, amounts02)},
		},
		SkippedForms: skippedForms(),
	}, nil
}

// collect mengubah baris laporan sumber menjadi jumlah per sandi OJK. Kode COA
// yang belum dipetakan dikumpulkan agar ekspor dapat ditolak dengan jelas.
func collect(rows []domain.ReportRow, form string, index map[string]MappingEntry) (map[string]decimal.Decimal, []string, error) {
	amounts := make(map[string]decimal.Decimal)
	var gaps []string
	for _, row := range rows {
		// "00000" adalah baris penyeimbang dari reporting repo: jurnal tidak tie-out.
		if row.AccountCode == "00000" {
			return nil, nil, fmt.Errorf("%w: ditemukan baris penyeimbang selisih jurnal", ErrUnbalancedSource)
		}
		entry, ok := index[row.AccountCode]
		if !ok {
			gaps = append(gaps, row.AccountCode)
			continue
		}
		if entry.Form != form {
			// Akun milik form lain (mis. pendapatan pada neraca) memang dilewati.
			continue
		}
		sign := entry.Sign
		if sign == 0 {
			sign = 1
		}
		contribution := row.Amount
		if sign < 0 {
			contribution = contribution.Neg()
		}
		amounts[entry.Sandi] = amounts[entry.Sandi].Add(contribution)
	}
	return amounts, gaps, nil
}

// deriveTotals mengisi baris total dengan menjumlahkan pos anak berbobot.
func deriveTotals(lines []formLine, amounts map[string]decimal.Decimal) {
	for _, line := range lines {
		if len(line.TotalFrom) == 0 {
			continue
		}
		total := decimal.Zero
		for _, part := range line.TotalFrom {
			v := amounts[part.Sandi]
			if part.Weight < 0 {
				total = total.Sub(v)
			} else {
				total = total.Add(v)
			}
		}
		amounts[line.Sandi] = total
	}
}

// renderLines menulis setiap baris form terurut; pos tanpa nilai tetap ditulis 0.
func renderLines(lines []formLine, amounts map[string]decimal.Decimal) []Line {
	out := make([]Line, 0, len(lines))
	for _, l := range lines {
		out = append(out, Line{
			Sandi:  l.Sandi,
			Name:   l.Name,
			Level:  l.Level,
			Amount: amounts[l.Sandi],
		})
	}
	return out
}

// mergeGaps menggabungkan kode yang belum dipetakan per form.
func mergeGaps(formA string, gapsA []string, formB string, gapsB []string) map[string][]string {
	missing := make(map[string][]string)
	if len(gapsA) > 0 {
		missing[formA] = gapsA
	}
	if len(gapsB) > 0 {
		missing[formB] = gapsB
	}
	return missing
}

func reportByCode(code string) OJKReportDefinition {
	for _, d := range OJKReportDefinitions {
		if d.Code == code {
			return d
		}
	}
	return OJKReportDefinition{DueDay: 10, CorrectionDay: 15}
}

func formName(form string) string {
	for _, f := range OJKBulananForms {
		if f.Form == form {
			return f.Name
		}
	}
	return form
}

func skippedForms() []OJKFormDefinition {
	out := make([]OJKFormDefinition, 0, len(OJKBulananForms))
	for _, f := range OJKBulananForms {
		if !f.Buildable {
			out = append(out, f)
		}
	}
	return out
}
