package config

import (
    "encoding/json"
    "os"
    "path/filepath"
)

// Config holds user preferences for command tool selection and other settings.
type Config struct {
    // Preferences maps an action identifier to the preferred tool name.
    Preferences map[string]string `json:"preferences"`
}

// defaultPath returns the path to the configuration file in the user's home directory.
func defaultPath() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(home, ".complete-command.json"), nil
}

// Load reads the configuration from disk. If the file does not exist, it returns
// a Config with empty preferences.
func Load() (*Config, error) {
    path, err := defaultPath()
    if err != nil {
        return nil, err
    }
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return &Config{Preferences: make(map[string]string)}, nil
        }
        return nil, err
    }
    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }
    if cfg.Preferences == nil {
        cfg.Preferences = make(map[string]string)
    }
    return &cfg, nil
}

// Save writes the configuration to disk. It overwrites any existing file.
func Save(cfg *Config) error {
    path, err := defaultPath()
    if err != nil {
        return err
    }
    data, err := json.MarshalIndent(cfg, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(path, data, 0o644)
}

// SetPreference records the selected tool for the given action identifier.
func (c *Config) SetPreference(actionID, tool string) {
    if c.Preferences == nil {
        c.Preferences = make(map[string]string)
    }
    c.Preferences[actionID] = tool
}

// PreferredTool returns the preferred tool for the given action identifier, if any.
func (c *Config) PreferredTool(actionID string) (string, bool) {
    t, ok := c.Preferences[actionID]
    return t, ok
}