package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderActivationView renders the unconfigured ONUs scanner and live provisioning wizard
func (m *AppModel) RenderActivationView() string {
	var b strings.Builder

	// 1. Persistent Top Tab Bar
	sessName, host, port := "", "", ""
	if m.activeSession != nil {
		sessName = m.activeSession.Name
		host = m.activeSession.Host
		port = m.activeSession.Port
	}
	b.WriteString(RenderTopNav(m.state, sessName, host, port, len(m.uncfgONUs), m.width))

	if m.isActivating {
		// Render Registration Form Wizard
		return b.String() + m.renderActivationForm()
	}

	title := TitleStyle.Render("⚡ Aktivasi & Provisioning ONU Baru (Unconfigured)")
	subtitle := SubtitleStyle.Render("Mendeteksi modem baru yang tersambung ke port PON OLT secara simultan.")
	b.WriteString(fmt.Sprintf("%s\n%s\n\n", title, subtitle))

	if len(m.uncfgONUs) == 0 {
		b.WriteString(CardStyle.Render("🟢 Tidak ada ONU unconfigured yang terdeteksi saat ini.\nSemua modem pada OLT sudah teregistrasi dengan baik.\nTekan [r] untuk scan ulang port PON, atau [n] untuk registrasi manual."))
		b.WriteString("\n\n" + fmt.Sprintf("[%s] Scan Ulang  [%s] Pre-Register Manual  [%s] Dashboard  [%s] All ONUs",
			KeyHelpStyle.Render("r"),
			KeyHelpStyle.Render("n"),
			KeyHelpStyle.Render("1"),
			KeyHelpStyle.Render("2"),
		))
		return b.String()
	}

	// Table Header
	header := fmt.Sprintf("%-4s %-18s %-12s %-24s %-14s",
		"#", "PORT OLT", "ID USULAN", "SERIAL NUMBER (SN)", "STATUS")
	b.WriteString(TableHeaderStyle.Render(header) + "\n")

	for i, uncfg := range m.uncfgONUs {
		cursor := "  "
		style := TableRowStyle
		if i == m.uncfgCursor {
			cursor = "👉"
			style = TableSelectedRowStyle
		}

		statusBadge := BadgeLOS.Render(" UNCONFIGURED ")
		row := fmt.Sprintf("%s %-2d %-18s %-12s %-24s %s",
			cursor,
			i+1,
			"gpon-olt_"+uncfg.Port,
			uncfg.OnuID,
			uncfg.SerialNumber,
			statusBadge,
		)
		b.WriteString(style.Render(row) + "\n")
	}

	b.WriteString("\n")

	helpText := fmt.Sprintf(
		"[%s] Mulai Form Aktivasi ONU Terpilih  [%s] Pre-Register Manual  [%s] Scan Ulang  [%s] Dashboard",
		KeyHelpStyle.Render("Enter"),
		KeyHelpStyle.Render("n"),
		KeyHelpStyle.Render("r"),
		KeyHelpStyle.Render("1"),
	)
	b.WriteString(CardStyle.Render(helpText))

	if m.statusMsg != "" {
		b.WriteString("\n\n" + lipgloss.NewStyle().Foreground(AccentYellow).Render("🔔 "+m.statusMsg))
	}

	return b.String()
}

func (m *AppModel) renderActivationForm() string {
	var b strings.Builder

	title := TitleStyle.Render("⚡ Wizard Registrasi & Provisioning ONU Baru:")
	subtitle := SubtitleStyle.Render("Lengkapi data OLT di bawah ini. Gunakan [←/→/Spasi] untuk memilih profil dari OLT.")
	b.WriteString(fmt.Sprintf("%s\n%s\n\n", title, subtitle))

	labels := []string{
		"1. Pilih Cabang OLT",
		"2. Port PON OLT",
		"3. Nomor / ID ONU",
		"4. Serial Number (SN)",
		"5. Tipe Model Modem",
		"6. VLAN Profile (ZTE)",
		"7. VLAN ID (Internet)",
		"8. Upload Profile (TCONT)",
		"9. Download Profile (Gemport)",
		"10. Username PPPoE",
		"11. Password PPPoE",
		"12. Nama Pelanggan",
		"13. Description / ODP",
	}

	var formRows []string
	for i, label := range labels {
		prompt := "  "
		inputStyle := lipgloss.NewStyle().Foreground(TextPrimary)
		if i == m.actFormCursor {
			prompt = "👉"
			inputStyle = lipgloss.NewStyle().Bold(true).Foreground(BrandEmerald).Background(lipgloss.Color("#1e293b"))
		}

		val := m.actFormInputs[i]
		hint := ""

		if i == 0 {
			hint = " [← / → Pilih OLT]"
		} else if i == 4 {
			hint = " [← / → Pilih Model]"
		} else if i == 5 {
			hint = " [← / → Profil VLAN]"
		} else if i == 6 {
			hint = " [← / → Database VLAN]"
		} else if i == 7 {
			hint = " [← / → TCONT OLT]"
		} else if i == 8 {
			hint = " [← / → Traffic Shape]"
		} else if i == 10 {
			if val != "" {
				val = strings.Repeat("•", len(val))
			}
		}

		if val == "" && i != m.actFormCursor {
			val = lipgloss.NewStyle().Foreground(TextMuted).Render("(otomatis/opsional)")
		}

		row := fmt.Sprintf("%s %-34s : %s%s", prompt, label, inputStyle.Render(" "+val+" "), lipgloss.NewStyle().Foreground(TextMuted).Render(hint))
		formRows = append(formRows, row)
	}

	b.WriteString(ActiveCardStyle.Render(strings.Join(formRows, "\n")) + "\n\n")

	helpText := fmt.Sprintf(
		"[%s] Eksekusi Aktivasi ke OLT  [%s] Navigasi Field  [%s] Batal Wizard",
		KeyHelpStyle.Render("Enter"),
		KeyHelpStyle.Render("Tab / ↑ / ↓"),
		KeyHelpStyle.Render("Esc"),
	)
	b.WriteString(CardStyle.Render(helpText))

	if m.statusMsg != "" {
		b.WriteString("\n\n" + lipgloss.NewStyle().Foreground(AccentYellow).Render("🔔 "+m.statusMsg))
	}

	return b.String()
}
