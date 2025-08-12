package ui

import (
    "fmt"
    "io"
    "sort"
    "strconv"
    "strings"

    "github.com/BlackOrder/complete-command/internal/config"
    "github.com/BlackOrder/complete-command/internal/detect"
    "github.com/BlackOrder/complete-command/internal/registry"

    "github.com/charmbracelet/bubbles/list"
    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

// actionModel is a generic form for building commands defined in the registry.
// It dynamically constructs input fields and list items based on the action's
// field definitions. Supported field types include string, path, bool, int,
// float, enum, and multi. On completion the model produces a shell command
// constructed from the selected tool and populated field values.
//
// The model uses TAB to cycle between text inputs and the list of option
// toggles. Ctrl+T cycles through available tools. The list displays boolean,
// numeric and enum fields with their current values and a "Build & Insert"
// entry to finalize the command. Preferences for a selected tool are
// persisted using the provided config pointer and action ID.
type actionModel struct {
    action  registry.Action
    tools   []string
    toolIdx int

    // input fields keyed by field key for strings, paths and multi entries.
    strInputs map[string]*textinput.Model
    // boolean fields as list items
    boolItems []boolFieldItem
    // numeric fields: int and float
    intItems   []intFieldItem
    floatItems []floatFieldItem
    // enum fields
    enumItems []enumFieldItem
    // keep order of items for display: combination of bool, int/float, enum and build.
    list list.Model

    // final command after building
    final   string
    cfg     *config.Config
    prefKey string
}

// boolFieldItem represents a toggleable boolean option in a dynamic action.
type boolFieldItem struct {
    key   string
    label string
    val   *bool
}

func (b boolFieldItem) FilterValue() string { return b.label }

// intFieldItem represents an integer option that can be increased or decreased.
type intFieldItem struct {
    key   string
    label string
    val   *int
    min   *float64
    max   *float64
}

func (i intFieldItem) FilterValue() string { return i.label }

// floatFieldItem represents a float option that can be modified via +/- keys.
type floatFieldItem struct {
    key   string
    label string
    val   *float64
    min   *float64
    max   *float64
}

func (f floatFieldItem) FilterValue() string { return f.label }

// enumFieldItem represents an enumeration option cycling through choices.
type enumFieldItem struct {
    key     string
    label   string
    choices []string
    idx     *int
}

func (e enumFieldItem) FilterValue() string { return e.label }

// staticItem is used for the final "Build & Insert" entry.
type staticItem struct {
    label string
}

func (s staticItem) FilterValue() string { return s.label }

// NewActionModel constructs a new dynamic action model for the given action.
// It uses the provided configuration to reorder tool candidates based on
// previous preferences. Fields are initialised with defaults when provided.
func NewActionModel(action registry.Action, cfg *config.Config) actionModel {
    // Determine available tools by checking candidate binaries in PATH.
    var available []string
    for _, c := range action.Candidates {
        if detect.Has(c) {
            available = append(available, c)
        }
    }
    if len(available) == 0 {
        // If none are found, fallback to listing all candidates anyway; the
        // resulting command may still be valid if user has them installed.
        available = append(available, action.Candidates...)
    }
    // Reorder tools based on preference.
    if cfg != nil && action.ID != "" {
        if pref, ok := cfg.PreferredTool(action.ID); ok {
            for i, t := range available {
                if t == pref {
                    available[0], available[i] = available[i], available[0]
                    break
                }
            }
        }
    }
    // Prepare input maps and list items.
    strInputs := make(map[string]*textinput.Model)
    var boolItems []boolFieldItem
    var intItems []intFieldItem
    var floatItems []floatFieldItem
    var enumItems []enumFieldItem
    // Create inputs based on field definitions.
    for _, f := range action.Fields {
        label := f.Label
        if label == "" {
            label = f.Key
        }
        switch f.Type {
        case "string", "path", "multi":
            ti := textinput.New()
            if f.Placeholder != "" {
                ti.Placeholder = f.Placeholder
            }
            if defStr, ok := f.Default.(string); ok {
                ti.SetValue(defStr)
            }
            strInputs[f.Key] = &ti
        case "bool":
            defBool := false
            if b, ok := f.Default.(bool); ok {
                defBool = b
            }
            val := new(bool)
            *val = defBool
            boolItems = append(boolItems, boolFieldItem{key: f.Key, label: label, val: val})
        case "int":
            var defInt int
            if f.Default != nil {
                switch v := f.Default.(type) {
                case int:
                    defInt = v
                case int64:
                    defInt = int(v)
                case float64:
                    defInt = int(v)
                }
            }
            val := new(int)
            *val = defInt
            intItems = append(intItems, intFieldItem{key: f.Key, label: label, val: val, min: f.Min, max: f.Max})
        case "float":
            var defF float64
            if f.Default != nil {
                switch v := f.Default.(type) {
                case float32:
                    defF = float64(v)
                case float64:
                    defF = v
                case int:
                    defF = float64(v)
                }
            }
            val := new(float64)
            *val = defF
            floatItems = append(floatItems, floatFieldItem{key: f.Key, label: label, val: val, min: f.Min, max: f.Max})
        case "enum":
            idx := new(int)
            defIdx := 0
            // Determine default index from Default (which may be string)
            if f.Default != nil {
                switch v := f.Default.(type) {
                case string:
                    for i, c := range f.Choices {
                        if c == v {
                            defIdx = i
                            break
                        }
                    }
                }
            }
            *idx = defIdx
            enumItems = append(enumItems, enumFieldItem{key: f.Key, label: label, choices: f.Choices, idx: idx})
        }
    }
    // Build list items: bool, int, float, enum and final build item.
    var items []list.Item
    for _, b := range boolItems {
        items = append(items, b)
    }
    for _, i := range intItems {
        items = append(items, i)
    }
    for _, f := range floatItems {
        items = append(items, f)
    }
    for _, e := range enumItems {
        items = append(items, e)
    }
    items = append(items, staticItem{label: "Build & Insert"})
    l := list.New(items, actionItemDelegate{}, 0, 0)
    l.SetShowStatusBar(false)
    l.SetFilteringEnabled(false)
    // Create and return the model.
    return actionModel{
        action:    action,
        tools:     available,
        toolIdx:   0,
        strInputs: strInputs,
        boolItems: boolItems,
        intItems:  intItems,
        floatItems: floatItems,
        enumItems: enumItems,
        list:      l,
        cfg:       cfg,
        prefKey:   action.ID,
    }
}

// Init implements tea.Model. No asynchronous initialization is required.
func (m actionModel) Init() tea.Cmd { return nil }

// Update processes incoming messages, updating focused inputs, toggles and selection.
func (m actionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c", "esc":
            return m, tea.Quit
        case "ctrl+t":
            // Cycle to next available tool.
            if len(m.tools) > 1 {
                m.toolIdx = (m.toolIdx + 1) % len(m.tools)
            }
            return m, nil
        case "tab":
            // Cycle focus between text inputs and list.
            focusedKey := ""
            for k, ti := range m.strInputs {
                if ti.Focused() {
                    focusedKey = k
                    break
                }
            }
            if focusedKey != "" {
                // Blur current and focus next input; order is deterministic by map iteration in Go
                blurNext := false
                var nextKey string
                for k, ti := range m.strInputs {
                    if k == focusedKey {
                        blurNext = true
                        ti.Blur()
                        continue
                    }
                    if blurNext && nextKey == "" {
                        nextKey = k
                        break
                    }
                }
                if nextKey != "" {
                    m.strInputs[nextKey].Focus()
                } else {
                    m.list.Select(0)
                }
            } else {
                // If list is focused, cycle back to first input.
                for _, ti := range m.strInputs {
                    ti.Focus()
                    break
                }
            }
        case "left", "right":
            // Left/right are unused for tool selection; ignore.
        case "+":
            // Increment numeric values when selected.
            idx := m.list.Index()
            if idx >= 0 && idx < len(m.boolItems)+len(m.intItems)+len(m.floatItems)+len(m.enumItems) {
                offset := len(m.boolItems)
                // Int items
                if idx >= offset && idx < offset+len(m.intItems) {
                    intIdx := idx - offset
                    v := m.intItems[intIdx].val
                    *v = *v + 1
                } else {
                    offset += len(m.intItems)
                    if idx >= offset && idx < offset+len(m.floatItems) {
                        floatIdx := idx - offset
                        *m.floatItems[floatIdx].val += 1.0
                    }
                }
            }
        case "-", "_":
            // Decrement numeric values when selected.
            idx := m.list.Index()
            if idx >= 0 && idx < len(m.boolItems)+len(m.intItems)+len(m.floatItems)+len(m.enumItems) {
                offset := len(m.boolItems)
                if idx >= offset && idx < offset+len(m.intItems) {
                    intIdx := idx - offset
                    if m.intItems[intIdx].min != nil {
                        minVal := int(*m.intItems[intIdx].min)
                        if *m.intItems[intIdx].val > minVal {
                            *m.intItems[intIdx].val--
                        }
                    } else if *m.intItems[intIdx].val > 0 {
                        *m.intItems[intIdx].val--
                    }
                } else {
                    offset += len(m.intItems)
                    if idx >= offset && idx < offset+len(m.floatItems) {
                        floatIdx := idx - offset
                        if m.floatItems[floatIdx].min != nil {
                            minVal := *m.floatItems[floatIdx].min
                            if *m.floatItems[floatIdx].val > minVal {
                                *m.floatItems[floatIdx].val -= 0.1
                            }
                        } else {
                            *m.floatItems[floatIdx].val -= 0.1
                        }
                    }
                }
            }
        case "enter":
            // If a text input is focused, pressing enter builds and exits.
            for _, ti := range m.strInputs {
                if ti.Focused() {
                    if m.cfg != nil && m.prefKey != "" {
                        m.cfg.SetPreference(m.prefKey, m.tools[m.toolIdx])
                        _ = config.Save(m.cfg)
                    }
                    m.final = m.buildCommand()
                    return m, tea.Quit
                }
            }
            idx := m.list.Index()
            // Determine which section this index belongs to.
            if idx >= 0 && idx < len(m.boolItems)+len(m.intItems)+len(m.floatItems)+len(m.enumItems)+1 {
                limit := len(m.boolItems)
                if idx < limit {
                    // toggle bool
                    *m.boolItems[idx].val = !*m.boolItems[idx].val
                } else {
                    idx -= limit
                    limit = len(m.intItems)
                    if idx < limit {
                        // numeric item; enter does nothing for ints
                    } else {
                        idx -= limit
                        limit = len(m.floatItems)
                        if idx < limit {
                            // floats: no action on enter
                        } else {
                            idx -= limit
                            limit = len(m.enumItems)
                            if idx < limit {
                                // cycle enum value to next
                                e := &m.enumItems[idx]
                                if len(e.choices) > 0 {
                                    cur := *e.idx
                                    cur++
                                    if cur >= len(e.choices) {
                                        cur = 0
                                    }
                                    *e.idx = cur
                                }
                            } else {
                                // final build item selected; build command and exit
                                if m.cfg != nil && m.prefKey != "" {
                                    m.cfg.SetPreference(m.prefKey, m.tools[m.toolIdx])
                                    _ = config.Save(m.cfg)
                                }
                                cmd := m.buildCommand()
                                m.final = cmd
                                return m, tea.Quit
                            }
                        }
                    }
                }
            }
        }
    }
    // Update text inputs if any of them is focused.
    for _, ti := range m.strInputs {
        if ti.Focused() {
            var cmd tea.Cmd
            *ti, cmd = ti.Update(msg)
            return m, cmd
        }
    }
    // Otherwise update the list.
    var cmd tea.Cmd
    m.list, cmd = m.list.Update(msg)
    return m, cmd
}

