package main

import (
    "fmt"
    "os"

    "github.com/BlackOrder/complete-command/internal/config"
    "github.com/BlackOrder/complete-command/internal/registry"
    "github.com/BlackOrder/complete-command/internal/ui"
    tea "github.com/charmbracelet/bubbletea"
)

// main runs the TUI search helper and prints the resulting command.
func main() {
    // Load user configuration from the home directory. If loading fails, continue with nil config.
    cfg, _ := config.Load()
    // Load the registry of actions from the YAML file in the repository. If loading
    // fails, fall back to the legacy search model.
    reg, err := registry.Load("registry.yaml")
    if err != nil || reg == nil || len(reg.Actions) == 0 {
        // Fall back to the simple search model.
        m := ui.NewSearchModelWithConfig("search", cfg)
        p := tea.NewProgram(m, tea.WithAltScreen())
        if mm, err2 := p.Run(); err2 != nil {
            fmt.Fprintln(os.Stderr, "error:", err2)
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
        return
    }
    // Present a palette of actions to choose from.
    palette := ui.NewPaletteModel(reg, cfg)
    p := tea.NewProgram(palette, tea.WithAltScreen())
    if mm, err := p.Run(); err != nil {
        fmt.Fprintln(os.Stderr, "error:", err)
        os.Exit(1)
    } else {
        // Check if an action was selected.
        if pm, ok := mm.(ui.PaletteModelAccessor); ok {
            sel := pm.GetSelected()
            if sel != nil {
                // Create and run the dynamic model for the selected action.
                actModel := ui.NewActionModel(*sel, cfg)
                p2 := tea.NewProgram(actModel, tea.WithAltScreen())
                if mm2, err := p2.Run(); err != nil {
                    fmt.Fprintln(os.Stderr, "error:", err)
                    os.Exit(1)
                } else {
                    if out, ok := mm2.(interface{ FinalCommand() string }); ok {
                        cmd := out.FinalCommand()
                        if cmd != "" {
                            fmt.Println(cmd)
                            os.Exit(0)
                        }
                    }
                }
            }
        }
    }
}
