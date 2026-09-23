package domain

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// Kunci system_config identitas bank Form 00.00 (Lampiran II SEOJK
// No. 16/SEOJK.03/2024). Delapan kunci pertama di-seed migrasi 000046; dua belas
// kunci butir 10 s.d. 21 di-seed migrasi 000096. Nilai kosong berarti field BELUM
// TERSEDIA; ekspor menyebutkan kunci yang harus diisi, bukan mengarang nilai.
//
// Kunci didefinisikan di paket domain supaya lapisan penyimpanan (repository),
// layanan, dan pengekspor OJK memakai sumber yang sama.
const (
	OJKBankEmailKey       = "ojk.bank.email"
	OJKBankWebsiteKey     = "ojk.bank.website"
	OJKBankCityCodeKey    = "ojk.bank.city_code"
	OJKBankOJKRegionKey   = "ojk.bank.ojk_region_code"
	OJKPICNameKey         = "ojk.report.pic_name"
	OJKPICDivisionKey     = "ojk.report.pic_division"
	OJKPICPhoneKey        = "ojk.report.pic_phone"
	OJKPICEmailKey        = "ojk.report.pic_email"
	OJKDividendsPaidKey   = "ojk.report.dividends_paid"
	OJKAnnualBonusKey     = "ojk.report.annual_bonus_tantiem"
	OJKAuditInfoKey       = "ojk.report.audit_info"
	OJKShareNominalKey    = "ojk.report.share_nominal_value"
	OJKPublicOfferingKey  = "ojk.report.public_offering_status"
	OJKPVAStatusKey       = "ojk.report.pva_status"
	OJKEBankingKey        = "ojk.report.ebanking_status"
	OJKITProviderKey      = "ojk.report.it_provider"
	OJKLakuPandaiProvKey  = "ojk.report.laku_pandai_provider"
	OJKLakuPandaiAgentKey = "ojk.report.laku_pandai_agent_count"
	OJKRUPSOwnershipKey   = "ojk.report.rups_ownership_change"
	OJKUltimateHolderKey  = "ojk.report.ultimate_shareholders"
)

// OJKProfileFieldMaxLen adalah panjang maksimum satu nilai identitas OJK. Batas ini
// menolak tempelan berkas yang jelas bukan identitas, tanpa mengubah nilai yang wajar.
const OJKProfileFieldMaxLen = 1000

var (
	// ErrOJKProfileEmpty menolak permintaan ubah tanpa satu bidang pun, agar aksi
	// kosong tidak disalahbaca sebagai perubahan tersimpan.
	ErrOJKProfileEmpty = NewLocalizedError("ojk_profile_empty", "tidak ada bidang profil OJK yang dikirim")
	// ErrOJKProfileFieldTooLong menolak nilai yang melampaui batas kolomnya.
	ErrOJKProfileFieldTooLong = NewLocalizedError("ojk_profile_field_too_long", "nilai profil OJK terlalu panjang")
	// ErrOJKProfileEmailInvalid menolak surel yang tidak berbentuk surel.
	ErrOJKProfileEmailInvalid = NewLocalizedError("ojk_profile_email_invalid", "alamat surel harus memuat tanda @")
	// ErrOJKProfileWebsiteInvalid menolak situs web tanpa skema http(s)://.
	ErrOJKProfileWebsiteInvalid = NewLocalizedError("ojk_profile_website_invalid", "situs web harus diawali http:// atau https://")
	// ErrOJKProfilePhoneInvalid menolak karakter yang tidak mungkin ada pada nomor telepon.
	ErrOJKProfilePhoneInvalid = NewLocalizedError("ojk_profile_phone_invalid", "nomor telepon hanya boleh berisi angka, spasi, dan tanda + - ( ) .") //nolint:staticcheck // pesan operator berbahasa Indonesia; tanda baca bagian dari daftar karakter yang sah
	// ErrOJKProfileAgentCountInvalid menolak jumlah agen yang bukan angka.
	ErrOJKProfileAgentCountInvalid = NewLocalizedError("ojk_profile_agent_count_invalid", "jumlah agen Laku Pandai harus berupa angka")
)

