package domain

import (
	"context"
	"time"
)

// --- OJK SLIK / CBAS Credit Bureau Models ---

type SLIKCollectibilityStatus string

const (
	CollectibilityLancar               SLIKCollectibilityStatus = "1_LANCAR"
	CollectibilityDalamPerhatianKhusus SLIKCollectibilityStatus = "2_DPK"
	CollectibilityKurangLancar         SLIKCollectibilityStatus = "3_KURANG_LANCAR"
	CollectibilityDiragukan            SLIKCollectibilityStatus = "4_DIRAGUKAN"
	CollectibilityMacet                SLIKCollectibilityStatus = "5_MACET"
)

type SLIKFacility struct {
	BankName       string                   `json:"bank_name"`
	FacilityType   string                   `json:"facility_type"` // KPR, KKM, Murabahah, Credit Card
	Plafond        float64                  `json:"plafond"`
	Outstanding    float64                  `json:"outstanding"`
	Collectibility SLIKCollectibilityStatus `json:"collectibility"`
	OverdueDays    int                      `json:"overdue_days"`
}

type SLIKCheckResult struct {
	NIK               string                 `json:"nik"`
	FullName          string                 `json:"full_name"`
	CheckedAt         time.Time              `json:"checked_at"`
	WorstCollectibility SLIKCollectibilityStatus `json:"worst_collectibility"`
	TotalOutstanding  float64                `json:"total_outstanding"`
	ActiveFacilities  []SLIKFacility         `json:"active_facilities"`
	IsEligible        bool                   `json:"is_eligible"`
}

// --- Dukcapil Identity Verification Models ---

type DukcapilVerifyInput struct {
	NIK         string `json:"nik"`
	FullName    string `json:"full_name"`
	DateOfBirth string `json:"date_of_birth"` // YYYY-MM-DD
}

type DukcapilVerifyResult struct {
	NIK          string    `json:"nik"`
	IsMatched    bool      `json:"is_matched"`
	MatchScore   float64   `json:"match_score"` // 0.0 - 1.0
	VerifiedAt   time.Time `json:"verified_at"`
	ProviderName string    `json:"provider_name"`
}

// --- Integration Gateway Interfaces ---

type SLIKGateway interface {
	CheckDebtor(ctx context.Context, nik string) (*SLIKCheckResult, error)
}

type DukcapilGateway interface {
	VerifyIdentity(ctx context.Context, input DukcapilVerifyInput) (*DukcapilVerifyResult, error)
}

type NotificationGateway interface {
	SendSMS(ctx context.Context, phoneNumber, message string) error
	SendWhatsApp(ctx context.Context, phoneNumber, message string) error
}
