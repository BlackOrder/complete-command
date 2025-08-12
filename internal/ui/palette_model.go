package ui

// This file defines the command palette model used to choose an action from the
// registry.  It has been enhanced with colourful styling using lipgloss and
// wraps its view in a rounded border for a more app‑like look.  The palette
// allows filtering by typing and navigation via the arrow keys.  When an item
// is selected and Enter is pressed the model exits and exposes the selected
// action via the PaletteModelAccessor interface.

import (
    "fmt"
    "io"
    "strings"

    "github.com/BlackOrder/complete-command/internal/config"
    "github.com/BlackOrder/complete-command/internal/registry"

    "github.com/charmbracelet/bubbles/list"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

// paletteItem wraps a registry.Action to implement the list.Item interface.  It
// exposes the action's title and synonyms for filtering.
type paletteItem struct {
    act *registry.Action
}

// FilterValue returns a string used by the list component to filter items.  It
// concatenates the action's title and its synonyms so typing any of those
// terms will match the corresponding item.
func (p paletteItem) FilterValue() string {
    if p.act == nil {
        return ""
    }
    return strings.Join(append([]string{p.act.Title}, p.act.Synonyms...), " ")
}

// paletteModel presents a list of actions loaded from the registry, allowing
// users to choose which command helper to invoke.  When an item is selected
// with Enter, the model records the chosen action and exits.
type paletteModel struct {
    list     list.Model
    cfg      *config.Config
    selected *registry.Action
}

// GetSelected returns the selected action after the palette model exits.  It is
// used outside of the ui package via the PaletteModelAccessor interface.
func (m paletteModel) GetSelected() *registry.Action {
    return m.selected
}

// PaletteModelAccessor is an interface exposing the GetSelected method.  The
// paletteModel implements this interface, allowing the main package to
// retrieve the chosen action without referencing the concrete type.
type PaletteModelAccessor interface {
    GetSelected() *registry.Action
}

// NewPaletteModel constructs a paletteModel with all actions from the
// registry.  Filtering is enabled to allow searching by synonyms.
func NewPaletteModel(reg *registry.Registry, cfg *config.Config) paletteModel {
    var items []list.Item
    for i := range reg.Actions {
        act := &reg.Actions[i]
        items = append(items, paletteItem{act: act})
    }
    l := list.New(items, paletteDelegate{}, 0, 0)
    l.SetShowStatusBar(false)
    l.SetFilteringEnabled(true)
    return paletteModel{
        list: l,
        cfg:  cfg,
    }
}

// Init returns nil; no asynchronous initialization is required.
func (m paletteModel) Init() tea.Cmd { return nil }

// Update handles user input for navigating and selecting actions.  When Enter
// is pressed, the selected action is stored and the program quits.
func (m paletteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c", "esc":
            return m, tea.Quit
        case "enter":
            // On enter, record selected action and quit.
            if item, ok := m.list.SelectedItem().(paletteItem); ok {
                m.selected = item.act
            }
            return m, tea.Quit
        }
    }
    var cmd tea.Cmd
    m.list, cmd = m.list.Update(msg)
    return m, cmd
}

// View renders the palette list along with a colourful header and basic
// instructions.  The entire view is wrapped in a rounded border to provide
// an app‑like feel.
func (m paletteModel) View() string {
    // Colourful header and instructions using lipgloss.
    title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("Command palette")
    instr := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("Use ↑/↓ or type to filter • Enter to select • ESC to quit")
    content := fmt.Sprintf("%s\n%s\n\n%s", title, instr, m.list.View())
    // Wrap in a rounded border with padding.
    style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1)
    return style.Render(content)
}

// FinalCommand is unused for the palette; it returns an empty string.
func (m paletteModel) FinalCommand() string { return "" }

// paletteDelegate customizes item rendering for the palette.  It highlights
// the selected item and applies subtle colouring to titles and candidate
// indicators.
type paletteDelegate struct{}

func (d paletteDelegate) Height() int { return 1 }
func (d paletteDelegate) Spacing() int { return 0 }
func (d paletteDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d paletteDelegate) Render(w io.Writer, m list.Model, idx int, listItem list.Item) {
    // Prefix indicates selection; pink when selected, grey otherwise.
    var prefix string
    if idx == m.Index() {
        prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("> ")
    } else {
        prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  ")
    }
    if item, ok := listItem.(paletteItem); ok && item.act != nil {
        title := lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Render(item.act.Title)
        cands := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(strings.Join(item.act.Candidates, "/"))
        fmt.Fprintf(w, "%s%s (%s)\n", prefix, title, cands)
    } else {
        // Fallback rendering for unexpected types.
        itemStr := lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Render(fmt.Sprint(listItem))
        fmt.Fprintf(w, "%s%s\n", prefix, itemStr)
    }
}