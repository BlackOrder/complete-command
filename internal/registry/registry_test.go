package registry

import "testing"

// TestLoad ensures the registry YAML file is parsed and contains actions.
func TestLoad(t *testing.T) {
    reg, err := Load("../../registry.yaml")
    if err != nil {
        t.Fatalf("failed to load registry: %v", err)
    }
    if len(reg.Actions) == 0 {
        t.Fatal("expected at least one action in registry")
    }
}