// OJKProfile adalah identitas Form 00.00 yang tidak muat di tabel bank_profile.
// Seluruh nilai bersifat konfigurasi bank; kosong berarti belum diisi dan penerjemah
// laporan menandainya belum tersedia.
type OJKProfile struct {
	BankEmail         string `json:"bank_email"`
	BankWebsite       string `json:"bank_website"`
	BankCityCode      string `json:"bank_city_code"`
	BankOJKRegionCode string `json:"bank_ojk_region_code"`
	PICName           string `json:"pic_name"`
	PICDivision       string `json:"pic_division"`
	PICPhone          string `json:"pic_phone"`
	PICEmail          string `json:"pic_email"`

	DividendsPaid        string `json:"dividends_paid"`
	AnnualBonusTantiem   string `json:"annual_bonus_tantiem"`
	AuditInfo            string `json:"audit_info"`
	ShareNominalValue    string `json:"share_nominal_value"`
	PublicOfferingStatus string `json:"public_offering_status"`
	PVAStatus            string `json:"pva_status"`
	EBankingStatus       string `json:"ebanking_status"`
	ITProvider           string `json:"it_provider"`
	LakuPandaiProvider   string `json:"laku_pandai_provider"`
	LakuPandaiAgentCount string `json:"laku_pandai_agent_count"`
	RUPSOwnershipChange  string `json:"rups_ownership_change"`
	UltimateShareholders string `json:"ultimate_shareholders"`
}

// UpdateOJKProfileInput adalah masukan ubah profil OJK. Bidang bertipe pointer agar
// hanya bidang yang benar-benar dikirim yang diubah; bidang kosong tetap sah dan
// berarti "belum tersedia", bukan galat.
type UpdateOJKProfileInput struct {
	BankEmail         *string `json:"bank_email,omitempty"`
	BankWebsite       *string `json:"bank_website,omitempty"`
	BankCityCode      *string `json:"bank_city_code,omitempty"`
	BankOJKRegionCode *string `json:"bank_ojk_region_code,omitempty"`
	PICName           *string `json:"pic_name,omitempty"`
	PICDivision       *string `json:"pic_division,omitempty"`
	PICPhone          *string `json:"pic_phone,omitempty"`
	PICEmail          *string `json:"pic_email,omitempty"`

	DividendsPaid        *string `json:"dividends_paid,omitempty"`
	AnnualBonusTantiem   *string `json:"annual_bonus_tantiem,omitempty"`
	AuditInfo            *string `json:"audit_info,omitempty"`
	ShareNominalValue    *string `json:"share_nominal_value,omitempty"`
	PublicOfferingStatus *string `json:"public_offering_status,omitempty"`
	PVAStatus            *string `json:"pva_status,omitempty"`
	EBankingStatus       *string `json:"ebanking_status,omitempty"`
	ITProvider           *string `json:"it_provider,omitempty"`
	LakuPandaiProvider   *string `json:"laku_pandai_provider,omitempty"`
	LakuPandaiAgentCount *string `json:"laku_pandai_agent_count,omitempty"`
	RUPSOwnershipChange  *string `json:"rups_ownership_change,omitempty"`
	UltimateShareholders *string `json:"ultimate_shareholders,omitempty"`
}

// ojkProfileFieldSpec memetakan kunci system_config ke bidang struct, sehingga
// pemetaan baca/tulis/audit hanya ada di satu tempat.
type ojkProfileFieldSpec struct {
	key string
	ptr func(*OJKProfile) *string
}

