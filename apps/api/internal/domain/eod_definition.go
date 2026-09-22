package domain

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Definisi langkah tutup hari (EOD) yang dikelola di database. Urutan langkah
// sebelumnya di-hardcode di service; keputusan pemilik sistem (docs/BACKLOG.md item 7)
// memindahkannya ke tabel agar bank dapat menyesuaikan tanpa rilis kode.
var (
	// ErrEODCoreStepRequired: langkah inti tidak boleh hilang dari definisi.
	ErrEODCoreStepRequired = errors.New("langkah inti EOD wajib ada dan tidak boleh dihapus")
	// ErrEODCoreStepDisabled: langkah inti tidak boleh dinonaktifkan.
	ErrEODCoreStepDisabled = errors.New("langkah inti EOD tidak boleh dinonaktifkan")
	// ErrEODDuplicateSequence: dua langkah memakai urutan yang sama.
	ErrEODDuplicateSequence = errors.New("urutan langkah EOD ganda")
	// ErrEODInvalidSequence: urutan kosong/nol/negatif.
	ErrEODInvalidSequence = errors.New("urutan langkah EOD harus bilangan positif")
	// ErrEODUnknownPrerequisite: prasyarat menunjuk kode langkah yang tidak ada.
	ErrEODUnknownPrerequisite = errors.New("prasyarat langkah EOD menunjuk langkah yang tidak ada")
	// ErrEODPrerequisiteCycle: prasyarat membentuk siklus.
	ErrEODPrerequisiteCycle = errors.New("prasyarat langkah EOD membentuk siklus")
	// ErrEODActiveDependsOnDisabled: langkah aktif bergantung pada langkah nonaktif,
	// sehingga tidak akan pernah berjalan. Ditolak agar tidak menjadi jebakan senyap.
	ErrEODActiveDependsOnDisabled = errors.New("langkah EOD aktif tidak boleh bergantung pada langkah nonaktif")
	// ErrEODDefinitionsUnavailable: definisi tidak dapat dibaca tepat sebelum tutup
	// hari. Tutup hari MENOLAK berjalan, bukan diam-diam melewati langkah.
	ErrEODDefinitionsUnavailable = errors.New("definisi langkah EOD tidak tersedia")
	// ErrEODNoDueTrigger: jalur terjadwal dipanggil tanpa pemicu terjadwal yang jatuh
	// tempo.
	ErrEODNoDueTrigger = errors.New("tidak ada pemicu EOD terjadwal yang jatuh tempo")
)

// EODStepCoreCodes adalah daftar kode langkah inti akuntansi yang WAJIB ada dan aktif
// pada setiap definisi. Daftar ini di kode (bukan hanya kolom core di database) supaya
// penghapusan baris inti — yang menghilangkan penandanya sekaligus — tetap tertangkap
// validasi. Ini pengaman, bukan sumber urutan.
var EODStepCoreCodes = []string{
	"loan_interest_accrual",
	"restructure_loss_amortization",
	"ppap",
	"ckpn_comparison",
}

// EODStepKnownCodes adalah langkah yang benar-benar punya pelaksana di mesin EOD.
// Bukan urutan; hanya daftar kemampuan. Definisi dengan kode di luar daftar ini
// ditolak agar tidak ada langkah yang tercatat "dijalankan" padahal tidak dikenali.
var EODStepKnownCodes = []string{
	"aro",
	"loan_interest_accrual",
	"restructure_loss_amortization",
	"ppap",
	"ckpn_comparison",
	"loan_penalty_accrual",
	"dormant",
}

// EODStepDefinition adalah satu langkah tutup hari beserta urutan, saklar, prasyarat,
// kategori, dan penanda inti.
type EODStepDefinition struct {
	Code          string   `json:"code"`
	Sequence      int      `json:"sequence"`
	Enabled       bool     `json:"enabled"`
	Core          bool     `json:"core"`
	Category      string   `json:"category"`
	Prerequisites []string `json:"prerequisites"`
	Description   string   `json:"description"`
}

