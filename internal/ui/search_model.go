package ui

import (
    "strings"

    "github.com/BlackOrder/complete-command/internal/actions"
    "github.com/BlackOrder/complete-command/internal/config"

    "github.com/charmbracelet/bubbles/list"
    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "io"
)

// model holds the UI state for the search helper.
type model struct {
    tools   []actions.SearchTool
    toolIdx int

    query textinput.Model
    dir   textinput.Model
    glob  textinput.Model

    ignoreCase bool
    word       bool
    regex      bool
    filesWith  bool
    hidden     bool
    context    int

    list list.Model

    // final holds the constructed command when the user exits.
    final string

    // cfg references the user configuration for saving preferences. It may be nil.
    cfg *config.Config
    // prefKey identifies the action associated with this model for preference storage.
    prefKey string
}

// NewSearchModel constructs a new search UI model.
// NewSearchModel returns a model without configuration. It is equivalent to
// NewSearchModelWithConfig with an empty preference key and nil config.
func NewSearchModel() model {
    return NewSearchModelWithConfig("", nil)
}

// NewSearchModelWithConfig constructs a new search UI model using the given action
// identifier and configuration. If cfg is non-nil and contains a preferred
// tool for the actionID, the tools slice will be reordered to place that tool
// first.
func NewSearchModelWithConfig(actionID string, cfg *config.Config) model {
    ti := textinput.New()
    ti.Placeholder = "search query"
    ti.Focus()

    di := textinput.New()
    di.Placeholder = "directory (default: .)"

    gi := textinput.New()
    gi.Placeholder = "glob (e.g. **/*.go)"

    tools := actions.AvailableSearchTools()
    // Reorder tools based on preference if available.
    if cfg != nil && actionID != "" {
        if pref, ok := cfg.PreferredTool(actionID); ok {
            for i, t := range tools {
                if string(t) == pref {
                    tools[0], tools[i] = tools[i], tools[0]
                    break
                }
            }
        }
    }

    items := []list.Item{
        item("Toggle Ignore Case"),
        item("Toggle Word Boundary"),
        item("Toggle Regex (off=literal)"),
        item("Toggle Files With Matches"),
        item("Toggle Show Hidden"),
        item("Set Context (+/-)"),
        item("Build & Insert"),
    }
    l := list.New(items, itemDelegate{}, 0, 0)
    l.SetShowStatusBar(false)
    l.SetFilteringEnabled(false)
    l.Title = "Search Options"

    return model{
        tools:   tools,
        toolIdx: 0,
        query:   ti,
        dir:     di,
        glob:    gi,
        list:    l,
        cfg:     cfg,
        prefKey: actionID,
    }
}

// item is a simple list item type.
type item string

// FilterValue returns the value used for filtering items.
func (i item) FilterValue() string { return string(i) }

// itemDelegate implements list item rendering.
type itemDelegate struct{}

// Height returns the height of each list item.
func (d itemDelegate) Height() int { return 1 }

// Spacing returns the spacing between list items.
func (d itemDelegate) Spacing() int { return 0 }

// Update handles list item update messages.
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

// Render draws the item to the writer.
func (d itemDelegate) Render(w io.Writer, m list.Model, idx int, listItem list.Item) {
    s := listItem.(item)
    prefix := "  "
    if idx == m.Index() {
        prefix = "> "
    }
    w.Write([]byte(prefix + string(s) + "\n"))
}

// resizeMsg is a custom message used to resize the list when the terminal changes.
type resizeMsg struct{ w, h int }

// Init initializes the model.
func (m model) Init() tea.Cmd { return nil }

// Update handles incoming events and updates the model accordingly.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        // Adjust list size when the window changes.
        return m, func() tea.Msg { return resizeMsg{msg.Width, msg.Height} }
    case resizeMsg:
        m.list.SetSize(msg.w, msg.h-8)
    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c", "esc":
            return m, tea.Quit
        case "tab":
            // cycle focus: query -> dir -> glob -> list -> query
            if m.query.Focused() {
                m.query.Blur()
                m.dir.Focus()
            } else if m.dir.Focused() {
                m.dir.Blur()
                m.glob.Focus()
            } else if m.glob.Focused() {
                m.glob.Blur()
                m.list.Select(0)
            } else {
                m.query.Focus()
            }
        case "left":
            if m.toolIdx > 0 {
                m.toolIdx--
            }
        case "right":
            if m.toolIdx < len(m.tools)-1 {
                m.toolIdx++
            }
        case "enter":
            switch m.list.Index() {
            case 0:
                m.ignoreCase = !m.ignoreCase
            case 1:
                m.word = !m.word
            case 2:
                m.regex = !m.regex
            case 3:
                m.filesWith = !m.filesWith
            case 4:
                m.hidden = !m.hidden
            case 5:
                // context is modified via +/- keys.
            case 6:
                // Build final command and exit. Also save the selected tool as a preference.
                opts := actions.SearchOptions{
                    Query:          m.query.Value(),
                    Dir:            strings.TrimSpace(m.dir.Value()),
                    Glob:           strings.TrimSpace(m.glob.Value()),
                    Word:           m.word,
                    IgnoreCase:     m.ignoreCase,
                    Regex:          m.regex,
                    Context:        m.context,
                    FilesWithMatch: m.filesWith,
                    Hidden:         m.hidden,
                }
                tool := m.tools[m.toolIdx]
                m.final = actions.BuildSearchCommand(tool, opts)
                // Persist preference if config is available.
                if m.cfg != nil && m.prefKey != "" {
                    m.cfg.SetPreference(m.prefKey, string(tool))
                    // ignore save error silently
                    _ = config.Save(m.cfg)
                }
                return m, tea.Quit
            }
        case "+":
            m.context++
        case "-", "_":
            if m.context > 0 {
                m.context--
            }
        }
    }
    // Pass messages to focused input or list.
    var cmd tea.Cmd
    if m.query.Focused() {
        m.query, cmd = m.query.Update(msg)
        return m, cmd
    }
    if m.dir.Focused() {
        m.dir, cmd = m.dir.Update(msg)
        return m, cmd
    }
    if m.glob.Focused() {
        m.glob, cmd = m.glob.Update(msg)
        return m, cmd
    }
    m.list, cmd = m.list.Update(msg)
    return m, cmd
}

// View renders the UI.
func (m model) View() string {
    header := "Search in files • Tool: " + string(m.tools[m.toolIdx]) + "  (←/→ switch)\n"
    header += "TAB to switch fields • ENTER to toggle/confirm • +/- for context • ESC to cancel\n\n"
    return header +
        "Query: " + m.query.View() + "\n" +
        "Dir:   " + m.dir.View() + "\n" +
        "Glob:  " + m.glob.View() + "\n\n" +
        m.list.View()
}

// FinalCommand returns the constructed command after the model exits.
func (m model) FinalCommand() string { return m.final }