func ojkProfileFieldSpecs() []ojkProfileFieldSpec {
	return []ojkProfileFieldSpec{
		{OJKBankEmailKey, func(p *OJKProfile) *string { return &p.BankEmail }},
		{OJKBankWebsiteKey, func(p *OJKProfile) *string { return &p.BankWebsite }},
		{OJKBankCityCodeKey, func(p *OJKProfile) *string { return &p.BankCityCode }},
		{OJKBankOJKRegionKey, func(p *OJKProfile) *string { return &p.BankOJKRegionCode }},
		{OJKPICNameKey, func(p *OJKProfile) *string { return &p.PICName }},
		{OJKPICDivisionKey, func(p *OJKProfile) *string { return &p.PICDivision }},
		{OJKPICPhoneKey, func(p *OJKProfile) *string { return &p.PICPhone }},
		{OJKPICEmailKey, func(p *OJKProfile) *string { return &p.PICEmail }},
		{OJKDividendsPaidKey, func(p *OJKProfile) *string { return &p.DividendsPaid }},
		{OJKAnnualBonusKey, func(p *OJKProfile) *string { return &p.AnnualBonusTantiem }},
		{OJKAuditInfoKey, func(p *OJKProfile) *string { return &p.AuditInfo }},
		{OJKShareNominalKey, func(p *OJKProfile) *string { return &p.ShareNominalValue }},
		{OJKPublicOfferingKey, func(p *OJKProfile) *string { return &p.PublicOfferingStatus }},
		{OJKPVAStatusKey, func(p *OJKProfile) *string { return &p.PVAStatus }},
		{OJKEBankingKey, func(p *OJKProfile) *string { return &p.EBankingStatus }},
		{OJKITProviderKey, func(p *OJKProfile) *string { return &p.ITProvider }},
		{OJKLakuPandaiProvKey, func(p *OJKProfile) *string { return &p.LakuPandaiProvider }},
		{OJKLakuPandaiAgentKey, func(p *OJKProfile) *string { return &p.LakuPandaiAgentCount }},
		{OJKRUPSOwnershipKey, func(p *OJKProfile) *string { return &p.RUPSOwnershipChange }},
		{OJKUltimateHolderKey, func(p *OJKProfile) *string { return &p.UltimateShareholders }},
	}
}

// KeyValues mengembalikan nilai seluruh field beserta kunci system_config-nya.
func (p *OJKProfile) KeyValues() map[string]string {
	out := make(map[string]string, len(ojkProfileFieldSpecs()))
	for _, s := range ojkProfileFieldSpecs() {
		out[s.key] = *s.ptr(p)
	}
	return out
}

// OJKProfileFromValues menyusun profil dari peta kunci system_config. Kunci yang
// tidak ada dibiarkan kosong (belum tersedia), bukan diberi nilai bawaan.
func OJKProfileFromValues(values map[string]string) *OJKProfile {
	p := &OJKProfile{}
	for _, s := range ojkProfileFieldSpecs() {
		*s.ptr(p) = values[s.key]
	}
	return p
}

// Normalize merapikan spasi di tepi tiap bidang; nilai di tengah tidak diubah.
func (p *OJKProfile) Normalize() {
	if p == nil {
		return
	}
	for _, s := range ojkProfileFieldSpecs() {
		ptr := s.ptr(p)
		*ptr = strings.TrimSpace(*ptr)
	}
}

// IsEmpty melaporkan tidak ada bidang pun yang dikirim.
func (in UpdateOJKProfileInput) IsEmpty() bool {
	return in.BankEmail == nil && in.BankWebsite == nil && in.BankCityCode == nil &&
		in.BankOJKRegionCode == nil && in.PICName == nil && in.PICDivision == nil &&
		in.PICPhone == nil && in.PICEmail == nil && in.DividendsPaid == nil &&
		in.AnnualBonusTantiem == nil && in.AuditInfo == nil && in.ShareNominalValue == nil &&
		in.PublicOfferingStatus == nil && in.PVAStatus == nil && in.EBankingStatus == nil &&
		in.ITProvider == nil && in.LakuPandaiProvider == nil && in.LakuPandaiAgentCount == nil &&
		in.RUPSOwnershipChange == nil && in.UltimateShareholders == nil
}

