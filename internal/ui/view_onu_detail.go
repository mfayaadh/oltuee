package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderONUDetail renders the comprehensive ONT Config & Diagnostics interface
func (m *AppModel) RenderONUDetail() string {
	if m.selectedONU == nil && m.ontDetail == nil {
		return CardStyle.Render("Tidak ada ONT yang dipilih.")
	}

	var b strings.Builder

	// 1. Persistent Top Tab Bar
	sessName, host, port := "", "", ""
	if m.activeSession != nil {
		sessName = m.activeSession.Name
		host = m.activeSession.Host
		port = m.activeSession.Port
	}
	b.WriteString(RenderTopNav(m.state, sessName, host, port, len(m.uncfgONUs), m.width))

	targetIntf := ""
	if m.selectedONU != nil {
		targetIntf = m.selectedONU.Interface
	} else if m.ontDetail != nil {
		targetIntf = m.ontDetail.Interface
	}

	title := TitleStyle.Render(fmt.Sprintf("🛠️  PENGATURAN & KONFIGURASI ONT: %s", targetIntf))
	subtitle := SubtitleStyle.Render("Ubah profil VLAN, TCONT, Gemport, kredensial PPPoE, nama pelanggan, Wi-Fi, atau cek redaman optik live.")
	b.WriteString(fmt.Sprintf("%s\n%s\n\n", title, subtitle))

	// 2. Telemetry & Optical Diagnostics Card
	var diagRows []string
	custName := "-"
	modelType := "ALL"
	serialNum := "-"
	distance := "-"
	uptime := "-"
	adminSt := "enable"
	phaseSt := "working"
	wanIP := "-"
	vlanProfile := m.ontEditInputs[0]
	if vlanProfile == "" {
		vlanProfile = m.defaultVlanProfile()
	}
	cvlanID := m.ontEditInputs[1]
	if cvlanID == "" {
		cvlanID = m.defaultVlanID()
	}
	uploadProf := m.ontEditInputs[2]
	if uploadProf == "" {
		uploadProf = "default"
	}
	downloadProf := m.ontEditInputs[3]
	if downloadProf == "" {
		downloadProf = "default"
	}

	if m.ontDetail != nil {
		if m.ontDetail.Name != "" {
			custName = m.ontDetail.Name
		}
		if m.ontDetail.ModelType != "" {
			modelType = m.ontDetail.ModelType
		}
		if m.ontDetail.SerialNumber != "" {
			serialNum = m.ontDetail.SerialNumber
		}
		if m.ontDetail.Distance != "" {
			distance = m.ontDetail.Distance
		}
		if m.ontDetail.OnlineDuration != "" {
			uptime = m.ontDetail.OnlineDuration
		}
		if m.ontDetail.AdminState != "" {
			adminSt = m.ontDetail.AdminState
		}
		if m.ontDetail.PhaseState != "" {
			phaseSt = m.ontDetail.PhaseState
		}
		if m.ontDetail.WanIP != "" {
			wanIP = m.ontDetail.WanIP
		}
		if m.ontDetail.VlanProfile != "" {
			vlanProfile = m.ontDetail.VlanProfile
		}
		if m.ontDetail.CVLAN != "" {
			cvlanID = m.ontDetail.CVLAN
		}
	} else if m.selectedONU != nil {
		if m.selectedONU.Name != "" {
			custName = m.selectedONU.Name
		}
		if m.selectedONU.ModelType != "" {
			modelType = m.selectedONU.ModelType
		}
		if m.selectedONU.SerialNumber != "" {
			serialNum = m.selectedONU.SerialNumber
		}
		if m.selectedONU.AdminState != "" {
			adminSt = m.selectedONU.AdminState
		}
		if m.selectedONU.PhaseState != "" {
			phaseSt = m.selectedONU.PhaseState
		}
		if m.selectedWanIP != "" {
			wanIP = m.selectedWanIP
		}
	}

	phaseBadge := BadgeWorking.Render(" ONLINE ")
	if strings.Contains(strings.ToLower(phaseSt), "dying") {
		phaseBadge = BadgeDyingGasp.Render(" DYING-GASP ")
	} else if strings.Contains(strings.ToLower(phaseSt), "los") {
		phaseBadge = BadgeLOS.Render(" LOS ")
	}

	diagRows = append(diagRows, fmt.Sprintf("%-16s: %-24s │ %-16s: %s", "Nama Pelanggan", custName, "Serial Number", serialNum))
	diagRows = append(diagRows, fmt.Sprintf("%-16s: %-24s │ %-16s: %s", "Tipe / Model", modelType, "Status Link", phaseBadge))
	diagRows = append(diagRows, fmt.Sprintf("%-16s: %-24s │ %-16s: %s", "Jarak Optik", distance, "Online Duration", uptime))
	diagRows = append(diagRows, fmt.Sprintf("%-16s: %-24s │ %-16s: %s", "Current WAN IP", wanIP, "Admin State", adminSt))
	diagRows = append(diagRows, fmt.Sprintf("%-16s: %-24s │ %-16s: %s",
		"VLAN Profile Aktif", lipgloss.NewStyle().Bold(true).Foreground(BrandEmerald).Render(vlanProfile),
		"CVLAN ID OLT", lipgloss.NewStyle().Bold(true).Foreground(BrandCyan).Render(cvlanID)))

	b.WriteString(CardStyle.Render(strings.Join(diagRows, "\n")) + "\n\n")

	// 3. Realtime Optical Power Gauge Card
	var powerLines []string
	rxPower := "-"
	txPower := "-"
	oltRx := "-"
	atten := "-"
	signalStatus := "MEMBACA..."
	statusColor := TextSecondary

	if m.selectedPower != nil {
		if m.selectedPower.OnuRx != "" {
			rxPower = m.selectedPower.OnuRx
		} else if m.selectedPower.RxPower != "" {
			rxPower = m.selectedPower.RxPower
		}
		if m.selectedPower.TxPower != "" {
			txPower = m.selectedPower.TxPower
		}
		if m.selectedPower.OltRx != "" {
			oltRx = m.selectedPower.OltRx
		} else if m.selectedPower.OltRxPower != "" {
			oltRx = m.selectedPower.OltRxPower
		}
		if m.selectedPower.Attenuation != "" {
			atten = m.selectedPower.Attenuation
		}

		signalStatus = m.selectedPower.Status
		if signalStatus == "GOOD" {
			statusColor = BrandGreen
		} else if signalStatus == "WARNING" {
			statusColor = AccentYellow
		} else if signalStatus == "CRITICAL" {
			statusColor = AccentRed
		}
	}

	powerHeader := fmt.Sprintf("📶 REDAMAN OPTIK: Rx ONU: %s │ Tx ONU: %s │ Rx OLT: %s │ Redaman: %s │ Mutu: %s",
		lipgloss.NewStyle().Bold(true).Foreground(BrandEmerald).Render(rxPower),
		txPower, oltRx, atten,
		lipgloss.NewStyle().Bold(true).Foreground(statusColor).Render(signalStatus),
	)
	powerLines = append(powerLines, powerHeader)
	b.WriteString(ActiveCardStyle.Render(strings.Join(powerLines, "\n")) + "\n\n")

	// 4. Interactive Configuration Editor Form
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(BrandEmerald).Render("⚙️  FORMULIR EDIT KONFIGURASI ONT (ZTE PROFILES & PPPOE):") + "\n")
	b.WriteString(SubtitleStyle.Render("Gunakan [↑/↓/Tab] navigasi, [←/→/Spasi] ganti opsi dropdown, ketik teks untuk input, [s/Enter] simpan.") + "\n\n")

	labels := []string{
		"1. VLAN Profile (ZTE)",
		"2. CVLAN ID (VLAN Switch)",
		"3. Upload Profile (TCONT)",
		"4. Download Profile (Gemport)",
		"5. PPPoE Username",
		"6. PPPoE Password",
		"7. Nama Pelanggan",
		"8. Description / ODP / Area",
		"9. Wi-Fi SSID (Remote OMCI)",
		"10. Wi-Fi Password WPA2",
	}

	var formRows []string
	for i, label := range labels {
		prompt := "  "
		inputStyle := lipgloss.NewStyle().Foreground(TextPrimary)
		if i == m.ontEditCursor {
			prompt = "👉"
			inputStyle = lipgloss.NewStyle().Bold(true).Foreground(BrandEmerald).Background(lipgloss.Color("#1e293b"))
		}

		val := m.ontEditInputs[i]
		hint := ""

		if i == 0 { // VLAN Profile dropdown
			hint = " [← / → Pilih Profil]"
		} else if i == 1 { // CVLAN dropdown
			hint = " [← / → Database VLAN]"
		} else if i == 2 { // TCONT dropdown
			hint = " [← / → Profil TCONT]"
		} else if i == 3 { // Gemport dropdown
			hint = " [← / → Profil Traffic]"
		}
		// Note: PPPoE password is NOT masked per user request

		if val == "" && i != m.ontEditCursor {
			val = lipgloss.NewStyle().Foreground(TextMuted).Render("(belum diset)")
		}

		row := fmt.Sprintf("%s %-32s : %s%s", prompt, label, inputStyle.Render(" "+val+" "), lipgloss.NewStyle().Foreground(TextMuted).Render(hint))
		formRows = append(formRows, row)
	}

	b.WriteString(ActiveCardStyle.Render(strings.Join(formRows, "\n")) + "\n\n")

	// 5. Action Bar & Shortcuts
	helpText := fmt.Sprintf(
		"[%s] Simpan & Terapkan  [%s] Cek Redaman  [%s] Reboot ONT  [%s] Restore Factory  [%s] Kembali ke All ONUs",
		KeyHelpStyle.Render("s / Enter"),
		KeyHelpStyle.Render("p"),
		KeyHelpStyle.Render("r"),
		KeyHelpStyle.Render("f"),
		KeyHelpStyle.Render("Esc"),
	)
	b.WriteString(CardStyle.Render(helpText))

	if m.statusMsg != "" {
		b.WriteString("\n\n" + lipgloss.NewStyle().Bold(true).Foreground(BrandEmerald).Render("🔔 "+m.statusMsg))
	}

	return b.String()
}