// EODStepRunRecord adalah riwayat satu langkah EOD pada satu tanggal bisnis.
type EODStepRunRecord struct {
	RunID        uuid.UUID      `json:"run_id"`
	BusinessDate time.Time      `json:"business_date"`
	StepCode     string         `json:"step_code"`
	Sequence     int            `json:"sequence"`
	Status       EODStepStatus  `json:"status"`
	Trigger      string         `json:"trigger"`
	Reason       string         `json:"reason,omitempty"`
	Summary      map[string]any `json:"summary,omitempty"`
	ExecutedBy   uuid.UUID      `json:"executed_by"`
	StartedAt    time.Time      `json:"started_at"`
	CompletedAt  time.Time      `json:"completed_at"`
}

// EODTrigger adalah definisi pemicu tutup hari: manual (dipakai sekarang) atau
// terjadwal (belum ada penjadwalnya).
type EODTrigger struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	TriggerType  string     `json:"trigger_type"`
	ScheduleCron string     `json:"schedule_cron,omitempty"`
	Enabled      bool       `json:"enabled"`
	Description  string     `json:"description"`
	LastRunAt    *time.Time `json:"last_run_at,omitempty"`
	NextRunAt    *time.Time `json:"next_run_at,omitempty"`
}

// EODStepRepository membaca definisi, menyimpan perubahan definisi, dan menulis riwayat
// hasil per langkah. Satu tipe di produksi (postgres) memenuhi seluruhnya; service
// batch memakai subset baca + tulis riwayat, service definisi memakai baca + simpan.
type EODStepRepository interface {
	ListDefinitions(ctx context.Context) ([]EODStepDefinition, error)
	// SaveDefinitions menyimpan perubahan urutan/saklar/prasyarat untuk kode yang SUDAH
	// ada. Kolom core TIDAK ikut diubah (imutabel lewat API) dan tidak ada operasi
	// hapus: langkah inti tidak dapat dihilangkan maupun dilucuti penandanya.
	SaveDefinitions(ctx context.Context, tx any, defs []EODStepDefinition, updatedBy uuid.UUID) error
	RecordStepResult(ctx context.Context, rec EODStepRunRecord) error
	ListStepRuns(ctx context.Context, businessDate time.Time) ([]EODStepRunRecord, error)
	ListDueScheduledTriggers(ctx context.Context, now time.Time) ([]EODTrigger, error)
	MarkTriggerRun(ctx context.Context, name string, ranAt time.Time, nextRunAt *time.Time) error
}

// EODDefinitionService mengelola definisi langkah EOD dan membaca riwayatnya.
type EODDefinitionService interface {
	ListDefinitions(ctx context.Context) ([]EODStepDefinition, error)
	// UpdateDefinitions memvalidasi lalu menyimpan SELURUH definisi dalam satu
	// transaksi beserta audit. Perubahan hanya boleh datang dari izin system:config.
	UpdateDefinitions(ctx context.Context, defs []EODStepDefinition, actor Actor) ([]EODStepDefinition, error)
	ListStepRuns(ctx context.Context, businessDate time.Time) ([]EODStepRunRecord, error)
}

