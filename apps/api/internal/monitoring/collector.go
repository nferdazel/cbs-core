package monitoring

import (
	"context"
	"errors"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// BusinessDateReader membaca tanggal bisnis berjalan. Dipenuhi oleh
// *postgres.BusinessDateRepository.
type BusinessDateReader interface {
	GetCurrentDate(ctx context.Context) (*domain.SystemBusinessDate, error)
}

// EODRunReader membaca riwayat langkah EOD pada satu tanggal bisnis. Tanggal nol
// berarti tanpa filter. Dipenuhi oleh *postgres.EODStepRepository.
type EODRunReader interface {
	ListStepRuns(ctx context.Context, businessDate time.Time) ([]domain.EODStepRunRecord, error)
}

// Deps adalah dependensi Collector. Semuanya opsional: yang nil dilaporkan sebagai
// temuan "tidak dapat dibaca/dikonfigurasi", bukan membuat endpoint gagal.
type Deps struct {
	Dates    BusinessDateReader
	EOD      EODRunReader
	PPAP     domain.PPAPRunMarker
	Activity domain.OperationalActivityReader
	Config   domain.SystemConfigService
	// Now dapat diganti pada uji agar hasil deterministik. Nil berarti time.Now.
	Now func() time.Time
}

// Collector mengambil potret data operasional lalu menjalankan evaluator murni.
// Ia hanya membaca: tidak menulis konfigurasi, tidak menjalankan tutup hari, dan
// tidak mengubah angka apa pun.
type Collector struct {
	deps Deps
}

// NewCollector membuat Collector. Deps.Now default time.Now.
func NewCollector(deps Deps) *Collector {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &Collector{deps: deps}
}

// Collect membaca data dari repositori dan mengembalikan Snapshot. Kegagalan membaca
// menjadi temuan, sehingga endpoint selalu dapat membalas 200 dengan gambaran apa
// adanya alih-alih gagal total saat satu sumber bermasalah.
func (c *Collector) Collect(ctx context.Context) Snapshot {
	now := c.deps.Now()
	data := Data{Now: now}

	if c.deps.Dates != nil {
		bd, err := c.deps.Dates.GetCurrentDate(ctx)
		data.BusinessDate, data.BusinessDateErr = bd, err
	} else {
		data.BusinessDateErr = errors.New("pembaca tanggal bisnis tidak dikonfigurasi")
	}

	// Riwayat langkah dibaca untuk tanggal bisnis berjalan saja. Tutup hari yang gagal
	// tidak memajukan tanggal, sehingga langkah FAILED selalu berada pada tanggal ini.
	switch {
	case c.deps.EOD == nil:
		data.EODStepsErr = errors.New("pembaca riwayat EOD tidak dikonfigurasi")
	case data.BusinessDate != nil:
		steps, err := c.deps.EOD.ListStepRuns(ctx, data.BusinessDate.CurrentDate)
		data.EODSteps, data.EODStepsErr = steps, err
	}

	if c.deps.PPAP != nil {
		last, known, err := c.deps.PPAP.LastRunBusinessDate(ctx)
		data.PPAPLastDate, data.PPAPKnown, data.PPAPErr = last, known, err
	}

	if c.deps.Activity != nil {
		operational, err := c.deps.Activity.HasOperationalActivity(ctx)
		if err != nil {
			data.OperationalErr = err
		} else {
			data.Operational = &operational
		}
	}

	if c.deps.Config != nil {
		data.CKPN = domain.CKPNParametersStatusFromConfig(ctx, c.deps.Config, now)
		data.CKPNEnabled = c.deps.Config.GetBool(ctx, domain.ConfigKeyCKPNEnabled, false)
	}

	findings := Evaluate(data)
	if findings == nil {
		findings = []Finding{}
	}
	return Snapshot{
		EvaluatedAt: now.UTC(),
		Findings:    findings,
		Summary:     summarize(findings),
	}
}

// summarize mencacah temuan per tingkat.
func summarize(findings []Finding) Summary {
	var s Summary
	for _, f := range findings {
		switch f.Severity {
		case SeverityCritical:
			s.Critical++
		case SeverityWarning:
			s.Warning++
		}
	}
	return s
}
