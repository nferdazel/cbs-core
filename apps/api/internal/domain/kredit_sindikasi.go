package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// kredit_sindikasi.go memuat fondasi data Form 06.02 "DAFTAR KREDIT SINDIKASI",
// Lampiran II SEOJK No. 16/SEOJK.03/2024.
//
// Sumber struktur form (PDF resmi, di luar repo):
//   - Form 06.02 "DAFTAR KREDIT SINDIKASI": PDF #page 173 (hlm. tercetak 121) kolom I-VII,
//     PDF #page 174 (hlm. 122) kolom VIII-XIII. Tiga belas kolom: I Sandi Kantor,
//     II ID Pihak Lawan, III No. Identitas, IV No. Rekening,
//     V Jumlah Pendanaan Sindikasi, VI Bagian Pendanaan,
//     VII Sandi Bank Peserta Sindikasi (Plafon + Baki Debet),
//     VIII Status Kepesertaan, IX Nomor Perjanjian Kredit Sindikasi,
//     X Pendanaan di Bank Pelapor, XI Kualitas,
//     XII Nominal Tunggakan (Pokok + Bunga), XIII Hari Tunggakan (Pokok + Bunga).
//     Satu baris = satu rekening fasilitas kredit sindikasi; TIDAK ada baris JUMLAH.
//   - Sandi: PDF #page 175-176 (hlm. 123-124). VIII 1 Arranger / 2 Anggota sindikasi;
//     X 1 Ya / 2 Tidak; XI 1 Lancar / 2 Dalam Perhatian Khusus / 3 Kurang Lancar /
//     4 Diragukan / 5 Macet.
//   - Penjelasan: PDF #page 177-178 (hlm. 125-126). IV unik dan "tidak boleh sama" serta
//     harus sama dengan nomor rekening SLIK, dan DIKOSONGKAN bila X = sandi 2. V adalah
//     jumlah pendanaan seluruh anggota; VI bagian BPR pelapor. IX nomor perjanjian induk
//     awal tanpa spasi. XIII paling singkat 0.
//
// Prinsip yang dipegang berkas ini:
//   - Semua nilai adalah ISIAN BANK; form tidak memberi rumus pengikat, jadi laporan
//     tidak menurunkan angka apa pun.
//   - Kolom III No. Identitas (NIK/NPWP debitur) SENGAJA TIDAK DISIMPAN mengikuti keputusan
//     privasi (docs/KEPUTUSAN-OJK.md §4 dan §7 butir 6, pola yang sama dengan Form 00.01
//     dan form berhenti). Laporan menuliskan "-" beserta alasannya.
//   - Kolom IV unik; penghapusan adalah soft-delete agar nomor rekening tidak dapat dipakai
//     ulang. Bila X = sandi 2, kolom IV WAJIB kosong; bila X = sandi 1, kolom IV WAJIB ada.
//   - Kolom I "Sandi Kantor" tidak disimpan: diambil dari kantor pelapor tunggal
//     bank_offices (migrasi 000112).

// Sandi baku Form 06.02 (PDF #page 175-176).
const (
	SindikasiKepesertaanArranger = "1"
	SindikasiKepesertaanAnggota  = "2"

	SindikasiPendanaanYa    = "1"
	SindikasiPendanaanTidak = "2"

	SindikasiKualitasLancar               = "1"
	SindikasiKualitasDalamPerhatianKhusus = "2"
	SindikasiKualitasKurangLancar         = "3"
	SindikasiKualitasDiragukan            = "4"
	SindikasiKualitasMacet                = "5"
)

// Status baku register kredit sindikasi.
const (
	SindikasiStatusAktif    = "AKTIF"
	SindikasiStatusNonaktif = "NONAKTIF"
)

// SindikasiNomorRekeningMax adalah batas panjang nomor rekening/sandi yang disimpan.
const SindikasiNomorRekeningMax = 64

var (
	// ErrSindikasiBankWide menandai permintaan register kredit sindikasi oleh aktor yang
	// tidak berwenang atas seluruh bank. Register ini bank-wide.
	ErrSindikasiBankWide = NewLocalizedError("kredit_sindikasi_bank_wide",
		"register kredit sindikasi bersifat bank-wide dan hanya dapat dibaca peran lintas cabang")
	// ErrSindikasiInputInvalid menandai data register kredit sindikasi tidak sah.
	ErrSindikasiInputInvalid = NewLocalizedError("kredit_sindikasi_input_invalid",
		"data register kredit sindikasi tidak valid")
	// ErrSindikasiNotFound menandai baris yang hendak dinonaktifkan tidak ada.
	ErrSindikasiNotFound = NewLocalizedError("kredit_sindikasi_not_found",
		"data register kredit sindikasi tidak ditemukan")
	// ErrSindikasiNoRekeningUsed menandai No. Rekening yang sudah pernah dipakai,
	// termasuk oleh baris NONAKTIF. Aturan "nomor rekening unik, tidak boleh sama"
	// Form 06.02 (PDF #page 177).
	ErrSindikasiNoRekeningUsed = NewLocalizedError("kredit_sindikasi_no_rekening_used",
		"nomor rekening sindikasi sudah pernah dipakai dan tidak boleh dipakai ulang")
)

