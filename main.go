package main

import (
    "fmt"
    "os"

    "github.com/BlackOrder/complete-command/internal/ui"
    tea "github.com/charmbracelet/bubbletea"
)

// main runs the TUI search helper and prints the resulting command.
func main() {
    m := ui.NewSearchModel()
    p := tea.NewProgram(m, tea.WithAltScreen())
    if mm, err := p.Run(); err != nil {
        fmt.Fprintln(os.Stderr, "error:", err)
        os.Exit(1)
    } else {
        if out, ok := mm.(interface{ FinalCommand() string }); ok {
            cmd := out.FinalCommand()
            if cmd != "" {
                fmt.Println(cmd)
                os.Exit(0)
            }
        }
    }
}