// Apply menyalin bidang yang dikirim ke profil kandidat.
func (in UpdateOJKProfileInput) Apply(p *OJKProfile) {
	set := func(dst *string, src *string) {
		if src != nil {
			*dst = *src
		}
	}
	set(&p.BankEmail, in.BankEmail)
	set(&p.BankWebsite, in.BankWebsite)
	set(&p.BankCityCode, in.BankCityCode)
	set(&p.BankOJKRegionCode, in.BankOJKRegionCode)
	set(&p.PICName, in.PICName)
	set(&p.PICDivision, in.PICDivision)
	set(&p.PICPhone, in.PICPhone)
	set(&p.PICEmail, in.PICEmail)
	set(&p.DividendsPaid, in.DividendsPaid)
	set(&p.AnnualBonusTantiem, in.AnnualBonusTantiem)
	set(&p.AuditInfo, in.AuditInfo)
	set(&p.ShareNominalValue, in.ShareNominalValue)
	set(&p.PublicOfferingStatus, in.PublicOfferingStatus)
	set(&p.PVAStatus, in.PVAStatus)
	set(&p.EBankingStatus, in.EBankingStatus)
	set(&p.ITProvider, in.ITProvider)
	set(&p.LakuPandaiProvider, in.LakuPandaiProvider)
	set(&p.LakuPandaiAgentCount, in.LakuPandaiAgentCount)
	set(&p.RUPSOwnershipChange, in.RUPSOwnershipChange)
	set(&p.UltimateShareholders, in.UltimateShareholders)
}

// ValidateOJKProfile menolak nilai yang jelas bukan identitas. Bidang kosong SELALU
// sah: kosong berarti bank belum menyediakannya. Nilai tidak dinormalkan diam-diam.
func ValidateOJKProfile(p *OJKProfile) error {
	if p == nil {
		return ErrOJKProfileEmpty
	}
	p.Normalize()
	for _, s := range ojkProfileFieldSpecs() {
		if len([]rune(*s.ptr(p))) > OJKProfileFieldMaxLen {
			return ErrOJKProfileFieldTooLong
		}
	}
	if p.BankEmail != "" && !strings.Contains(p.BankEmail, "@") {
		return ErrOJKProfileEmailInvalid
	}
	if p.PICEmail != "" && !strings.Contains(p.PICEmail, "@") {
		return ErrOJKProfileEmailInvalid
	}
	if p.BankWebsite != "" && !hasHTTPScheme(p.BankWebsite) {
		return ErrOJKProfileWebsiteInvalid
	}
	if p.PICPhone != "" && !allowedChars(p.PICPhone, "0123456789 +-().") {
		return ErrOJKProfilePhoneInvalid
	}
	if p.LakuPandaiAgentCount != "" && !allDigits(p.LakuPandaiAgentCount) {
		return ErrOJKProfileAgentCountInvalid
	}
	return nil
}

// hasHTTPScheme melaporkan situs web berawalan skema http:// atau https://.
func hasHTTPScheme(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

// allDigits melaporkan seluruh karakter adalah angka 0-9.
func allDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(value) > 0
}

// OJKProfileChanges membangun catatan audit nilai sebelum -> sesudah per kunci yang
// benar-benar berubah.
func OJKProfileChanges(before, after *OJKProfile) map[string]any {
	changes := map[string]any{}
	if before == nil {
		before = &OJKProfile{}
	}
	if after == nil {
		after = &OJKProfile{}
	}
	b, a := before.KeyValues(), after.KeyValues()
	for _, s := range ojkProfileFieldSpecs() {
		if b[s.key] != a[s.key] {
			changes[s.key] = map[string]any{"before": b[s.key], "after": a[s.key]}
		}
	}
	return changes
}

// OJKProfileStore membaca dan menulis identitas OJK pada system_config. Penulisan
// dibungkus transaksi bersama audit oleh layanan.
type OJKProfileStore interface {
	// Get membaca seluruh kunci ojk.*. Kunci yang belum ada menjadi nilai kosong.
	Get(ctx context.Context) (*OJKProfile, error)
	// GetTx membaca di dalam transaksi tulis, sehingga nilai "sebelum" pada audit
	// mencerminkan keadaan yang benar-benar ditimpa.
	GetTx(ctx context.Context, tx any) (*OJKProfile, error)
	// SaveTx menyimpan seluruh kunci (upsert), termasuk yang dikosongkan bank.
	SaveTx(ctx context.Context, tx any, profile *OJKProfile, updatedBy uuid.UUID) error
}

// OJKProfileService mengelola identitas Form 00.00 yang disimpan di system_config:
// membaca untuk pengaturan dan mengubah dengan validasi serta audit dalam satu transaksi.
type OJKProfileService interface {
	Get(ctx context.Context) (*OJKProfile, error)
	Update(ctx context.Context, input UpdateOJKProfileInput, actor Actor) (*OJKProfile, error)
}
