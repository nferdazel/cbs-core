package service_test

import (
	"context"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
)

func TestMockSLIKGateway_CheckDebtor(t *testing.T) {
	gw := service.NewMockSLIKGateway()

	// Case 1: Good debtor (LANCAR)
	res, err := gw.CheckDebtor(context.Background(), "3171012345678901")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsEligible {
		t.Fatal("expected debtor to be eligible")
	}
	if res.WorstCollectibility != domain.CollectibilityLancar {
		t.Fatalf("expected LANCAR, got %s", res.WorstCollectibility)
	}

	// Case 2: Bad debtor NPL (ending in 5)
	resBad, err := gw.CheckDebtor(context.Background(), "3171012345678905")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resBad.IsEligible {
		t.Fatal("expected NPL debtor to be ineligible")
	}
	if resBad.WorstCollectibility != domain.CollectibilityMacet {
		t.Fatalf("expected MACET, got %s", resBad.WorstCollectibility)
	}
}

func TestMockDukcapilGateway_VerifyIdentity(t *testing.T) {
	gw := service.NewMockDukcapilGateway()
	res, err := gw.VerifyIdentity(context.Background(), domain.DukcapilVerifyInput{
		NIK:         "3171012345678901",
		FullName:    "Budi Santoso",
		DateOfBirth: "1990-01-01",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsMatched {
		t.Fatal("expected Dukcapil NIK match to be true")
	}
}
