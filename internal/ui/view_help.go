package ui

import (
	"fmt"
	"strings"
)

// RenderHelpView renders keybindings and cheatsheet documentation
func (m *AppModel) RenderHelpView() string {
	var b strings.Builder

	title := TitleStyle.Render("📖 Panduan & Pintasan Tombol (Keybindings) OLTUEE")
	b.WriteString(title + "\n\n")

	keys := []struct {
		Key  string
		Desc string
	}{
		{"1", "Buka Dashboard (Ringkasan Multi-Site & Status Kartu Layanan OLT)"},
		{"2", "Buka All ONUs & PON Explorer (Daftar ONU Lintas OLT / Per Port)"},
		{"3", "Buka Unconfigured ONU & Wizard Aktivasi (Auto / Manual)"},
		{"4 / Esc", "Buka Daftar Sesi OLT (Winbox-style Saved Sessions) / Disconnect"},
		{"/", "Mulai Live Search pada All ONUs (Nama, PPPoE, SN, Interface, Site)"},
		{"o / c / p", "Dropdown Pilih OLT / Kartu GTGH / Port PON 1-16 (di All ONUs)"},
		{"f / Tab", "Ganti Filter Status (Semua / Online / DyingGasp / LOS)"},
		{"Enter", "Connect OLT / Buka Config & Telemetri ONT / Submit Form"},
		{"s / Enter", "Simpan & Terapkan Konfigurasi ONT (di halaman Detail ONT)"},
		{"p / r / f", "Cek Redaman Optik / Reboot ONT / Restore Factory (di Detail ONT)"},
		{"a / e / d", "Tambah / Edit / Hapus Sesi OLT (di daftar Sesi OLT)"},
		{"r", "Refresh data langsung dari OLT / Sinkronkan seluruh site"},
		{"Ctrl+Q / Ctrl+C", "Keluar dari aplikasi OLTUEE"},
	}

	var rows []string
	for _, k := range keys {
		rows = append(rows, fmt.Sprintf("%-16s : %s", KeyHelpStyle.Render(k.Key), k.Desc))
	}

	b.WriteString(CardStyle.Render(strings.Join(rows, "\n")) + "\n\n")
	b.WriteString(SubtitleStyle.Render("Tekan [Esc], [q], atau [1] untuk kembali ke menu utama."))

	return b.String()
}
