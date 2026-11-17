package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mfayaadh/oltuee/internal/olt"
)

// RenderONUListView renders the Unified Master All ONUs & PON Explorer interface
func (m *AppModel) RenderONUListView() string {
	var b strings.Builder

	// 1. Persistent Top Tab Bar
	sessName, host, port := "", "", ""
	if m.activeSession != nil {
		sessName = m.activeSession.Name
		host = m.activeSession.Host
		port = m.activeSession.Port
	}
	b.WriteString(RenderTopNav(m.state, sessName, host, port, len(m.uncfgONUs), m.width))

	// 2. Dropdown popup states
	if m.isSelectingOlt {
		return b.String() + m.renderOltDropdown()
	}
	gtghs := m.getGTGHCards()
	if m.isSelectingCard && len(gtghs) > 0 {
		return b.String() + m.renderCardDropdown(gtghs)
	}
	if m.isSelectingPort && len(gtghs) > 0 && m.selectedCardIdx < len(gtghs) {
		return b.String() + m.renderPortDropdown(gtghs[m.selectedCardIdx])
	}

	// 3. Header Title & Description
	title := TitleStyle.Render("🌐 Master All ONUs & PON Explorer (Telemetri FTTH)")
	subtitle := SubtitleStyle.Render("Tabel terpadu seluruh pelanggan lintas OLT. Gunakan filter cabang, slot kartu GTGH, dan port PON instan.")
	b.WriteString(fmt.Sprintf("%s\n%s\n\n", title, subtitle))

	// 4. Cascading Filter Chips Bar
	activeOltLabel := "Semua OLT (Global)"
	if m.activeSession != nil {
		activeOltLabel = m.activeSession.Name
	}

	activeCardLabel := "Semua Slot"
	if len(gtghs) > 0 && m.selectedCardIdx < len(gtghs) && m.activeSession != nil {
		activeCardLabel = fmt.Sprintf("Slot %s (%s)", gtghs[m.selectedCardIdx].Slot, gtghs[m.selectedCardIdx].CfgType)
	}

	activePortLabel := "Semua Port"
	if m.activeSession != nil && m.selectedPortNum > 0 {
		activePortLabel = fmt.Sprintf("Port %d (1..16)", m.selectedPortNum)
	}

	filterBar1 := fmt.Sprintf("[%s] OLT: %s  │  [%s] Kartu: %s  │  [%s] Port: %s",
		KeyHelpStyle.Render("o"),
		lipgloss.NewStyle().Bold(true).Foreground(BrandEmerald).Render(activeOltLabel),
		KeyHelpStyle.Render("c"),
		lipgloss.NewStyle().Bold(true).Foreground(BrandCyan).Render(activeCardLabel),
		KeyHelpStyle.Render("p"),
		lipgloss.NewStyle().Bold(true).Foreground(TextPrimary).Render(activePortLabel),
	)

	// Status Tabs
	statusFilters := []struct {
		id    int
		label string
	}{
		{0, "SEMUA"},
		{1, "🟢 ONLINE"},
		{2, "🟡 DYING-GASP"},
		{3, "🔴 LOS"},
	}
	var filterPills []string
	for _, f := range statusFilters {
		if m.filterMode == f.id {
			filterPills = append(filterPills, ActiveTabStyle.Render(f.label))
		} else {
			filterPills = append(filterPills, InactiveTabStyle.Render(f.label))
		}
	}
	filterBar2 := fmt.Sprintf("[%s] Status: %s", KeyHelpStyle.Render("f/Tab"), strings.Join(filterPills, " "))

	// Search Input Display
	searchIcon := "🔍"
	searchVal := m.searchQuery
	if m.isSearching {
		searchIcon = "✏️"
		searchVal = m.searchQuery + "█"
	}
	if searchVal == "" {
		searchVal = "Ketik / untuk cari..."
	}
	searchBar := fmt.Sprintf("%s Live Search [%s]: [ %s ]", searchIcon, KeyHelpStyle.Render("/"), searchVal)

	b.WriteString(fmt.Sprintf("%s\n%s   │   %s\n\n", filterBar1, filterBar2, searchBar))

	// 5. Filtered Records
	filtered := m.getFilteredUnifiedONUs()

	if len(filtered) == 0 {
		b.WriteString(CardStyle.Render("Tidak ada pelanggan yang cocok dengan kriteria filter saat ini.\nTekan [f] untuk ganti status filter atau [/] untuk ubah kata kunci pencarian."))
		b.WriteString("\n\n" + fmt.Sprintf("[%s] Ganti OLT  [%s] Ganti Slot  [%s] Ganti Port  [%s] Reset Filter  [%s] Dashboard",
			KeyHelpStyle.Render("o"),
			KeyHelpStyle.Render("c"),
			KeyHelpStyle.Render("p"),
			KeyHelpStyle.Render("f"),
			KeyHelpStyle.Render("1"),
		))
		return b.String()
	}

	// 6. Pagination Calculations
	pageSize := 12
	totalPages := (len(filtered) + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if m.onuPage >= totalPages {
		m.onuPage = totalPages - 1
	}
	if m.onuPage < 0 {
		m.onuPage = 0
	}

	startIdx := m.onuPage * pageSize
	endIdx := startIdx + pageSize
	if endIdx > len(filtered) {
		endIdx = len(filtered)
	}
	pageRecords := filtered[startIdx:endIdx]

	// 7. Master Data Table
	var tableLines []string
	header := fmt.Sprintf(" %-3s %-11s %-10s %-20s %-13s %-13s %-10s %-10s",
		"#", "LOKASI OLT", "PORT:ID", "NAMA PELANGGAN", "PPPOE USER", "SERIAL NUMBER", "RX POWER", "STATUS")
	tableLines = append(tableLines, TableHeaderStyle.Render(header))

	for i, r := range pageRecords {
		globalIdx := startIdx + i
		cursor := "  "
		rowStyle := TableRowStyle
		if globalIdx == m.onuCursor {
			cursor = "> "
			rowStyle = TableSelectedRowStyle
		} else if i%2 == 1 {
			rowStyle = TableAltRowStyle
		}

		statusBadge := BadgeWorking.Render(" ONLINE ")
		stLower := strings.ToLower(r.State)
		if strings.Contains(stLower, "dying") || strings.Contains(stLower, "gasp") {
			statusBadge = BadgeDyingGasp.Render(" DYINGGASP ")
		} else if strings.Contains(stLower, "los") || strings.Contains(stLower, "off") {
			statusBadge = BadgeLOS.Render(" LOS ")
		}

		custName := r.CustomerName
		if custName == "" {
			custName = r.Description
		}
		if custName == "" {
			custName = "-"
		}
		if len(custName) > 19 {
			custName = custName[:17] + ".."
		}

		pppoe := r.PPPoEUser
		if pppoe == "" {
			pppoe = "-"
		}
		if len(pppoe) > 12 {
			pppoe = pppoe[:11] + ".."
		}

		sn := r.SN
		if sn == "" {
			sn = "-"
		}
		if len(sn) > 12 {
			sn = sn[:11] + ".."
		}

		rxPwr := r.RxPower
		if rxPwr == "" {
			rxPwr = "-"
		}

		siteDisplay := r.SiteName
		siteDisplay = strings.TrimPrefix(siteDisplay, "OLT ")
		if idx := strings.Index(siteDisplay, " ("); idx > 0 {
			siteDisplay = siteDisplay[:idx]
		}
		if len(siteDisplay) > 10 {
			siteDisplay = siteDisplay[:9] + "."
		}

		portDisplay := r.PONPort + ":" + r.IDNumber
		if r.IDNumber == "" {
			portDisplay = r.OnuID
		}
		if len(portDisplay) > 9 {
			portDisplay = portDisplay[:8] + "."
		}

		row := fmt.Sprintf("%s%-3d %-11s %-10s %-20s %-13s %-13s %-10s %s",
			cursor,
			globalIdx+1,
			siteDisplay,
			portDisplay,
			custName,
			pppoe,
			sn,
			rxPwr,
			statusBadge,
		)
		tableLines = append(tableLines, rowStyle.Render(row))
	}

	b.WriteString(CardStyle.Render(strings.Join(tableLines, "\n")) + "\n")

	b.WriteString(fmt.Sprintf("\nMenampilkan %d - %d dari total %d pelanggan (Halaman %d/%d)\n\n",
		startIdx+1, endIdx, len(filtered), m.onuPage+1, totalPages))

	// 8. Bottom Action Bar
	helpText := fmt.Sprintf(
		"[%s] Buka Config ONT  [%s] Ganti OLT  [%s] Ganti Slot  [%s] Ganti Port  [%s] Filter  [%s] Cari  [%s] Dashboard",
		KeyHelpStyle.Render("Enter"),
		KeyHelpStyle.Render("o"),
		KeyHelpStyle.Render("c"),
		KeyHelpStyle.Render("p"),
		KeyHelpStyle.Render("f/Tab"),
		KeyHelpStyle.Render("/"),
		KeyHelpStyle.Render("1"),
	)
	b.WriteString(CardStyle.Render(helpText))

	if m.statusMsg != "" {
		b.WriteString("\n\n" + lipgloss.NewStyle().Foreground(AccentYellow).Render("🔔 "+m.statusMsg))
	}

	return b.String()
}

func (m *AppModel) renderOltDropdown() string {
	var b strings.Builder

	title := TitleStyle.Render("🏢  PILIH CABANG OLT TARGET:")
	subtitle := SubtitleStyle.Render("Pilih cabang untuk membatasi tampilan atau pilih SEMUA CABANG.")
	b.WriteString(fmt.Sprintf("%s\n%s\n\n", title, subtitle))

	var rows []string

	// Option 0: ALL OLTs
	cursor0 := "  "
	style0 := lipgloss.NewStyle().Foreground(TextPrimary)
	if m.oltCursor == 0 {
		cursor0 = "👉"
		style0 = lipgloss.NewStyle().Bold(true).Foreground(BrandEmerald).Background(lipgloss.Color("#1e293b"))
	}
	rows = append(rows, style0.Render(fmt.Sprintf("%s [SEMUA CABANG OLT] - Agregasi Lintas OLT", cursor0)))

	for i, sess := range m.store.Sessions {
		cursor := "  "
		style := lipgloss.NewStyle().Foreground(TextPrimary)
		if i+1 == m.oltCursor {
			cursor = "👉"
			style = lipgloss.NewStyle().Bold(true).Foreground(BrandEmerald).Background(lipgloss.Color("#1e293b"))
		}

		row := fmt.Sprintf("%s %-24s : %s:%s (%s)",
			cursor,
			sess.Name,
			sess.Host,
			sess.Port,
			sess.Model,
		)
		rows = append(rows, style.Render(row))
	}

	b.WriteString(ActiveCardStyle.Render(strings.Join(rows, "\n")) + "\n\n")

	helpText := fmt.Sprintf(
		"[%s] Pilih Cabang OLT  [%s] Navigasi  [%s] Batal",
		KeyHelpStyle.Render("Enter"),
		KeyHelpStyle.Render("↑ / ↓"),
		KeyHelpStyle.Render("Esc"),
	)
	b.WriteString(CardStyle.Render(helpText))

	return b.String()
}

func (m *AppModel) renderCardDropdown(gtghs []olt.CardInfo) string {
	var b strings.Builder

	title := TitleStyle.Render("🎛️  PILIH KARTU PON (GTGH) UNTUK DIBUKA:")
	subtitle := SubtitleStyle.Render("Memilih kartu akan otomatis membuka Port PON 1 pada slot tersebut.")
	b.WriteString(fmt.Sprintf("%s\n%s\n\n", title, subtitle))

	var rows []string
	for i, c := range gtghs {
		cursor := "  "
		style := lipgloss.NewStyle().Foreground(TextPrimary)
		if i == m.cardCursor {
			cursor = "👉"
			style = lipgloss.NewStyle().Bold(true).Foreground(BrandEmerald).Background(lipgloss.Color("#1e293b"))
		}

		statusBadge := BadgeWorking.Render(" INSERVICE ")
		if c.Status != "INSERVICE" && c.Status != "" {
			statusBadge = BadgeOffline.Render(" " + c.Status + " ")
		}

		row := fmt.Sprintf("%s Slot %-2s : %-10s (%s-Port) %s",
			cursor,
			c.Slot,
			c.CfgType,
			c.Port,
			statusBadge,
		)
		rows = append(rows, style.Render(row))
	}

	b.WriteString(ActiveCardStyle.Render(strings.Join(rows, "\n")) + "\n\n")

	helpText := fmt.Sprintf(
		"[%s] Pilih Slot & Buka Port 1  [%s] Navigasi  [%s] Batal",
		KeyHelpStyle.Render("Enter"),
		KeyHelpStyle.Render("↑ / ↓"),
		KeyHelpStyle.Render("Esc"),
	)
	b.WriteString(CardStyle.Render(helpText))

	return b.String()
}

func (m *AppModel) renderPortDropdown(card olt.CardInfo) string {
	var b strings.Builder

	title := TitleStyle.Render(fmt.Sprintf("🔌  PILIH PORT PON PADA SLOT %s (%s):", card.Slot, card.CfgType))
	subtitle := SubtitleStyle.Render("Gunakan tombol panah untuk memilih port target.")
	b.WriteString(fmt.Sprintf("%s\n%s\n\n", title, subtitle))

	var rows []string
	for p := 1; p <= 16; p++ {
		cursor := "  "
		style := lipgloss.NewStyle().Foreground(TextPrimary)
		if p == m.portCursor {
			cursor = "👉"
			style = lipgloss.NewStyle().Bold(true).Foreground(BrandEmerald).Background(lipgloss.Color("#1e293b"))
		}

		intfName := fmt.Sprintf("gpon-olt_1/%s/%d", card.Slot, p)
		row := fmt.Sprintf("%s Port %-2d : %-20s",
			cursor,
			p,
			intfName,
		)
		rows = append(rows, style.Render(row))
	}

	b.WriteString(ActiveCardStyle.Render(strings.Join(rows, "\n")) + "\n\n")

	helpText := fmt.Sprintf(
		"[%s] Buka Port Terpilih  [%s] Navigasi  [%s] Batal",
		KeyHelpStyle.Render("Enter"),
		KeyHelpStyle.Render("↑ / ↓"),
		KeyHelpStyle.Render("Esc"),
	)
	b.WriteString(CardStyle.Render(helpText))

	return b.String()
}

func (m *AppModel) getGTGHCards() []olt.CardInfo {
	var gtghs []olt.CardInfo
	for _, c := range m.cards {
		cfg := strings.ToLower(c.CfgType)
		real := strings.ToLower(c.RealType)
		if strings.Contains(cfg, "gtgh") || strings.Contains(real, "gtgh") ||
			strings.Contains(cfg, "gtgo") || strings.Contains(real, "gtgo") ||
			strings.Contains(cfg, "gpon") {
			gtghs = append(gtghs, c)
		}
	}
	if len(gtghs) == 0 && len(m.cards) > 0 {
		return m.cards
	}
	return gtghs
}

func (m *AppModel) getFilteredUnifiedONUs() []olt.GlobalONURecord {
	records := m.allCachedONUs
	if len(records) == 0 {
		records = olt.LoadAllCachedONUs(m.store.Sessions)
		m.allCachedONUs = records
	}

	var filtered []olt.GlobalONURecord
	for _, r := range records {
		// 1. OLT filter
		if m.activeSession != nil && r.SiteID != m.activeSession.ID {
			continue
		}

		// 2. Card Slot filter
		gtghs := m.getGTGHCards()
		if m.activeSession != nil && len(gtghs) > 0 && m.selectedCardIdx < len(gtghs) {
			slot := gtghs[m.selectedCardIdx].Slot
			if !strings.HasPrefix(r.PONPort, "1/"+slot+"/") && r.PONPort != slot {
				continue
			}
		}

		// 3. Port filter
		if m.activeSession != nil && m.selectedPortNum > 0 && len(gtghs) > 0 && m.selectedCardIdx < len(gtghs) {
			targetPort := fmt.Sprintf("1/%s/%d", gtghs[m.selectedCardIdx].Slot, m.selectedPortNum)
			if r.PONPort != targetPort && !strings.HasSuffix(r.PONPort, fmt.Sprintf("/%d", m.selectedPortNum)) {
				continue
			}
		}

		// 4. Status Filter
		stLower := strings.ToLower(r.State)
		if m.filterMode == 1 && !strings.Contains(stLower, "working") && !strings.Contains(stLower, "online") {
			continue
		} else if m.filterMode == 2 && !strings.Contains(stLower, "dying") && !strings.Contains(stLower, "gasp") {
			continue
		} else if m.filterMode == 3 && !strings.Contains(stLower, "los") && !strings.Contains(stLower, "off") {
			continue
		}

		// 5. Multi-Token Query Filter with Normalized Serial Number Matching
		if m.searchQuery != "" {
			tokens := strings.Fields(strings.ToLower(m.searchQuery))
			targetStr := strings.ToLower(fmt.Sprintf("%s %s %s %s %s %s %s %s %s",
				r.CustomerName, r.PPPoEUser, r.Description, r.SN, r.FullIntf, r.OnuID, r.SiteName, r.ModelType, r.PONPort))
			snClean := strings.ToLower(regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(r.SN, ""))

			allMatched := true
			for _, token := range tokens {
				cleanToken := regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(token, "")
				if !strings.Contains(targetStr, token) && (cleanToken == "" || !strings.Contains(snClean, cleanToken)) {
					allMatched = false
					break
				}
			}
			if !allMatched {
				continue
			}
		}

		filtered = append(filtered, r)
	}

	return filtered
}
