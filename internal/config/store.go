package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Session represents a saved OLT connection profile
type Session struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Host           string    `json:"host"`
	Port           string    `json:"port"`
	Protocol       string    `json:"protocol"` // "telnet"
	Username       string    `json:"username"`
	Password       string    `json:"password"`
	Model          string    `json:"model"` // "ZTE-C300", "ZTE-C320", "ZTE-C600"
	DefaultVLAN    string    `json:"default_vlan"`
	DefaultProfile string    `json:"default_profile"`
	LastConnected  time.Time `json:"last_connected,omitempty"`
}

// Store handles persistent session storage
type Store struct {
	filePath string
	Sessions []Session `json:"sessions"`
}

// DataDir returns the base directory for OLTUEE config and cache files.
// Respects OLTUEE_DATA_DIR if set, otherwise defaults to ~/.config/oltuee.
func DataDir() (string, error) {
	if envDir := os.Getenv("OLTUEE_DATA_DIR"); envDir != "" {
		if err := os.MkdirAll(envDir, 0700); err != nil {
			return "", err
		}
		return envDir, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	configDir := filepath.Join(home, ".config", "oltuee")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return "", err
	}
	return configDir, nil
}

// NewStore initializes session store at ~/.config/oltuee/sessions.json (or $OLTUEE_DATA_DIR/sessions.json)
func NewStore() (*Store, error) {
	configDir, err := DataDir()
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(configDir, "sessions.json")
	store := &Store{
		filePath: filePath,
		Sessions: []Session{},
	}

	if err := store.Load(); err != nil {
		if os.IsNotExist(err) {
			_ = store.Save()
		}
	}

	return store, nil
}

// Load reads sessions from file
func (s *Store) Load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.Sessions)
}

// Save writes sessions to file
func (s *Store) Save() error {
	data, err := json.MarshalIndent(s.Sessions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0600)
}

// AddOrUpdate adds or updates a session
func (s *Store) AddOrUpdate(sess Session) error {
	found := false
	for i, item := range s.Sessions {
		if item.ID == sess.ID {
			s.Sessions[i] = sess
			found = true
			break
		}
	}
	if !found {
		s.Sessions = append(s.Sessions, sess)
	}
	return s.Save()
}

// Delete removes a session
func (s *Store) Delete(id string) error {
	newList := []Session{}
	for _, item := range s.Sessions {
		if item.ID != id {
			newList = append(newList, item)
		}
	}
	s.Sessions = newList
	return s.Save()
}
