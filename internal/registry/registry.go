package registry

import (
    "io"
    "os"

    yaml "gopkg.in/yaml.v3"
)

// Field defines a single input field in an action template.
type Field struct {
    Key         string      `yaml:"key"`
    Type        string      `yaml:"type"`
    Label       string      `yaml:"label"`
    Placeholder string      `yaml:"placeholder"`
    Default     interface{} `yaml:"default"`
    Choices     []string    `yaml:"choices"`
    Required    bool        `yaml:"required"`
    Min         *float64    `yaml:"min"`
    Max         *float64    `yaml:"max"`
    ShowIf      string      `yaml:"showIf"`
    Entry       string      `yaml:"entry"`
}

// Action defines a single command-building action.
type Action struct {
    ID         string            `yaml:"id"`
    Title      string            `yaml:"title"`
    Synonyms   []string          `yaml:"synonyms"`
    Candidates []string          `yaml:"candidates"`
    Template   map[string]string `yaml:"template"`
    Fields     []Field           `yaml:"fields"`
}

// Registry holds a collection of actions loaded from YAML.
type Registry struct {
    Actions []Action `yaml:"actions"`
}

// Load reads a registry from the given YAML file path.
func Load(path string) (*Registry, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()
    return Parse(f)
}

// Parse decodes a registry from an io.Reader.
func Parse(r io.Reader) (*Registry, error) {
    var reg Registry
    dec := yaml.NewDecoder(r)
    if err := dec.Decode(&reg); err != nil {
        return nil, err
    }
    return &reg, nil
}