package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderLoginView renders the form to add or edit an OLT session
func (m *AppModel) RenderLoginView() string {
	var b strings.Builder

	title := TitleStyle.Render("📝 Form Tambah / Edit Koneksi OLT")
	subtitle := SubtitleStyle.Render("Gunakan tombol [Tab] / [Shift+Tab] untuk pindah field, [Enter] untuk Simpan.")
	b.WriteString(fmt.Sprintf("%s\n%s\n\n", title, subtitle))

	fieldLabels := []string{
		"1. Nama Site / Label",
		"2. IP Address / Host",
		"3. Port (Telnet)",
		"4. Username",
		"5. Password",
		"6. Model OLT (ZTE-C300 / ZTE-C320 / ZTE-C600)",
		"7. Default VLAN ID",
		"8. Default VLAN Profile",
	}

	var formRows []string
	for i, label := range fieldLabels {
		prompt := "  "
		inputStyle := lipgloss.NewStyle().Foreground(TextPrimary)
		if i == m.formCursor {
			prompt = "👉"
			inputStyle = lipgloss.NewStyle().Bold(true).Foreground(BrandEmerald).Background(lipgloss.Color("#1e293b"))
		}

		val := m.formInputs[i]
		if i == 4 { // password masking
			val = strings.Repeat("•", len(val))
		}
		if val == "" && i != m.formCursor {
			val = lipgloss.NewStyle().Foreground(TextMuted).Render("(kosong)")
		}

		row := fmt.Sprintf("%s %-32s : %s", prompt, label, inputStyle.Render(" "+val+" "))
		formRows = append(formRows, row)
	}

	b.WriteString(CardStyle.Render(strings.Join(formRows, "\n")))
	b.WriteString("\n\n")

	helpText := fmt.Sprintf(
		"[%s] Simpan & Connect  [%s] Pindah Field  [%s] Batal",
		KeyHelpStyle.Render("Enter"),
		KeyHelpStyle.Render("Tab / Down"),
		KeyHelpStyle.Render("Esc"),
	)
	b.WriteString(helpText)

	if m.statusMsg != "" {
		b.WriteString("\n\n" + lipgloss.NewStyle().Foreground(AccentRed).Render("⚠️ "+m.statusMsg))
	}

	return b.String()
}