// buildCommand assembles the command string for the selected tool and current
// field values. It is called when exiting the model.
func (m actionModel) buildCommand() string {
    // Build a map of field values keyed by their keys.
    values := make(map[string]interface{})
    for k, ti := range m.strInputs {
        values[k] = ti.Value()
    }
    // Booleans
    for _, b := range m.boolItems {
        if b.val != nil {
            values[b.key] = *b.val
        }
    }
    // Integers
    for _, it := range m.intItems {
        if it.val != nil {
            values[it.key] = *it.val
        }
    }
    // Floats
    for _, fl := range m.floatItems {
        if fl.val != nil {
            values[fl.key] = *fl.val
        }
    }
    // Enums
    for _, e := range m.enumItems {
        if e.idx != nil && *e.idx < len(e.choices) {
            values[e.key] = e.choices[*e.idx]
        }
    }
    // Render template for selected tool.
    template := m.action.Template[m.tools[m.toolIdx]]
    result := ""
    i := 0
    for i < len(template) {
        // Look for next placeholder "{{"...
        if strings.HasPrefix(template[i:], "{{") {
            end := strings.Index(template[i:], "}}")
            if end < 0 {
                // unmatched braces; append rest
                result += template[i:]
                break
            }
            placeholder := template[i+2 : i+end]
            // Parse placeholder: key[?] or key|fmt or key|fmt? etc.
            if strings.Contains(placeholder, "|") {
                parts := strings.SplitN(placeholder, "|", 2)
                key := parts[0]
                format := parts[1]
                v, ok := values[key]
                if ok {
                    // Use Sprintf with format specifier, trimming leading % if present.
                    if strings.HasPrefix(format, "%") {
                        result += fmt.Sprintf(format, v)
                    } else {
                        result += fmt.Sprintf(format, v)
                    }
                }
            } else if strings.Contains(placeholder, "?") {
                // Conditional placeholder: key?value
                parts := strings.SplitN(placeholder, "?", 2)
                key := parts[0]
                flag := parts[1]
                v, ok := values[key]
                if ok {
                    switch bv := v.(type) {
                    case bool:
                        if bv {
                            result += flag
                        }
                    case string:
                        if bv != "" {
                            result += flag
                        }
                    case int:
                        if bv != 0 {
                            result += flag
                        }
                    case float64:
                        if bv != 0 {
                            result += flag
                        }
                    }
                }
            } else {
                // Simple key replacement
                v, ok := values[placeholder]
                if ok {
                    switch vv := v.(type) {
                    case string:
                        result += vv
                    case bool:
                        if vv {
                            result += "true"
                        } else {
                            result += "false"
                        }
                    case int:
                        result += strconv.Itoa(vv)
                    case float64:
                        // Convert float to string without trailing zeros.
                        result += fmt.Sprintf("%g", vv)
                    default:
                        result += fmt.Sprint(vv)
                    }
                }
            }
            i += end + 2
        } else {
            result += string(template[i])
            i++
        }
    }
    fields := strings.Fields(result)
    return strings.Join(fields, " ")
}

