package config

import (
    "os"
    "testing"
)

// TestLoadSave ensures that preferences are persisted across load/save cycles.
func TestLoadSave(t *testing.T) {
    tmp := t.TempDir()
    // Override HOME for the duration of the test.
    oldHome := os.Getenv("HOME")
    t.Cleanup(func() { os.Setenv("HOME", oldHome) })
    if err := os.Setenv("HOME", tmp); err != nil {
        t.Fatalf("failed to set HOME: %v", err)
    }
    cfg, err := Load()
    if err != nil {
        t.Fatalf("Load error: %v", err)
    }
    if len(cfg.Preferences) != 0 {
        t.Fatalf("expected empty preferences, got %v", cfg.Preferences)
    }
    cfg.SetPreference("action", "tool")
    if err := Save(cfg); err != nil {
        t.Fatalf("Save error: %v", err)
    }
    cfg2, err := Load()
    if err != nil {
        t.Fatalf("Load after save error: %v", err)
    }
    if pref, ok := cfg2.PreferredTool("action"); !ok || pref != "tool" {
        t.Fatalf("expected preference 'tool', got %v %v", pref, ok)
    }
}