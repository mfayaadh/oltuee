package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "oltuee_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	store := &Store{
		filePath: filepath.Join(tempDir, "sessions.json"),
		Sessions: []Session{},
	}

	// 1. Add session
	sess := Session{
		ID:          "test-1",
		Name:        "OLT Test Site",
		Host:        "192.168.1.100",
		Port:        "23",
		Username:    "admin",
		Password:    "pass",
		Model:       "ZTE-C300",
		DefaultVLAN: "100",
	}
	if err := store.AddOrUpdate(sess); err != nil {
		t.Fatalf("Failed to add session: %v", err)
	}

	if len(store.Sessions) != 1 {
		t.Fatalf("Expected 1 session, got %d", len(store.Sessions))
	}

	// 2. Load from disk
	store2 := &Store{filePath: store.filePath}
	if err := store2.Load(); err != nil {
		t.Fatalf("Failed to load store: %v", err)
	}
	if len(store2.Sessions) != 1 || store2.Sessions[0].Name != "OLT Test Site" {
		t.Fatalf("Loaded session mismatch: %+v", store2.Sessions)
	}

	// 3. Delete session
	if err := store2.Delete("test-1"); err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}
	if len(store2.Sessions) != 0 {
		t.Fatalf("Expected 0 sessions after delete, got %d", len(store2.Sessions))
	}
}
