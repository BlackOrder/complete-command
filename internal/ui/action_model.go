package ui

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/BlackOrder/complete-command/internal/config"
	"github.com/BlackOrder/complete-command/internal/detect"
	"github.com/BlackOrder/complete-command/internal/integration"
	"github.com/BlackOrder/complete-command/internal/registry"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// actionModel is a generic form for building commands defined in the registry.
// It dynamically constructs input fields and list items based on the action's
// field definitions. Supported field types include string, path, bool, int,
// float, enum, and multi. On completion the model produces a shell command
// constructed from the selected tool and populated field values.
//
// The model uses TAB to cycle between text inputs and the list of option
// toggles. Arrow keys left/right switch between candidate tools. The list
// displays boolean, numeric and enum fields with their current values and a
// "Build & Insert" entry to finalize the command.
//
// Preferences for a selected tool are persisted using the provided config
// pointer and action ID.
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

	// shellMsg holds a transient message about shell integration installation/uninstallation.
	shellMsg string
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
		// Set default label to key if none provided.
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
			// Focus the first text input by default later.
			strInputs[f.Key] = &ti
		case "bool":
			// Determine default: bool fields default to false unless explicitly set true.
			defBool := false
			if b, ok := f.Default.(bool); ok {
				defBool = b
			}
			val := new(bool)
			*val = defBool
			boolItems = append(boolItems, boolFieldItem{key: f.Key, label: label, val: val})
		case "int":
			// Convert default to int if provided. YAML unmarshals numbers as int or float64.
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
			// Convert default to float64 if provided.
			var defF float64
			if f.Default != nil {
				switch v := f.Default.(type) {
				case float32:
					defF = float64(v)
				case float64:
					defF = v
				case int:
					defF = float64(v)
				case int64:
					defF = float64(v)
				}
			}
			val := new(float64)
			*val = defF
			floatItems = append(floatItems, floatFieldItem{key: f.Key, label: label, val: val, min: f.Min, max: f.Max})
		case "enum":
			// Determine default index.
			idx := 0
			if def, ok := f.Default.(string); ok {
				for i, c := range f.Choices {
					if c == def {
						idx = i
						break
					}
				}
			}
			indexPtr := new(int)
			*indexPtr = idx
			enumItems = append(enumItems, enumFieldItem{key: f.Key, label: label, choices: f.Choices, idx: indexPtr})
		default:
			// Unsupported type: skip.
		}
	}
	// Build list items for bool/int/float/enum plus the final build item.
	var listItems []list.Item
	for _, b := range boolItems {
		listItems = append(listItems, b)
	}
	for _, i := range intItems {
		listItems = append(listItems, i)
	}
	for _, f := range floatItems {
		listItems = append(listItems, f)
	}
	for _, e := range enumItems {
		listItems = append(listItems, e)
	}
	listItems = append(listItems, staticItem{label: "Build & Insert"})
	l := list.New(listItems, actionItemDelegate{}, 0, 0)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Title = action.Title + " options"
	// Focus the first text input if any.
	firstFocused := false
	for _, ti := range strInputs {
		if !firstFocused {
			ti.Focus()
			firstFocused = true
		}
	}
	return actionModel{
		action:     action,
		tools:      available,
		toolIdx:    0,
		strInputs:  strInputs,
		boolItems:  boolItems,
		intItems:   intItems,
		floatItems: floatItems,
		enumItems:  enumItems,
		list:       l,
		cfg:        cfg,
		prefKey:    action.ID,
	}
}

// Init returns nil; no special initialization required.
func (m actionModel) Init() tea.Cmd { return nil }