// ValidateEODStepDefinitions menegakkan seluruh pengaman konfigurasi urutan. Dipanggil
// sebelum definisi dipakai untuk menjalankan tutup hari DAN sebelum disimpan, sehingga
// konfigurasi yang merusak akuntansi tidak pernah sampai ke database.
func ValidateEODStepDefinitions(defs []EODStepDefinition) error {
	if len(defs) == 0 {
		return fmt.Errorf("%w: definisi kosong", ErrEODDefinitionsUnavailable)
	}

	byCode := make(map[string]EODStepDefinition, len(defs))
	seqSeen := make(map[int]string, len(defs))
	for _, def := range defs {
		code := def.Code
		if code == "" {
			return fmt.Errorf("%w: ada langkah tanpa kode", ErrEODDefinitionsUnavailable)
		}
		if _, dup := byCode[code]; dup {
			return fmt.Errorf("kode langkah EOD %q muncul lebih dari sekali", code)
		}
		if def.Sequence <= 0 {
			return fmt.Errorf("%w: langkah %s bernilai %d", ErrEODInvalidSequence, code, def.Sequence)
		}
		if other, dup := seqSeen[def.Sequence]; dup {
			return fmt.Errorf("%w: langkah %s dan %s memakai urutan %d", ErrEODDuplicateSequence, other, code, def.Sequence)
		}
		seqSeen[def.Sequence] = code
		byCode[code] = def
	}

	// Kode harus dikenal mesin; langkah tanpa pelaksana tidak boleh masuk definisi.
	known := make(map[string]struct{}, len(EODStepKnownCodes))
	for _, c := range EODStepKnownCodes {
		known[c] = struct{}{}
	}
	for _, def := range defs {
		if _, ok := known[def.Code]; !ok {
			return fmt.Errorf("%w: kode %q tidak dikenal mesin EOD", ErrEODDefinitionsUnavailable, def.Code)
		}
	}

	// Prasyarat harus ada, bukan diri sendiri, dan (bila langkah aktif) harus aktif.
	for _, def := range defs {
		for _, prereq := range def.Prerequisites {
			if prereq == def.Code {
				return fmt.Errorf("langkah EOD %s menjadikan dirinya sendiri prasyarat", def.Code)
			}
			dep, ok := byCode[prereq]
			if !ok {
				return fmt.Errorf("%w: langkah %s memerlukan %s", ErrEODUnknownPrerequisite, def.Code, prereq)
			}
			if def.Enabled && !dep.Enabled {
				return fmt.Errorf("%w: langkah aktif %s memerlukan %s yang nonaktif", ErrEODActiveDependsOnDisabled, def.Code, prereq)
			}
		}
	}

	if err := detectEODPrerequisiteCycle(byCode); err != nil {
		return err
	}

	// Langkah inti wajib ada, aktif, dan ditandai inti.
	for _, code := range EODStepCoreCodes {
		def, ok := byCode[code]
		if !ok {
			return fmt.Errorf("%w: %s", ErrEODCoreStepRequired, code)
		}
		if !def.Enabled {
			return fmt.Errorf("%w: %s", ErrEODCoreStepDisabled, code)
		}
		if !def.Core {
			return fmt.Errorf("%w: %s harus ditandai core", ErrEODCoreStepRequired, code)
		}
	}

	return nil
}

// detectEODPrerequisiteCycle menjalankan DFS tiga warna atas graf prasyarat.
func detectEODPrerequisiteCycle(byCode map[string]EODStepDefinition) error {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(byCode))
	var visit func(code string) error
	visit = func(code string) error {
		color[code] = gray
		for _, prereq := range byCode[code].Prerequisites {
			switch color[prereq] {
			case gray:
				return fmt.Errorf("%w: %s -> %s", ErrEODPrerequisiteCycle, code, prereq)
			case white:
				if err := visit(prereq); err != nil {
					return err
				}
			}
		}
		color[code] = black
		return nil
	}
	for code := range byCode {
		if color[code] == white {
			if err := visit(code); err != nil {
				return err
			}
		}
	}
	return nil
}

// SortEODStepDefinitions menyalin dan mengurutkan definisi menaik berdasarkan sequence.
// Salinan dibuat agar pemanggil tidak mengubah irisan yang dipegang cache.
func SortEODStepDefinitions(defs []EODStepDefinition) []EODStepDefinition {
	out := make([]EODStepDefinition, len(defs))
	copy(out, defs)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Sequence < out[j].Sequence })
	return out
}

// EODStepDefinitionByName mengembalikan salinan definisi untuk satu kode.
func EODStepDefinitionByName(defs []EODStepDefinition, code string) (EODStepDefinition, bool) {
	for _, def := range defs {
		if def.Code == code {
			return def, true
		}
	}
	return EODStepDefinition{}, false
}
