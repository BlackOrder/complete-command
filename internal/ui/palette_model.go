package ui

import (
    "fmt"
    "io"
    "strings"

    "github.com/BlackOrder/complete-command/internal/config"
    "github.com/BlackOrder/complete-command/internal/registry"

    "github.com/charmbracelet/bubbles/list"
    tea "github.com/charmbracelet/bubbletea"
)

// paletteItem wraps a registry.Action to implement the list.Item interface. It
// exposes the action's title and synonyms for filtering.
type paletteItem struct {
    act *registry.Action
}

func (p paletteItem) FilterValue() string {
    if p.act == nil {
        return ""
    }
    // Include synonyms and title in filter string.
    return strings.Join(append([]string{p.act.Title}, p.act.Synonyms...), " ")
}

// paletteModel presents a list of actions loaded from the registry, allowing
// users to choose which command helper to invoke. When an item is selected
// with Enter, the model records the chosen action and exits.
type paletteModel struct {
    list     list.Model
    cfg      *config.Config
    selected *registry.Action
}

// GetSelected returns the selected action after the palette model exits. It is
// used outside of the ui package via the PaletteModelAccessor interface.
func (m paletteModel) GetSelected() *registry.Action {
    return m.selected
}

// PaletteModelAccessor is an interface exposing the GetSelected method. The
// paletteModel implements this interface, allowing the main package to
// retrieve the chosen action without referencing the concrete type.
type PaletteModelAccessor interface {
    GetSelected() *registry.Action
}

// NewPaletteModel constructs a paletteModel with all actions from the
// registry. Filtering is enabled to allow searching by synonyms.
func NewPaletteModel(reg *registry.Registry, cfg *config.Config) paletteModel {
    var items []list.Item
    for i := range reg.Actions {
        act := &reg.Actions[i]
        items = append(items, paletteItem{act: act})
    }
    l := list.New(items, paletteDelegate{}, 0, 0)
    l.SetShowStatusBar(false)
    l.SetFilteringEnabled(true)
    l.Title = "Choose command"
    return paletteModel{
        list: l,
        cfg:  cfg,
    }
}

// Init returns nil; no asynchronous initialization is required.
func (m paletteModel) Init() tea.Cmd { return nil }

// Update handles user input for navigating and selecting actions. When Enter
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

// View renders the palette list along with basic instructions.
func (m paletteModel) View() string {
    s := "Command palette\n"
    s += "Use ↑/↓ or type to filter • Enter to select • ESC to quit\n\n"
    return s + m.list.View()
}

// FinalCommand is unused for the palette; it returns an empty string.
func (m paletteModel) FinalCommand() string { return "" }

// paletteDelegate customizes item rendering for the palette. It highlights
// the selected item.
type paletteDelegate struct{}

func (d paletteDelegate) Height() int { return 1 }
func (d paletteDelegate) Spacing() int { return 0 }
func (d paletteDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d paletteDelegate) Render(w io.Writer, m list.Model, idx int, listItem list.Item) {
    prefix := "  "
    if idx == m.Index() {
        prefix = "> "
    }
    if item, ok := listItem.(paletteItem); ok && item.act != nil {
        fmt.Fprintf(w, "%s%s (%s)\n", prefix, item.act.Title, strings.Join(item.act.Candidates, "/"))
    } else {
        fmt.Fprintf(w, "%s%v\n", prefix, listItem)
    }
}