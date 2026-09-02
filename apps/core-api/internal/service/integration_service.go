package service

import (
	"context"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

type mockSLIKGateway struct{}

func NewMockSLIKGateway() domain.SLIKGateway {
	return &mockSLIKGateway{}
}

func (g *mockSLIKGateway) CheckDebtor(_ context.Context, nik string) (*domain.SLIKCheckResult, error) {
	if len(nik) < 16 {
		return nil, fmt.Errorf("invalid NIK format, must be 16 digits")
	}

	// Mock logic: NIK ending in '5' returns bad collectibility (NPL) for testing
	worstCol := domain.CollectibilityLancar
	isEligible := true
	if len(nik) > 0 && nik[len(nik)-1] == '5' {
		worstCol = domain.CollectibilityMacet
		isEligible = false
	}

	return &domain.SLIKCheckResult{
		NIK:                 nik,
		FullName:            "NASABAH SLIK VERIFIED",
		CheckedAt:           time.Now().UTC(),
		WorstCollectibility: worstCol,
		TotalOutstanding:    25000000.0,
		ActiveFacilities: []domain.SLIKFacility{
			{
				BankName:       "BANK BCA",
				FacilityType:   "KARTU KREDIT",
				Plafond:        10000000,
				Outstanding:    2500000,
				Collectibility: domain.CollectibilityLancar,
				OverdueDays:    0,
			},
			{
				BankName:       "BPR SEJAHTERA",
				FacilityType:   "KREDIT MIKRO",
				Plafond:        30000000,
				Outstanding:    22500000,
				Collectibility: worstCol,
				OverdueDays:    0,
			},
		},
		IsEligible: isEligible,
	}, nil
}

type mockDukcapilGateway struct{}

func NewMockDukcapilGateway() domain.DukcapilGateway {
	return &mockDukcapilGateway{}
}

func (g *mockDukcapilGateway) VerifyIdentity(_ context.Context, input domain.DukcapilVerifyInput) (*domain.DukcapilVerifyResult, error) {
	if len(input.NIK) < 16 {
		return nil, fmt.Errorf("invalid NIK format")
	}
	return &domain.DukcapilVerifyResult{
		NIK:          input.NIK,
		IsMatched:    true,
		MatchScore:   0.98,
		VerifiedAt:   time.Now().UTC(),
		ProviderName: "DUKCAPIL_KEMENDAGRI_API",
	}, nil
}

type mockNotificationGateway struct{}

func NewMockNotificationGateway() domain.NotificationGateway {
	return &mockNotificationGateway{}
}

func (g *mockNotificationGateway) SendSMS(_ context.Context, phoneNumber, message string) error {
	fmt.Printf("📱 [SMS Gateway to %s]: %s\n", phoneNumber, message)
	return nil
}

func (g *mockNotificationGateway) SendWhatsApp(_ context.Context, phoneNumber, message string) error {
	fmt.Printf("💬 [WA Gateway to %s]: %s\n", phoneNumber, message)
	return nil
}
