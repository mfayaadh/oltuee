package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mfayaadh/oltuee/internal/config"
	"github.com/mfayaadh/oltuee/internal/ui"
)

func main() {
	store, err := config.NewStore()
	if err != nil {
		fmt.Printf("Error initializing config store: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(ui.InitialModel(store), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v\n", err)
		os.Exit(1)
	}
}
