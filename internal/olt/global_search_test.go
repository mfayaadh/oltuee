package olt

import (
	"testing"

	"github.com/mfayaadh/oltuee/internal/config"
)

func TestSearchGlobalONUs(t *testing.T) {
	records := []GlobalONURecord{
		{
			SiteID:       "site-alpha",
			SiteName:     "OLT Site Alpha",
			PONPort:      "1/3/1",
			IDNumber:     "1",
			FullIntf:     "gpon-onu_1/3/1:1",
			CustomerName: "Alice",
			PPPoEUser:    "100000000001",
			SN:           "ZTEG00000001",
			State:        "working",
		},
		{
			SiteID:       "site-beta",
			SiteName:     "OLT Site Beta",
			PONPort:      "1/8/2",
			IDNumber:     "5",
			FullIntf:     "gpon-onu_1/8/2:5",
			CustomerName: "Bob",
			PPPoEUser:    "100000000002",
			SN:           "ZTEG00000002",
			State:        "dyinggasp",
		},
		{
			SiteID:       "site-gamma",
			SiteName:     "OLT Site Gamma",
			PONPort:      "1/1/3",
			IDNumber:     "8",
			FullIntf:     "gpon-onu_1/1/3:8",
			CustomerName: "Charlie",
			PPPoEUser:    "100000000003",
			SN:           "HWTC00000003",
			State:        "los",
		},
	}

	// 1. Search by customer name
	res := SearchGlobalONUs(records, "ali", 0)
	if len(res) != 1 || res[0].CustomerName != "Alice" {
		t.Fatalf("expected 1 result for 'ali', got %d", len(res))
	}

	// 2. Search by PPPoE username
	res = SearchGlobalONUs(records, "100000000002", 0)
	if len(res) != 1 || res[0].CustomerName != "Bob" {
		t.Fatalf("expected 1 result for '100000000002', got %d", len(res))
	}

	// 3. Search by SN
	res = SearchGlobalONUs(records, "HWTC0000", 0)
	if len(res) != 1 || res[0].SN != "HWTC00000003" {
		t.Fatalf("expected 1 result for 'HWTC0000', got %d", len(res))
	}

	// 4. Filter by status: Online (1)
	res = SearchGlobalONUs(records, "", 1)
	if len(res) != 1 || res[0].State != "working" {
		t.Fatalf("expected 1 online result, got %d", len(res))
	}

	// 5. Filter by status: DyingGasp (2)
	res = SearchGlobalONUs(records, "", 2)
	if len(res) != 1 || res[0].State != "dyinggasp" {
		t.Fatalf("expected 1 dyinggasp result, got %d", len(res))
	}

	// 6. Filter by status: LOS (3)
	res = SearchGlobalONUs(records, "", 3)
	if len(res) != 1 || res[0].State != "los" {
		t.Fatalf("expected 1 los result, got %d", len(res))
	}
}

func TestCalculateSiteSummaries(t *testing.T) {
	sessions := []config.Session{
		{ID: "alpha", Name: "OLT Alpha"},
		{ID: "beta", Name: "OLT Beta"},
	}

	records := []GlobalONURecord{
		{SiteID: "alpha", State: "working"},
		{SiteID: "alpha", State: "working"},
		{SiteID: "alpha", State: "dyinggasp"},
		{SiteID: "beta", State: "los"},
	}

	summaries := CalculateSiteSummaries(sessions, records)
	if len(summaries) != 2 {
		t.Fatalf("expected 2 site summaries, got %d", len(summaries))
	}

	if summaries[0].TotalONUs != 3 || summaries[0].OnlineCount != 2 || summaries[0].DyingGasp != 1 {
		t.Errorf("unexpected Alpha metrics: %+v", summaries[0])
	}

	if summaries[1].TotalONUs != 1 || summaries[1].LOSCount != 1 {
		t.Errorf("unexpected Beta metrics: %+v", summaries[1])
	}
}
