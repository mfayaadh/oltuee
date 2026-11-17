package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderSessionsView renders the Winbox-style saved OLT connection list
func (m *AppModel) RenderSessionsView() string {
	var b strings.Builder

	title := TitleStyle.Render("⚡ OLTUEE - OLT Management System (TUI)")
	subtitle := SubtitleStyle.Render("Pilih OLT yang tersimpan (Winbox-style) atau tekan 'a' untuk menambah OLT baru.")
	b.WriteString(fmt.Sprintf("%s\n%s\n\n", title, subtitle))

	if len(m.store.Sessions) == 0 {
		b.WriteString(CardStyle.Render("Belum ada OLT tersimpan.\nTekan [a] untuk menambahkan koneksi OLT baru ke ~/.config/oltuee/sessions.json.") + "\n\n")
		helpText := fmt.Sprintf(
			"[%s] Tambah OLT  [%s] Bantuan  [%s] Keluar",
			KeyHelpStyle.Render("a"),
			KeyHelpStyle.Render("?"),
			KeyHelpStyle.Render("Ctrl+Q"),
		)
		b.WriteString(CardStyle.Render(helpText))
		if m.statusMsg != "" {
			b.WriteString("\n" + lipgloss.NewStyle().Foreground(AccentYellow).Render("🔔 "+m.statusMsg))
		}
		return b.String()
	}

	// Table Header & Rows (Framed in CardStyle)
	var sessionRows []string
	header := fmt.Sprintf(" %-3s %-22s %-20s %-8s %-12s %-10s %-8s",
		"#", "NAMA SITE", "IP / HOST", "PORT", "USER", "MODEL", "VLAN")
	sessionRows = append(sessionRows, TableHeaderStyle.Render(header))

	for i, sess := range m.store.Sessions {
		cursor := "  "
		style := TableRowStyle
		if i == m.sessionCursor {
			cursor = "> "
			style = TableSelectedRowStyle
		} else if i%2 == 1 {
			style = TableAltRowStyle
		}

		row := fmt.Sprintf("%s%-2d %-22s %-20s %-8s %-12s %-10s %-8s",
			cursor,
			i+1,
			truncate(sess.Name, 22),
			sess.Host,
			sess.Port,
			sess.Username,
			sess.Model,
			sess.DefaultVLAN,
		)
		sessionRows = append(sessionRows, style.Render(row))
	}

	b.WriteString(CardStyle.Render(strings.Join(sessionRows, "\n")) + "\n\n")

	// Quick Help bar
	helpText := fmt.Sprintf(
		"[%s] Connect  [%s] Tambah OLT  [%s] Edit  [%s] Hapus  [%s] All ONUs  [%s] Bantuan  [%s] Keluar",
		KeyHelpStyle.Render("Enter"),
		KeyHelpStyle.Render("a"),
		KeyHelpStyle.Render("e"),
		KeyHelpStyle.Render("d"),
		KeyHelpStyle.Render("2"),
		KeyHelpStyle.Render("?"),
		KeyHelpStyle.Render("Ctrl+Q"),
	)
	b.WriteString(CardStyle.Render(helpText))

	if m.statusMsg != "" {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(AccentYellow).Render("🔔 "+m.statusMsg))
	}

	return b.String()
}

func truncate(str string, maxLen int) string {
	if len(str) > maxLen {
		return str[:maxLen-2] + ".."
	}
	return str
}
