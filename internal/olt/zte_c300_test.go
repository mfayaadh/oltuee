package olt

import (
	"regexp"
	"strings"
	"testing"
)

func TestCardParsing(t *testing.T) {
	// Sample 'show card' output from ZTE C300
	sampleShowCard := `Rack Shelf Slot CfgType RealType Port  HardVer SoftVer         Status
-------------------------------------------------------------------------------
1    1     0    PRWH    PRWH     0     V1.0.0                  INSERVICE
1    1     1    PRWH    PRWH     0     V1.0.0                  NOPOWER
1    1     3    GTGH    GTGHG    16    V1.0.0  V2.1.0          INSERVICE
1    1     4    GTGH    GTGHG    16    V1.0.0  V2.1.0          INSERVICE
1    1     10   SCTM    SCTMB    5     V1.0.1  V2.1.0          INSERVICE
1    1     11   SCTM             5                             OFFLINE
`
	lines := strings.Split(sampleShowCard, "\n")
	var cards []CardInfo
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "Rack") || strings.HasPrefix(l, "---") {
			continue
		}
		fields := strings.Fields(l)
		if len(fields) >= 5 {
			card := CardInfo{
				Rack: fields[0], Shelf: fields[1], Slot: fields[2],
				CfgType: fields[3], RealType: fields[4],
				Status: fields[len(fields)-1],
			}
			if len(fields) >= 7 {
				card.Port = fields[5]
			}
			cards = append(cards, card)
		}
	}

	if len(cards) != 6 {
		t.Fatalf("Expected 6 cards, got %d", len(cards))
	}

	gtghCount := 0
	for _, c := range cards {
		if strings.Contains(c.CfgType, "GTG") {
			gtghCount++
			if c.Status != "INSERVICE" {
				t.Errorf("Expected GTGH card to be INSERVICE, got %s", c.Status)
			}
		}
	}
	if gtghCount != 2 {
		t.Fatalf("Expected 2 GTGH cards, got %d", gtghCount)
	}
}

func TestONUStateParsing(t *testing.T) {
	// Sample 'show gpon onu state' output from ZTE C300
	sampleState := `OnuIndex   Admin State  OMCC State  Phase State  Channel
--------------------------------------------------------------
1/3/1:1     enable       enable      working      1(GPON)
1/3/1:2     enable       enable      working      1(GPON)
1/3/1:16    enable       disable     DyingGasp    1(GPON)
1/3/1:42    enable       disable     LOS          1(GPON)
1/3/1:5     enable       enable      working      1(GPON)
ONU Number: 46/49`

	re := regexp.MustCompile(`(\d+/\d+/\d+):(\d+)\s+(\S+)\s+(\S+)\s+(\S+)\s+`)

	var onus []ONUInfo
	for _, l := range strings.Split(sampleState, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "Onu") || strings.HasPrefix(l, "---") || strings.HasPrefix(l, "ONU Number") {
			continue
		}
		if m := re.FindStringSubmatch(l); len(m) >= 6 {
			onus = append(onus, ONUInfo{
				Interface:  "gpon-onu_" + m[1] + ":" + m[2],
				Port:       m[1],
				OnuID:      m[2],
				AdminState: m[3],
				OmciState:  m[4],
				PhaseState: m[5],
			})
		}
	}

	if len(onus) != 5 {
		t.Fatalf("Expected 5 ONUs, got %d", len(onus))
	}
	if onus[0].PhaseState != "working" {
		t.Errorf("Expected working, got %s", onus[0].PhaseState)
	}
	if onus[2].PhaseState != "DyingGasp" {
		t.Errorf("Expected DyingGasp, got %s", onus[2].PhaseState)
	}
	if onus[3].PhaseState != "LOS" {
		t.Errorf("Expected LOS, got %s", onus[3].PhaseState)
	}
}

func TestBaseInfoParsing(t *testing.T) {
	// Sample 'show gpon onu baseinfo' output from ZTE C300
	sampleBaseInfo := `OnuIndex            Type        Mode    AuthInfo                State
-------------------------------------------------------------------------------
gpon-onu_1/3/1:1    ALL         sn      SN:ZTEG00000001         ready
gpon-onu_1/3/1:2    ALL         sn      SN:ZTEG00000002         ready
gpon-onu_1/3/1:18   HG8245H5    sn      SN:HWTC00000003         ready
gpon-onu_1/3/1:25   ALL         sn      SN:ZTEG00000004         ready`

	snRe := regexp.MustCompile(`gpon-onu_(\d+/\d+/\d+:\d+)\s+(\S+)\s+\S+\s+SN:(\S+)\s+`)

	type baseData struct{ sn, model string }
	dataMap := make(map[string]baseData)

	for _, line := range strings.Split(sampleBaseInfo, "\n") {
		if m := snRe.FindStringSubmatch(line); len(m) >= 4 {
			dataMap[m[1]] = baseData{sn: m[3], model: m[2]}
		}
	}

	if len(dataMap) != 4 {
		t.Fatalf("Expected 4 entries, got %d", len(dataMap))
	}
	if d, ok := dataMap["1/3/1:1"]; !ok || d.sn != "ZTEG00000001" {
		t.Errorf("Missing or wrong SN for 1/3/1:1: %+v", d)
	}
	if d, ok := dataMap["1/3/1:18"]; !ok || d.sn != "HWTC00000003" || d.model != "HG8245H5" {
		t.Errorf("Missing or wrong data for 1/3/1:18: %+v", d)
	}
}

