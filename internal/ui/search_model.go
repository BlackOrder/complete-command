package ui

import (
    "fmt"
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

    // Initialize an empty list; we'll populate its items after constructing the model.
    l := list.New(nil, itemDelegate{}, 0, 0)
    l.SetShowStatusBar(false)
    l.SetFilteringEnabled(false)
    l.Title = "Search options"

    // Build the model now so we can assign option pointers.
    m := model{
        tools:   tools,
        toolIdx: 0,
        query:   ti,
        dir:     di,
        glob:    gi,
        list:    l,
        cfg:     cfg,
        prefKey: actionID,
    }
    // Create list items with pointers to the model's fields so the delegate can
    // display current toggle and numeric values.
    boolItems := []list.Item{
        boolOptItem{label: "Ignore case", val: &m.ignoreCase},
        boolOptItem{label: "Word boundary", val: &m.word},
        boolOptItem{label: "Use regex", val: &m.regex},
        boolOptItem{label: "Only filenames", val: &m.filesWith},
        boolOptItem{label: "Include hidden", val: &m.hidden},
    }
    intItem := intOptItem{label: "Context lines", val: &m.context}
    buildItem := simpleItem{label: "Build & Insert"}
    // Set the list's items in the proper order.
    m.list.SetItems(append(append(boolItems, intItem), buildItem))
    return m
}

// item is a simple list item type.
// boolOptItem represents a toggleable boolean option. The val pointer is
// dereferenced to determine the current state.
type boolOptItem struct {
    label string
    val   *bool
}

func (b boolOptItem) FilterValue() string { return b.label }

// intOptItem represents an integer option. The val pointer holds the current
// numeric value, typically modified via +/- keys.
type intOptItem struct {
    label string
    val   *int
}

func (i intOptItem) FilterValue() string { return i.label }

// simpleItem represents a non-toggle list entry, such as the build command.
type simpleItem struct {
    label string
}

func (s simpleItem) FilterValue() string { return s.label }

// itemDelegate implements list item rendering for the search options. It
// displays the current state of boolean and integer options inline.
type itemDelegate struct{}

func (d itemDelegate) Height() int { return 1 }
func (d itemDelegate) Spacing() int { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, idx int, listItem list.Item) {
    prefix := "  "
    if idx == m.Index() {
        prefix = "> "
    }
    switch it := listItem.(type) {
    case boolOptItem:
        state := "[ ]"
        if it.val != nil && *it.val {
            state = "[x]"
        }
        fmt.Fprintf(w, "%s%s %s\n", prefix, state, it.label)
    case intOptItem:
        val := 0
        if it.val != nil {
            val = *it.val
        }
        fmt.Fprintf(w, "%s[%d] %s\n", prefix, val, it.label)
    case simpleItem:
        fmt.Fprintf(w, "%s%s\n", prefix, it.label)
    default:
        // Fallback rendering for unexpected types
        fmt.Fprintf(w, "%s%v\n", prefix, it)
    }
}

// Init initializes the model.
func (m model) Init() tea.Cmd { return nil }

// Update handles incoming events and updates the model accordingly.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c", "esc":
            return m, tea.Quit
        case "tab":
            // Cycle focus through inputs and list.
            if m.query.Focused() {
                m.query.Blur()
                m.dir.Focus()
            } else if m.dir.Focused() {
                m.dir.Blur()
                m.glob.Focus()
            } else if m.glob.Focused() {
                m.glob.Blur()
                // Focus the list by selecting the first item.
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
        case "+":
            // Increase context lines when the context option is selected.
            if m.list.Index() == 5 {
                m.context++
            }
        case "-", "_":
            // Decrease context lines when selected.
            if m.list.Index() == 5 && m.context > 0 {
                m.context--
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
                // context lines option; do nothing on enter
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
                    _ = config.Save(m.cfg)
                }
                return m, tea.Quit
            }
        }
    }
    // Pass messages to the focused input or list.
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
