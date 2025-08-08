package actions

import (
    "fmt"
    "strings"

    "github.com/BlackOrder/complete-command/internal/detect"
)

// SearchTool identifies a binary used for searching text in files.
type SearchTool string

// Supported search tools.
const (
    ToolRG   SearchTool = "rg"
    ToolGrep SearchTool = "grep"
    ToolAwk  SearchTool = "awk"
)

// SearchOptions defines the configurable options for building a search command.
type SearchOptions struct {
    Query          string
    Dir            string
    Word           bool
    IgnoreCase     bool
    Regex          bool
    Context        int
    Glob           string
    FilesWithMatch bool
    Hidden         bool
}

// AvailableSearchTools returns the search tools available on the current host.
func AvailableSearchTools() []SearchTool {
    if detect.Has(string(ToolRG)) {
        return []SearchTool{ToolRG, ToolGrep, ToolAwk}
    }
    if detect.Has(string(ToolGrep)) {
        return []SearchTool{ToolGrep, ToolAwk}
    }
    return []SearchTool{ToolAwk}
}

// BuildSearchCommand constructs a shell command using the selected tool and options.
func BuildSearchCommand(tool SearchTool, o SearchOptions) string {
    dir := "."
    if strings.TrimSpace(o.Dir) != "" {
        dir = o.Dir
    }
    switch tool {
    case ToolRG:
        args := []string{"rg"}
        if o.IgnoreCase {
            args = append(args, "-i")
        }
        if o.Word {
            args = append(args, "-w")
        }
        if !o.Regex {
            args = append(args, "-F")
        }
        if o.Context > 0 {
            args = append(args, fmt.Sprintf("-C %d", o.Context))
        }
        if o.Glob != "" {
            args = append(args, fmt.Sprintf("-g '%s'", o.Glob))
        }
        if o.FilesWithMatch {
            args = append(args, "-l")
        }
        if o.Hidden {
            args = append(args, "-uu")
        }
        args = append(args, fmt.Sprintf("'%s'", o.Query), dir)
        return strings.Join(args, " ")
    case ToolGrep:
        args := []string{"grep", "-R", "-n"}
        if o.IgnoreCase {
            args = append(args, "-i")
        }
        if o.Word {
            args = append(args, "-w")
        }
        if o.Context > 0 {
            args = append(args, fmt.Sprintf("-C %d", o.Context))
        }
        if o.FilesWithMatch {
            args = append(args, "-l")
        }
        if !o.Regex {
            args = append(args, "-F")
        }
        // grep doesn’t have glob the same way; use dir and pattern.
        return fmt.Sprintf("%s '%s' %s", strings.Join(args, " "), o.Query, dir)
    default:
        // Fallback: simple awk contains search.
        q := strings.ReplaceAll(o.Query, "'", "'\\''")
        return fmt.Sprintf("awk 'FNR==1{fn=FILENAME} /%s/{print fn":"FNR":"$0}' $(find %s -type f)", q, dir)
    }
}