// Update processes incoming messages, updating focused inputs, toggles and selection.
func (m actionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "ctrl+i":
			if txt, err := integration.ToggleShellIntegration(); err == nil {
				m.shellMsg = txt
			} else {
				m.shellMsg = "Integration error: " + err.Error()
			}
			return m, nil
		case "ctrl+t":
			// Cycle to next available tool.
			if len(m.tools) > 1 {
				m.toolIdx = (m.toolIdx + 1) % len(m.tools)
			}
			return m, nil
		case "tab":
			// Cycle focus between text inputs and list. Determine current focus.
			// If any text input is focused, move to the next input; otherwise focus list.
			focusedKey := ""
			for k, ti := range m.strInputs {
				if ti.Focused() {
					focusedKey = k
					break
				}
			}
			if focusedKey != "" {
				// Blur current and focus next input. Determine order by map iteration; unpredictable but acceptable.
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
					// Focus the list.
					// Always select the first item when focusing the list.
					m.list.Select(0)
				}
			} else {
				// If list is focused, cycle back to first input.
				// Focus the first available text input.
				for _, ti := range m.strInputs {
					ti.Focus()
					break
				}
			}
		case "left":
			// Left/right are no longer used for tool selection; ignore.
		case "right":
			// Left/right are no longer used for tool selection; ignore.
		case "+":
			// If a numeric item is selected, increment it.
			idx := m.list.Index()
			if idx >= 0 && idx < len(m.boolItems)+len(m.intItems)+len(m.floatItems)+len(m.enumItems) {
				offset := len(m.boolItems)
				// int items
				if idx >= offset && idx < offset+len(m.intItems) {
					intIdx := idx - offset
					// increment the integer value
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
			// Decrease numeric values if possible.
			idx := m.list.Index()
			if idx >= 0 && idx < len(m.boolItems)+len(m.intItems)+len(m.floatItems)+len(m.enumItems) {
				offset := len(m.boolItems)
				if idx >= offset && idx < offset+len(m.intItems) {
					intIdx := idx - offset
					// apply min if defined
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
						// apply min for floats
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
			// If a text input is focused, pressing enter builds and inserts immediately.
			for _, ti := range m.strInputs {
				if ti.Focused() {
					// Persist preference
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
				// Build boundaries to locate which item.
				limit := len(m.boolItems)
				if idx < limit {
					// toggle bool
					*m.boolItems[idx].val = !*m.boolItems[idx].val
				} else {
					idx -= limit
					limit = len(m.intItems)
					if idx < limit {
						// numeric item; pressing enter does nothing
					} else {
						idx -= limit
						limit = len(m.floatItems)
						if idx < limit {
							// float item; no enter action
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
								// Persist preference
								if m.cfg != nil && m.prefKey != "" {
									m.cfg.SetPreference(m.prefKey, m.tools[m.toolIdx])
									_ = config.Save(m.cfg)
								}
								// Build the command
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

// buildCommand constructs the final shell command by populating the selected
// template for the chosen tool with the values captured in the form.
func (m actionModel) buildCommand() string {
	tool := m.tools[m.toolIdx]
	tmpl, ok := m.action.Template[tool]
	if !ok {
		// If no template for selected tool, fall back to first available.
		for _, t := range m.tools {
			if tmp, ok2 := m.action.Template[t]; ok2 {
				tmpl = tmp
				break
			}
		}
	}
	vals := make(map[string]interface{})
	// Gather values from text inputs.
	for key, ti := range m.strInputs {
		vals[key] = ti.Value()
	}
	// Booleans
	for _, b := range m.boolItems {
		vals[b.key] = *b.val
	}
	// Ints
	for _, i := range m.intItems {
		vals[i.key] = *i.val
	}
	// Floats
	for _, f := range m.floatItems {
		vals[f.key] = *f.val
	}
	// Enums
	for _, e := range m.enumItems {
		if e.idx != nil && len(e.choices) > 0 {
			vals[e.key] = e.choices[*e.idx]
		}
	}
	// Replace placeholders in the template.
	// Use a builder to construct the command string.
	result := ""
	// We'll parse the template by scanning for {{...}} directives.
	for {
		start := strings.Index(tmpl, "{{")
		if start == -1 {
			result += tmpl
			break
		}
		// Append prefix before the placeholder.
		result += tmpl[:start]
		tmpl = tmpl[start+2:]
		end := strings.Index(tmpl, "}}")
		if end == -1 {
			// unmatched braces; append rest and break
			result += "{{" + tmpl
			break
		}
		placeholder := strings.TrimSpace(tmpl[:end])
		tmpl = tmpl[end+2:]
		// Process placeholder
		// conditional: key?value
		if qIndex := strings.Index(placeholder, "?"); qIndex > 0 {
			key := strings.TrimSpace(placeholder[:qIndex])
			cond := placeholder[qIndex+1:]
			if val, ok := vals[key]; ok {
				include := false
				switch v := val.(type) {
				case bool:
					include = v
				case string:
					include = v != ""
				case int:
					include = v != 0
				case float64:
					include = v != 0
				default:
					include = val != nil
				}
				if include {
					// If cond contains %%s or %%d, format with value
					if strings.Contains(cond, "%") {
						switch v := val.(type) {
						case string:
							result += fmt.Sprintf(cond, v)
						case int:
							result += fmt.Sprintf(cond, v)
						case float64:
							// Format float with no trailing zeros by converting to string
							result += fmt.Sprintf(cond, v)
						default:
							result += fmt.Sprintf(cond, v)
						}
					} else {
						result += cond
					}
				}
			}
		} else if bar := strings.Index(placeholder, "|"); bar > 0 {
			key := strings.TrimSpace(placeholder[:bar])
			format := placeholder[bar+1:]
			if val, ok := vals[key]; ok {
				// Multi values are stored as comma or space separated strings.
				switch v := val.(type) {
				case []string:
					// apply format for each entry
					for i, entry := range v {
						if strings.Contains(format, "%") {
							result += fmt.Sprintf(format, entry)
						} else {
							result += format
						}
						if i < len(v)-1 {
							result += " "
						}
					}
				case string:
					if v != "" {
						if strings.Contains(format, "%") {
							result += fmt.Sprintf(format, v)
						} else {
							result += format
						}
					}
				case int:
					if strings.Contains(format, "%") {
						result += fmt.Sprintf(format, v)
					} else {
						result += format
					}
				case float64:
					if strings.Contains(format, "%") {
						result += fmt.Sprintf(format, v)
					} else {
						result += format
					}
				}
			}
		} else {
			// Simple replacement: output value as string.
			key := placeholder
			if val, ok := vals[key]; ok {
				switch v := val.(type) {
				case string:
					result += v
				case int:
					result += strconv.Itoa(v)
				case float64:
					// Convert float to string without trailing zeros.
					result += fmt.Sprintf("%g", v)
				default:
					// For other types, use fmt.Sprint
					result += fmt.Sprint(v)
				}
			}
		}
	}
	// Collapse any duplicate spaces.
	fields := strings.Fields(result)
	return strings.Join(fields, " ")
}

// View renders the current form state.
func (m actionModel) View() string {
	// Header with action title and current tool.
	s := fmt.Sprintf("%s • Tool: %s  (Ctrl+T next tool)\n", m.action.Title, m.tools[m.toolIdx])
	s += "TAB to switch fields • ENTER to toggle/build • +/- to adjust • Ctrl+I install/uninstall • ESC to cancel\n"
	if m.shellMsg != "" {
		s += m.shellMsg + "\n"
	}
	s += "\n"
	// Render text inputs in an arbitrary but deterministic order (sorted by key).
	// To ensure stable ordering across runs, gather keys and sort them.
	keys := make([]string, 0, len(m.strInputs))
	for k := range m.strInputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		ti := m.strInputs[k]
		s += strings.Title(k) + ": " + ti.View() + "\n"
	}
	s += "\n" + m.list.View()
	return s
}

// FinalCommand returns the constructed command after the model exits.
func (m actionModel) FinalCommand() string { return m.final }

// actionItemDelegate handles rendering of list items for actionModel. It displays
// current values for booleans, integers, floats and enums.
type actionItemDelegate struct{}

func (d actionItemDelegate) Height() int                             { return 1 }
func (d actionItemDelegate) Spacing() int                            { return 0 }
func (d actionItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d actionItemDelegate) Render(w io.Writer, m list.Model, idx int, listItem list.Item) {
	prefix := "  "
	if idx == m.Index() {
		prefix = "> "
	}
	switch it := listItem.(type) {
	case boolFieldItem:
		state := "[ ]"
		if it.val != nil && *it.val {
			state = "[x]"
		}
		fmt.Fprintf(w, "%s%s %s\n", prefix, state, it.label)
	case intFieldItem:
		val := 0
		if it.val != nil {
			val = *it.val
		}
		fmt.Fprintf(w, "%s[%d] %s\n", prefix, val, it.label)
	case floatFieldItem:
		val := 0.0
		if it.val != nil {
			val = *it.val
		}
		// Format float with up to one decimal place for display.
		fmt.Fprintf(w, "%s[%.1f] %s\n", prefix, val, it.label)
	case enumFieldItem:
		choice := ""
		if it.idx != nil && *it.idx < len(it.choices) {
			choice = it.choices[*it.idx]
		}
		fmt.Fprintf(w, "%s[%s] %s\n", prefix, choice, it.label)
	case staticItem:
		fmt.Fprintf(w, "%s%s\n", prefix, it.label)
	default:
		fmt.Fprintf(w, "%s%v\n", prefix, it)
	}
}