func TestUncfgParsing(t *testing.T) {
	// Sample 'show gpon onu uncfg' output from ZTE C300
	sampleUncfg := `OnuIndex                 Sn                  State
---------------------------------------------------------------------
gpon-onu_1/3/1:1         HWTC00000099        unknown`

	re := regexp.MustCompile(`gpon-onu_(\d+/\d+/\d+):(\d+)\s+([A-Za-z0-9]{8,16})\s+`)

	var uncfgs []UncfgONU
	for _, l := range strings.Split(sampleUncfg, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "Onu") || strings.HasPrefix(l, "---") {
			continue
		}
		if m := re.FindStringSubmatch(l); len(m) >= 4 {
			uncfgs = append(uncfgs, UncfgONU{Port: m[1], OnuID: m[2], SerialNumber: m[3]})
		}
	}

	if len(uncfgs) != 1 {
		t.Fatalf("Expected 1 unconfigured ONU, got %d", len(uncfgs))
	}
	if uncfgs[0].SerialNumber != "HWTC00000099" {
		t.Errorf("Expected SN HWTC00000099, got %s", uncfgs[0].SerialNumber)
	}
	if uncfgs[0].Port != "1/3/1" || uncfgs[0].OnuID != "1" {
		t.Errorf("Wrong port/id: %+v", uncfgs[0])
	}
}

func TestPromptDetection(t *testing.T) {
	tc := &TelnetClient{hostname: "OLT-CORE-01"}

	// Should detect prompt at end
	if !tc.looksLikePromptAtEnd("some output\r\nOLT-CORE-01#") {
		t.Error("Failed to detect prompt at end")
	}

	// Should detect config mode prompt
	if !tc.looksLikePromptAtEnd("some output\r\nOLT-CORE-01(config)#") {
		t.Error("Failed to detect config prompt at end")
	}

	// Should NOT detect prompt mid-output (the echoed command)
	if tc.looksLikePromptAtEnd("OLT-CORE-01#show gpon onu state gpon-olt_1/3/1") {
		t.Error("False positive: echoed command should not look like prompt")
	}

	// Should NOT detect command echo when cmd is passed
	if tc.looksLikeCommandFinished("OLT-CORE-01#show card\r\n", "show card") {
		t.Error("False positive: echoed command line should not finish execution")
	}

	// Table separators should not match
	if tc.looksLikePromptAtEnd("--------------------------------------------------------------") {
		t.Error("False positive on table separator")
	}
}

func TestCustomerNameParsing(t *testing.T) {
	testCases := []struct {
		raw         string
		expectedNm  string
		expectedPPP string
	}{
		{"100000000001-Alice", "Alice", "100000000001"},
		{"Bob Smith-100000000002", "Bob Smith", "100000000002"},
		{"Charlie Demo | 100000000003", "Charlie Demo", "100000000003"},
		{"100000000004", "-", "100000000004"},
		{"Diana.A-100000000005", "Diana.A", "100000000005"},
		{"78100000000006-Evan", "Evan", "78100000000006"},
		{"100000000007-Fiona / Block B", "Fiona / Block B", "100000000007"},
		{"100000000008 - George", "George", "100000000008"},
		{"Hannah-100000000009", "Hannah", "100000000009"},
		{"Ivan | 100000000010", "Ivan", "100000000010"},
	}

	for _, tc := range testCases {
		name, pppoe := parseCustomerName(tc.raw)
		if name != tc.expectedNm || pppoe != tc.expectedPPP {
			t.Errorf("parseCustomerName(%q) = (%q, %q); want (%q, %q)",
				tc.raw, name, pppoe, tc.expectedNm, tc.expectedPPP)
		}
	}
}

func TestCustomerDescriptionParsing(t *testing.T) {
	testCases := []struct {
		raw         string
		expectedNm  string
		expectedPPP string
	}{
		{"3$$100000000001-Alice$$", "Alice", "100000000001"},
		{"5$$100000000002-BOB$$", "BOB", "100000000002"},
		{"11$$100000000003-CHARLIE$$", "CHARLIE", "100000000003"},
	}

	for _, tc := range testCases {
		name, pppoe := parseCustomerDescription(tc.raw)
		if name != tc.expectedNm || pppoe != tc.expectedPPP {
			t.Errorf("parseCustomerDescription(%q) = (%q, %q); want (%q, %q)",
				tc.raw, name, pppoe, tc.expectedNm, tc.expectedPPP)
		}
	}
}

func TestOpticalPowerAttenuationRegex(t *testing.T) {
	sample := `           OLT                  ONU              Attenuation
-----------------------------------------------------------------
 up      Rx : -21.140(dBm)      Tx: 2.380(dBm)        23.520(dB)
 down    Tx : 3.120(dBm)        Rx: -18.420(dBm)      21.540(dB)`
	attenRe := regexp.MustCompile(`(?i)down[\s\S]*?([-\d.]+)\s*(?:\(db\)|\bdb\b)`)
	m := attenRe.FindStringSubmatch(sample)
	if len(m) < 2 || m[1] != "21.540" {
		t.Fatalf("expected attenuation 21.540, got %v", m)
	}
}
