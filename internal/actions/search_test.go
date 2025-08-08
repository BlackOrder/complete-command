package actions

import (
    "strings"
    "testing"
)

func TestBuildSearchCommandGrep(t *testing.T) {
    opts := SearchOptions{Query: "foo", Dir: ".", Regex: true}
    cmd := BuildSearchCommand(ToolGrep, opts)
    if !strings.HasPrefix(cmd, "grep") {
        t.Fatalf("expected grep command, got %s", cmd)
    }
    if !strings.Contains(cmd, "foo") {
        t.Errorf("expected query to be present in command")
    }
}

func TestBuildSearchCommandRG(t *testing.T) {
    opts := SearchOptions{Query: "bar", Dir: ".", Regex: false, IgnoreCase: true}
    cmd := BuildSearchCommand(ToolRG, opts)
    if !strings.HasPrefix(cmd, "rg") {
        t.Fatalf("expected rg command, got %s", cmd)
    }
    if !strings.Contains(cmd, "-F") {
        t.Errorf("expected literal match flag (-F) to be included for non-regex")
    }
    if !strings.Contains(cmd, "-i") {
        t.Errorf("expected ignore case flag (-i) to be included")
    }
}

func TestBuildSearchCommandAwk(t *testing.T) {
    opts := SearchOptions{Query: "baz", Dir: "."}
    cmd := BuildSearchCommand(ToolAwk, opts)
    if !strings.HasPrefix(cmd, "awk") {
        t.Fatalf("expected awk fallback, got %s", cmd)
    }
    if !strings.Contains(cmd, "baz") {
        t.Errorf("expected query to be present in awk command")
    }
}