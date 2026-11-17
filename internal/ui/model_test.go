package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mfayaadh/oltuee/internal/config"
	"github.com/mfayaadh/oltuee/internal/olt"
)

func TestModelTransitions(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "oltuee_ui_test")
	defer os.RemoveAll(tempDir)

	store := StoreHelper(filepath.Join(tempDir, "sessions.json"))
	m := InitialModel(store)

	// 1. Initial State should be StateSessions
	if m.state != StateSessions {
		t.Fatalf("Expected StateSessions, got %v", m.state)
	}

	// Verify RenderSessionsView produces non-empty text
	viewStr := m.View()
	if len(viewStr) == 0 {
		t.Fatal("Expected non-empty view string")
	}

	// 2. Press 'a' to open Login/Add form
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = newM.(AppModel)
	if m.state != StateLogin {
		t.Fatalf("Expected StateLogin after pressing 'a', got %v", m.state)
	}

	// 3. Press 'Esc' to cancel back to Sessions
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newM.(AppModel)
	if m.state != StateSessions {
		t.Fatalf("Expected StateSessions after pressing 'Esc', got %v", m.state)
	}

	// 4. Simulate OLT Connected message
	sampleCards := []olt.CardInfo{
		{Slot: "3", CfgType: "GTGH", RealType: "GTGHG", Port: "16", Status: "INSERVICE"},
		{Slot: "4", CfgType: "GTGH", RealType: "GTGHG", Port: "16", Status: "INSERVICE"},
	}
	m.cards = sampleCards
	m.state = StateDashboard

	// 5. Press '2' to go to ONU Explorer
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = newM.(AppModel)
	if m.state != StateONUList {
		t.Fatalf("Expected StateONUList after pressing '2', got %v", m.state)
	}

	// 6. Test Card Dropdown Toggle ('c')
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = newM.(AppModel)
	if !m.isSelectingCard {
		t.Fatal("Expected isSelectingCard to be true after pressing 'c'")
	}

	// Press 'enter' to select slot 3
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(AppModel)
	if m.isSelectingCard {
		t.Fatal("Expected isSelectingCard to be false after pressing Enter")
	}
	if m.selectedPortNum != 1 {
		t.Fatalf("Expected selectedPortNum to automatically be 1, got %d", m.selectedPortNum)
	}

	// 7. Test Port Dropdown Toggle ('p')
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = newM.(AppModel)
	if !m.isSelectingPort {
		t.Fatal("Expected isSelectingPort to be true after pressing 'p'")
	}
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newM.(AppModel)

	// 8. Test Open ONT Config & Detail View ('enter')
	m.allCachedONUs = []olt.GlobalONURecord{
		{SiteID: "sess-1", PONPort: "1/3/1", IDNumber: "1", FullIntf: "gpon-onu_1/3/1:1", CustomerName: "Budi", State: "working"},
	}
	m.onuCursor = 0
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(AppModel)
	if m.state != StateONUDetail {
		t.Fatalf("Expected StateONUDetail after pressing Enter on ONU, got %v", m.state)
	}

	// 9. Test Cycling VLAN Profiles in ONT Config
	m.oltFullConfig = &olt.OLTFullConfig{
		VlanProfiles: []olt.VlanProfile{
			{Name: "PPPOE-100", CVLAN: "100"},
			{Name: "PPPOE-200", CVLAN: "200"},
		},
	}
	m.ontEditCursor = 0
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = newM.(AppModel)
	if m.ontEditInputs[0] != "PPPOE-200" {
		t.Fatalf("Expected VLAN Profile to cycle to PPPOE-200, got %s", m.ontEditInputs[0])
	}

	// 10. Test Add ONU / Activation Wizard ('3')
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // back to list
	m = newM.(AppModel)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = newM.(AppModel)
	if m.state != StateActivation {
		t.Fatalf("Expected StateActivation after pressing 3, got %v", m.state)
	}
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}) // open manual register wizard
	m = newM.(AppModel)
	if !m.isActivating {
		t.Fatal("Expected isActivating to be true after pressing 'n'")
	}
}

func StoreHelper(path string) *config.Store {
	s := &config.Store{
		Sessions: []config.Session{
			{ID: "sess-1", Name: "Test OLT", Host: "127.0.0.1", Port: "23"},
		},
	}
	return s
}