// View renders the current form state. A colourful header and instructions
// precede the list of inputs and options. The entire view is wrapped in a
// rounded border to provide an app‑like feel.
func (m actionModel) View() string {
    // Colourful header with action title and current tool.
    title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render(m.action.Title)
    tool := lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(fmt.Sprintf("Tool: %s", m.tools[m.toolIdx]))
    header := fmt.Sprintf("%s • %s  (Ctrl+T next tool)\n", title, tool)
    instructions := "TAB to switch fields • ENTER to toggle/build • +/- to adjust • ESC to cancel"
    header += lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(instructions) + "\n\n"
    // Render text inputs in sorted order.
    keys := make([]string, 0, len(m.strInputs))
    for k := range m.strInputs {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    content := header
    for _, k := range keys {
        ti := m.strInputs[k]
        label := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(strings.Title(k) + ": ")
        content += label + ti.View() + "\n"
    }
    content += "\n" + m.list.View()
    // Wrap in a rounded border with padding.
    style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1)
    return style.Render(content)
}

// FinalCommand returns the constructed command after the model exits.
func (m actionModel) FinalCommand() string { return m.final }

// actionItemDelegate handles rendering of list items for actionModel. It displays
// current values for booleans, integers, floats and enums with colour.
type actionItemDelegate struct{}

