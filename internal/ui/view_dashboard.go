package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mfayaadh/oltuee/internal/olt"
)

// RenderDashboard renders the multi-site executive dashboard and active OLT overview
func (m *AppModel) RenderDashboard() string {
	var b strings.Builder

	// 1. Persistent Top Tab Bar
	sessName, host, port := "", "", ""
	if m.activeSession != nil {
		sessName = m.activeSession.Name
		host = m.activeSession.Host
		port = m.activeSession.Port
	}
	b.WriteString(RenderTopNav(m.state, sessName, host, port, len(m.uncfgONUs), m.width))

	// 2. Executive Multi-Site Metric Cards (Compact Single-Row Banner)
	siteSummaries := m.liveSummaries
	if len(siteSummaries) == 0 {
		siteSummaries = olt.CalculateSiteSummaries(m.store.Sessions, m.allCachedONUs)
	}

	totalONUs := 0
	totalOnline := 0
	totalDyingGasp := 0
	totalLOS := 0
	activeOLTCount := 0

	for _, s := range siteSummaries {
		totalONUs += s.TotalONUs
		totalOnline += s.OnlineCount
		totalDyingGasp += s.DyingGasp
		totalLOS += s.LOSCount
		if s.Status == "ONLINE" {
			activeOLTCount++
		}
	}

	// Compact Metric Pills (Never overflows terminal)
	onlinePill := lipgloss.NewStyle().Bold(true).Foreground(BrandEmerald).Render(fmt.Sprintf("🟢 ONLINE: %d", totalOnline))
	dgPill := lipgloss.NewStyle().Bold(true).Foreground(AccentYellow).Render(fmt.Sprintf("🟡 DYING-GASP: %d", totalDyingGasp))
	losPill := lipgloss.NewStyle().Bold(true).Foreground(AccentRed).Render(fmt.Sprintf("🔴 LOS: %d", totalLOS))
	oltPill := lipgloss.NewStyle().Bold(true).Foreground(AccentBlue).Render(fmt.Sprintf("🏢 OLT: %d/%d Aktif", activeOLTCount, len(m.store.Sessions)))
	totalPill := lipgloss.NewStyle().Bold(true).Foreground(TextPrimary).Render(fmt.Sprintf("📊 TOTAL: %d ONU", totalONUs))

	metricBanner := fmt.Sprintf(" %s   │   %s   │   %s   │   %s   │   %s ",
		onlinePill, dgPill, losPill, oltPill, totalPill)
	
	bannerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BrandEmerald).
		Background(BgCard).
		Padding(0, 1)

	b.WriteString(bannerStyle.Render(metricBanner) + "\n\n")

	// 3. Multi-Site Summary Table (Framed in CardStyle)
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(BrandEmerald).Render("🌐 Status Kesehatan Jaringan Seluruh Site OLT:") + "\n")
	
	var siteRows []string
	header := fmt.Sprintf(" %-18s %-18s %-7s %-8s %-10s %-6s %-9s",
		"LOKASI OLT", "HOST:PORT", "TOTAL", "ONLINE", "DYINGGASP", "LOS", "STATUS")
	siteRows = append(siteRows, TableHeaderStyle.Render(header))

	for i, s := range siteSummaries {
		statusPill := BadgeWorking.Render(" ONLINE ")
		if s.Status != "ONLINE" {
			statusPill = BadgeOffline.Render(" STANDBY ")
		}

		nameDisplay := s.Session.Name
		if len(nameDisplay) > 17 {
			nameDisplay = nameDisplay[:16] + "."
		}

		hostDisplay := fmt.Sprintf("%s:%s", s.Session.Host, s.Session.Port)
		if len(hostDisplay) > 18 {
			hostDisplay = hostDisplay[:17] + "."
		}

		row := fmt.Sprintf(" %-18s %-18s %-7d %-8d %-10d %-6d %s",
			nameDisplay,
			hostDisplay,
			s.TotalONUs,
			s.OnlineCount,
			s.DyingGasp,
			s.LOSCount,
			statusPill,
		)
		rowStyle := TableRowStyle
		if i%2 == 1 {
			rowStyle = TableAltRowStyle
		}
		siteRows = append(siteRows, rowStyle.Render(row))
	}
	b.WriteString(CardStyle.Render(strings.Join(siteRows, "\n")) + "\n\n")

	// 4. Hardware Cards of Active OLT (if connected)
	if m.activeSession != nil {
		gtghCards := m.getGTGHCards()
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(BrandEmerald).Render(fmt.Sprintf("🎛️  Kartu Layanan OLT Aktif (%s):", m.activeSession.Name)) + "\n")

		if len(gtghCards) == 0 {
			b.WriteString(CardStyle.Render("Sedang membaca data kartu GTGH dari OLT... Tekan [r] untuk refresh.") + "\n\n")
		} else {
			var cardRows []string
			cardHeader := fmt.Sprintf("  %-8s %-12s %-12s %-16s %-14s", "SLOT", "CFG TYPE", "REAL TYPE", "KAPASITAS PORT", "STATUS OPERASI")
			cardRows = append(cardRows, TableHeaderStyle.Render(cardHeader))

			for _, c := range gtghCards {
				statusPill := BadgeWorking.Render(" INSERVICE ")
				if c.Status != "INSERVICE" {
					statusPill = BadgeOffline.Render(" " + c.Status + " ")
				}

				portCount := "16 Port GPON"
				if c.Port != "" && c.Port != "0" {
					portCount = fmt.Sprintf("%s Port GPON", c.Port)
				}

				row := fmt.Sprintf("  Slot %-4s %-12s %-12s %-16s %s",
					c.Slot,
					c.CfgType,
					c.RealType,
					portCount,
					statusPill,
				)
				cardRows = append(cardRows, row)
			}
			b.WriteString(CardStyle.Render(strings.Join(cardRows, "\n")) + "\n\n")
		}

		// PON Ports Utilization Gauge
		if len(m.ponPorts) > 0 {
			b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(BrandEmerald).Render("🔌 Utilisasi Port PON Aktif:") + "\n")
			var portRows []string
			portHeader := fmt.Sprintf("  %-18s %-10s %-10s %-24s", "PORT INTERFACE", "ADMIN", "LINK", "UTILISASI PORT")
			portRows = append(portRows, TableHeaderStyle.Render(portHeader))

			for _, p := range m.ponPorts {
				linkBadge := BadgeWorking.Render(" UP ")
				if p.LinkState != "up" {
					linkBadge = BadgeOffline.Render(" DOWN ")
				}

				adminBadge := BadgeWorking.Render(" ENABLE ")
				if p.AdminState != "enable" {
					adminBadge = BadgeOffline.Render(" DISABLE ")
				}

				gauge := RenderProgressBar(p.TotalONUs, 128, 12)
				gaugeDisplay := fmt.Sprintf("[%s] %d ONU", gauge, p.TotalONUs)

				row := fmt.Sprintf("  %-18s %-10s %-10s %s",
					p.FullIntf,
					adminBadge,
					linkBadge,
					gaugeDisplay,
				)
				portRows = append(portRows, row)
			}
			b.WriteString(CardStyle.Render(strings.Join(portRows, "\n")) + "\n\n")
		}
	}

	// 5. Unconfigured Alert
	if len(m.uncfgONUs) > 0 {
		alertMsg := fmt.Sprintf("⚠️  Ditemukan %d ONU Unconfigured baru yang belum diaktivasi! Tekan [3] untuk buka wizard aktivasi.", len(m.uncfgONUs))
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(AccentYellow).Render(alertMsg) + "\n\n")
	}

	// 6. Navigation Shortcuts
	shortcuts := fmt.Sprintf(
		"[%s] All ONUs (Explorer)  [%s] Add ONU  [%s] Sesi OLT  [%s] Refresh  [%s] Keluar",
		KeyHelpStyle.Render("2 / /"),
		KeyHelpStyle.Render("3"),
		KeyHelpStyle.Render("4 / Esc"),
		KeyHelpStyle.Render("r"),
		KeyHelpStyle.Render("Ctrl+Q"),
	)
	b.WriteString(CardStyle.Render(shortcuts))

	return b.String()
}