// SindikasiItem adalah satu rekening fasilitas kredit sindikasi (satu baris Form 06.02).
// Tidak ada kolom turunan: seluruh nilai adalah isian bank. Kolom III No. Identitas tidak
// ada di sini karena sengaja tidak disimpan (keputusan privasi).
type SindikasiItem struct {
	ID uuid.UUID `json:"id"`
	// CounterpartyID kolom II: pengenal pihak lawan/debitur, teks apa adanya.
	CounterpartyID string `json:"counterparty_id"`
	// NoRekening kolom IV: unik bila terisi; kosong bila pendanaan bukan di bank pelapor.
	NoRekening string `json:"no_rekening,omitempty"`
	// JumlahPendanaanSindikasi kolom V: seluruh anggota sindikasi, rupiah penuh.
	JumlahPendanaanSindikasi decimal.Decimal `json:"jumlah_pendanaan_sindikasi"`
	// BagianPendanaan kolom VI: bagian BPR pelapor, rupiah penuh.
	BagianPendanaan decimal.Decimal `json:"bagian_pendanaan"`
	// SandiBankPeserta kolom VII: sandi bank peserta mengacu SPOJK, teks apa adanya.
	SandiBankPeserta string `json:"sandi_bank_peserta"`
	// Plafon sub-kolom VII, rupiah penuh; isian bank.
	Plafon decimal.Decimal `json:"plafon"`
	// BakiDebet sub-kolom VII, rupiah penuh; isian bank.
	BakiDebet decimal.Decimal `json:"baki_debet"`
	// StatusKepesertaanCode kolom VIII: 1 Arranger, 2 Anggota sindikasi.
	StatusKepesertaanCode string `json:"status_kepesertaan_code"`
	// NomorPerjanjianInduk kolom IX: nomor perjanjian induk awal tanpa spasi.
	NomorPerjanjianInduk string `json:"nomor_perjanjian_induk"`
	// PendanaanDiBankPelaporCode kolom X: 1 Ya, 2 Tidak.
	PendanaanDiBankPelaporCode string `json:"pendanaan_di_bank_pelapor_code"`
	// KualitasCode kolom XI: 1-5.
	KualitasCode string `json:"kualitas_code"`
	// TunggakanPokok kolom XII, rupiah penuh; isian bank.
	TunggakanPokok decimal.Decimal `json:"tunggakan_pokok"`
	// TunggakanBunga kolom XII, rupiah penuh; isian bank.
	TunggakanBunga decimal.Decimal `json:"tunggakan_bunga"`
	// HariTunggakanPokok kolom XIII, jumlah hari, paling singkat 0.
	HariTunggakanPokok int `json:"hari_tunggakan_pokok"`
	// HariTunggakanBunga kolom XIII, jumlah hari, paling singkat 0.
	HariTunggakanBunga int       `json:"hari_tunggakan_bunga"`
	AsOf               time.Time `json:"as_of"`
	Status             string    `json:"status"`
	Note               string    `json:"note,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// UpdateSindikasiItemInput adalah isi upsert satu baris kredit sindikasi. ID kosong =
// buat baru; ID terisi = perbarui baris itu. Tanggal memakai format YYYY-MM-DD.
type UpdateSindikasiItemInput struct {
	ID                         string          `json:"id"`
	CounterpartyID             string          `json:"counterparty_id"`
	NoRekening                 string          `json:"no_rekening"`
	JumlahPendanaanSindikasi   decimal.Decimal `json:"jumlah_pendanaan_sindikasi"`
	BagianPendanaan            decimal.Decimal `json:"bagian_pendanaan"`
	SandiBankPeserta           string          `json:"sandi_bank_peserta"`
	Plafon                     decimal.Decimal `json:"plafon"`
	BakiDebet                  decimal.Decimal `json:"baki_debet"`
	StatusKepesertaanCode      string          `json:"status_kepesertaan_code"`
	NomorPerjanjianInduk       string          `json:"nomor_perjanjian_induk"`
	PendanaanDiBankPelaporCode string          `json:"pendanaan_di_bank_pelapor_code"`
	KualitasCode               string          `json:"kualitas_code"`
	TunggakanPokok             decimal.Decimal `json:"tunggakan_pokok"`
	TunggakanBunga             decimal.Decimal `json:"tunggakan_bunga"`
	HariTunggakanPokok         int             `json:"hari_tunggakan_pokok"`
	HariTunggakanBunga         int             `json:"hari_tunggakan_bunga"`
	AsOf                       string          `json:"as_of"`
	Status                     string          `json:"status"`
	Note                       string          `json:"note"`
}

// BuildSindikasiItem memvalidasi masukan dan membentuk baris yang siap disimpan.
// Perhatian khusus: kolom IV No. Rekening WAJIB kosong bila kolom X = sandi 2 (tidak),
// dan WAJIB terisi bila kolom X = sandi 1 (ya) — keduanya dinyatakan PDF #page 177.
func BuildSindikasiItem(in UpdateSindikasiItemInput) (SindikasiItem, error) {
	out := SindikasiItem{
		CounterpartyID:             strings.TrimSpace(in.CounterpartyID),
		NoRekening:                 strings.TrimSpace(in.NoRekening),
		JumlahPendanaanSindikasi:   in.JumlahPendanaanSindikasi,
		BagianPendanaan:            in.BagianPendanaan,
		SandiBankPeserta:           strings.TrimSpace(in.SandiBankPeserta),
		Plafon:                     in.Plafon,
		BakiDebet:                  in.BakiDebet,
		StatusKepesertaanCode:      strings.TrimSpace(in.StatusKepesertaanCode),
		NomorPerjanjianInduk:       strings.TrimSpace(in.NomorPerjanjianInduk),
		PendanaanDiBankPelaporCode: strings.TrimSpace(in.PendanaanDiBankPelaporCode),
		KualitasCode:               strings.TrimSpace(in.KualitasCode),
		TunggakanPokok:             in.TunggakanPokok,
		TunggakanBunga:             in.TunggakanBunga,
		HariTunggakanPokok:         in.HariTunggakanPokok,
		HariTunggakanBunga:         in.HariTunggakanBunga,
		Status:                     strings.ToUpper(strings.TrimSpace(in.Status)),
		Note:                       strings.TrimSpace(in.Note),
	}
	if out.CounterpartyID == "" {
		return out, fmt.Errorf("%w: ID Pihak Lawan wajib diisi", ErrSindikasiInputInvalid)
	}
	if len(out.CounterpartyID) > SindikasiNomorRekeningMax {
		return out, fmt.Errorf("%w: ID Pihak Lawan maksimal %d karakter",
			ErrSindikasiInputInvalid, SindikasiNomorRekeningMax)
	}
	switch out.StatusKepesertaanCode {
	case SindikasiKepesertaanArranger, SindikasiKepesertaanAnggota:
	default:
		return out, fmt.Errorf("%w: status kepesertaan hanya 1 atau 2", ErrSindikasiInputInvalid)
	}
	switch out.PendanaanDiBankPelaporCode {
	case SindikasiPendanaanYa, SindikasiPendanaanTidak:
	default:
		return out, fmt.Errorf("%w: pendanaan di bank pelapor hanya 1 atau 2", ErrSindikasiInputInvalid)
	}
	switch out.KualitasCode {
	case SindikasiKualitasLancar, SindikasiKualitasDalamPerhatianKhusus,
		SindikasiKualitasKurangLancar, SindikasiKualitasDiragukan, SindikasiKualitasMacet:
	default:
		return out, fmt.Errorf("%w: kualitas hanya 1 sampai 5", ErrSindikasiInputInvalid)
	}
	if out.NomorPerjanjianInduk == "" {
		return out, fmt.Errorf("%w: Nomor Perjanjian Kredit Sindikasi wajib diisi", ErrSindikasiInputInvalid)
	}
	if len(out.NomorPerjanjianInduk) > SindikasiNomorRekeningMax {
		return out, fmt.Errorf("%w: Nomor Perjanjian maksimal %d karakter",
			ErrSindikasiInputInvalid, SindikasiNomorRekeningMax)
	}
	if len(out.SandiBankPeserta) > 16 {
		return out, fmt.Errorf("%w: sandi bank peserta maksimal 16 karakter", ErrSindikasiInputInvalid)
	}
	// Kolom IV dan X saling mengikat (PDF #page 177).
	if out.PendanaanDiBankPelaporCode == SindikasiPendanaanTidak && out.NoRekening != "" {
		return out, fmt.Errorf("%w: No. Rekening harus dikosongkan bila pendanaan di bank pelapor = 2 (tidak)",
			ErrSindikasiInputInvalid)
	}
	if out.PendanaanDiBankPelaporCode == SindikasiPendanaanYa && out.NoRekening == "" {
		return out, fmt.Errorf("%w: No. Rekening wajib diisi bila pendanaan di bank pelapor = 1 (ya)",
			ErrSindikasiInputInvalid)
	}
	if len(out.NoRekening) > SindikasiNomorRekeningMax {
		return out, fmt.Errorf("%w: No. Rekening maksimal %d karakter",
			ErrSindikasiInputInvalid, SindikasiNomorRekeningMax)
	}
	if out.JumlahPendanaanSindikasi.IsNegative() {
		return out, fmt.Errorf("%w: jumlah pendanaan sindikasi tidak boleh negatif", ErrSindikasiInputInvalid)
	}
	if out.BagianPendanaan.IsNegative() {
		return out, fmt.Errorf("%w: bagian pendanaan tidak boleh negatif", ErrSindikasiInputInvalid)
	}
	if out.Plafon.IsNegative() {
		return out, fmt.Errorf("%w: plafon tidak boleh negatif", ErrSindikasiInputInvalid)
	}
	if out.BakiDebet.IsNegative() {
		return out, fmt.Errorf("%w: baki debet tidak boleh negatif", ErrSindikasiInputInvalid)
	}
	if out.TunggakanPokok.IsNegative() {
		return out, fmt.Errorf("%w: tunggakan pokok tidak boleh negatif", ErrSindikasiInputInvalid)
	}
	if out.TunggakanBunga.IsNegative() {
		return out, fmt.Errorf("%w: tunggakan bunga tidak boleh negatif", ErrSindikasiInputInvalid)
	}
	if out.HariTunggakanPokok < 0 {
		return out, fmt.Errorf("%w: hari tunggakan pokok paling singkat 0", ErrSindikasiInputInvalid)
	}
	if out.HariTunggakanBunga < 0 {
		return out, fmt.Errorf("%w: hari tunggakan bunga paling singkat 0", ErrSindikasiInputInvalid)
	}
	if out.Status == "" {
		out.Status = SindikasiStatusAktif
	}
	if out.Status != SindikasiStatusAktif && out.Status != SindikasiStatusNonaktif {
		return out, fmt.Errorf("%w: status hanya AKTIF atau NONAKTIF", ErrSindikasiInputInvalid)
	}
	var err error
	if out.ID, err = parseSindikasiID(in.ID); err != nil {
		return out, err
	}
	if out.AsOf, err = parseSindikasiDate(in.AsOf, "as_of"); err != nil {
		return out, err
	}
	return out, nil
}

// parseSindikasiID mengurai id yang dikirim klien; kosong berarti buat baru.
func parseSindikasiID(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.New(), nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: id bukan UUID yang sah", ErrSindikasiInputInvalid)
	}
	return id, nil
}

// parseSindikasiDate mewajibkan tanggal berformat YYYY-MM-DD dan menyebut nama kolomnya.
func parseSindikasiDate(raw, field string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("%w: %s wajib diisi (YYYY-MM-DD)", ErrSindikasiInputInvalid, field)
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s harus format YYYY-MM-DD", ErrSindikasiInputInvalid, field)
	}
	return t, nil
}

// SindikasiRegisterRepository menyimpan dan membaca register kredit sindikasi. Seluruh
// pembacaan bank-wide; penulisan menerima transaksi dari layanan agar audit berada pada
// transaksi yang sama.
type SindikasiRegisterRepository interface {
	ListItems(ctx context.Context) ([]SindikasiItem, error)
	// ListSindikasiForOJK membaca baris AKTIF yang as_of-nya berada pada bulan asOf,
	// urutan deterministik. Sumber Form 06.02.
	ListSindikasiForOJK(ctx context.Context, asOf time.Time) ([]SindikasiItem, error)
	// UpsertItemTx menyimpan (INSERT ... ON CONFLICT id DO UPDATE) satu baris. No. Rekening
	// yang sudah dipakai baris lain (termasuk NONAKTIF) ditolak ErrSindikasiNoRekeningUsed.
	UpsertItemTx(ctx context.Context, tx any, item SindikasiItem, actorID uuid.UUID) error
	// SoftDeleteItemTx menonaktifkan satu baris (status -> NONAKTIF); nomor rekening tetap
	// terpakai. found=false bila baris tidak ada.
	SoftDeleteItemTx(ctx context.Context, tx any, id uuid.UUID, actorID uuid.UUID) (bool, error)
}

// SindikasiReport adalah keluaran baca-saja register kredit sindikasi untuk satu posisi.
type SindikasiReport struct {
	AsOf  time.Time       `json:"as_of"`
	Items []SindikasiItem `json:"items"`
}

// SindikasiRegisterService merakit laporan register kredit sindikasi dan melayani
// pengisian berizin.
type SindikasiRegisterService interface {
	SindikasiReport(ctx context.Context, asOf time.Time, actor Actor) (SindikasiReport, error)
	ListItems(ctx context.Context) ([]SindikasiItem, error)
	UpsertItem(ctx context.Context, input UpdateSindikasiItemInput, actor Actor) (*SindikasiItem, error)
	DeleteItem(ctx context.Context, id uuid.UUID, actor Actor) error
}
