package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/BlackOrder/complete-command/internal/config"
	"github.com/BlackOrder/complete-command/internal/integration"
	"github.com/BlackOrder/complete-command/internal/registry"
	"github.com/BlackOrder/complete-command/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

// main runs the TUI search helper and prints the resulting command.
func main() {
	// Define command-line flags for shell integration and action selection.
	installShell := flag.Bool("install-shell", false, "Install shell integration (binds Ctrl+G to insert built commands)")
	uninstallShell := flag.Bool("uninstall-shell", false, "Uninstall shell integration")
	actionFlag := flag.String("action", "", "Skip the palette and start with the specified action (by ID, title or synonym)")

	// Custom usage message describing the tool.
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "complete-command is an interactive helper for composing system and networking commands.\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Usage:\n  %s [options] [action]\n\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nIf an action is provided as a positional argument or via --action, the palette step is skipped and the corresponding form is shown immediately.\n")
	}
	flag.Parse()

	// Handle shell integration installation/uninstallation.
	if *installShell {
		txt, err := integration.ToggleShellIntegration(installShell)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		fmt.Println(txt)
		return
	}
	if *uninstallShell {
		txt, err := integration.ToggleShellIntegration(new(bool))
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		fmt.Println(txt)
		return
	}

	// Determine the requested action, if any. Positional argument overrides --action if provided.
	if flag.NArg() > 0 {
		// Use the first argument as the action name if --action is empty.
		if *actionFlag == "" {
			*actionFlag = flag.Arg(0)
		}
	}

	// Load user configuration; ignore error on load.
	cfg, _ := config.Load()

	// Load registry of actions from YAML. If fail, fallback to legacy search.
	reg, err := registry.Load("registry.yaml")
	if err != nil || reg == nil || len(reg.Actions) == 0 {
		// fallback search mode if registry unavailable.
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

	// If an action is specified, attempt to locate it in the registry.
	if *actionFlag != "" {
		name := strings.ToLower(*actionFlag)
		var selected *registry.Action
		// Match by ID, title, or synonym.
		for i := range reg.Actions {
			act := &reg.Actions[i]
			if strings.ToLower(act.ID) == name || strings.ToLower(act.Title) == name {
				selected = act
				break
			}
			for _, syn := range act.Synonyms {
				if strings.ToLower(syn) == name {
					selected = act
					break
				}
			}
			if selected != nil {
				break
			}
		}
		if selected != nil {
			// Run the chosen action directly.
			actModel := ui.NewActionModel(*selected, cfg)
			p := tea.NewProgram(actModel, tea.WithAltScreen())
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
		// If action not found, report error and exit.
		fmt.Fprintf(os.Stderr, "Unknown action: %s\n", *actionFlag)
		os.Exit(1)
	}

	// No specific action provided: show palette for selection.
	palette := ui.NewPaletteModel(reg, cfg)
	p := tea.NewProgram(palette, tea.WithAltScreen())
	if mm, err2 := p.Run(); err2 != nil {
		fmt.Fprintln(os.Stderr, "error:", err2)
		os.Exit(1)
	} else {
		// Process selected action if any.
		if pm, ok := mm.(ui.PaletteModelAccessor); ok {
			sel := pm.GetSelected()
			if sel != nil {
				actModel := ui.NewActionModel(*sel, cfg)
				p2 := tea.NewProgram(actModel, tea.WithAltScreen())
				if mm2, err3 := p2.Run(); err3 != nil {
					fmt.Fprintln(os.Stderr, "error:", err3)
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