func (d actionItemDelegate) Height() int                             { return 1 }
func (d actionItemDelegate) Spacing() int                            { return 0 }
func (d actionItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d actionItemDelegate) Render(w io.Writer, m list.Model, idx int, listItem list.Item) {
    // Colourful prefix depending on selection.
    var prefix string
    if idx == m.Index() {
        prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("> ")
    } else {
        prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  ")
    }
    switch it := listItem.(type) {
    case boolFieldItem:
        var stateStr string
        if it.val != nil && *it.val {
            stateStr = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("[x]")
        } else {
            stateStr = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("[ ]")
        }
        labelStr := lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Render(it.label)
        fmt.Fprintf(w, "%s%s %s\n", prefix, stateStr, labelStr)
    case intFieldItem:
        val := 0
        if it.val != nil {
            val = *it.val
        }
        valStr := lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Render(fmt.Sprintf("[%d]", val))
        labelStr := lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Render(it.label)
        fmt.Fprintf(w, "%s%s %s\n", prefix, valStr, labelStr)
    case floatFieldItem:
        val := 0.0
        if it.val != nil {
            val = *it.val
        }
        valStr := lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Render(fmt.Sprintf("[%.1f]", val))
        labelStr := lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Render(it.label)
        fmt.Fprintf(w, "%s%s %s\n", prefix, valStr, labelStr)
    case enumFieldItem:
        choice := ""
        if it.idx != nil && *it.idx < len(it.choices) {
            choice = it.choices[*it.idx]
        }
        choiceStr := lipgloss.NewStyle().Foreground(lipgloss.Color("198")).Render(fmt.Sprintf("[%s]", choice))
        labelStr := lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Render(it.label)
        fmt.Fprintf(w, "%s%s %s\n", prefix, choiceStr, labelStr)
    case staticItem:
        labelStr := lipgloss.NewStyle().Foreground(lipgloss.Color("198")).Bold(true).Render(it.label)
        fmt.Fprintf(w, "%s%s\n", prefix, labelStr)
    default:
        fmt.Fprintf(w, "%s%v\n", prefix, listItem)
    }
}