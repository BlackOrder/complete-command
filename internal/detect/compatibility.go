package detect

import (
	"fmt"
	"strings"
)

// ToolCompatibility holds information about tool compatibility across platforms
type ToolCompatibility struct {
	Name           string
	OSSupport      map[string]bool    // OS -> supported
	VersionRanges  map[string]string  // OS -> min version
	FlagVariations map[string]string  // standard flag -> OS-specific flag
	Alternatives   []string           // alternative tools
}

// CompatibilityMatrix holds compatibility information for all tools
type CompatibilityMatrix struct {
	tools map[string]*ToolCompatibility
}

// NewCompatibilityMatrix creates and initializes the compatibility matrix
func NewCompatibilityMatrix() *CompatibilityMatrix {
	cm := &CompatibilityMatrix{
		tools: make(map[string]*ToolCompatibility),
	}
	
	// Initialize with known tool compatibility information
	cm.initializeKnownTools()
	
	return cm
}

// initializeKnownTools populates the matrix with known tool compatibility
func (cm *CompatibilityMatrix) initializeKnownTools() {
	// Grep variants
	cm.tools["grep"] = &ToolCompatibility{
		Name: "grep",
		OSSupport: map[string]bool{
			"linux":   true,
			"darwin":  true,
			"windows": true, // via WSL or Git Bash
		},
		VersionRanges: map[string]string{
			"linux":  "2.0+",
			"darwin": "2.5+", // BSD grep has different features
		},
		FlagVariations: map[string]string{
			"--color=auto":     "--color=auto", // GNU grep
			"--color=always":   "--color=always",
			"--exclude-dir":    "--exclude-dir", // GNU grep only
			"--include":        "--include",     // GNU grep only
		},
		Alternatives: []string{"rg", "ag", "ack"},
	}
	
	cm.tools["rg"] = &ToolCompatibility{
		Name: "rg",
		OSSupport: map[string]bool{
			"linux":   true,
			"darwin":  true,
			"windows": true,
		},
		VersionRanges: map[string]string{
			"linux":   "0.10+",
			"darwin":  "0.10+",
			"windows": "0.10+",
		},
		FlagVariations: map[string]string{
			"--color=auto":   "--color=auto",
			"--color=always": "--color=always",
			"--hidden":       "--hidden",
			"--glob":         "--glob",
		},
		Alternatives: []string{"grep", "ag"},
	}
	
	// Find tools
	cm.tools["find"] = &ToolCompatibility{
		Name: "find",
		OSSupport: map[string]bool{
			"linux":   true,
			"darwin":  true,
			"windows": false, // Not natively available
		},
		FlagVariations: map[string]string{
			"-executable": "-executable", // GNU find
			"-perm":       "-perm",
			"-newer":      "-newer",
		},
		Alternatives: []string{"fd"},
	}
	
	cm.tools["fd"] = &ToolCompatibility{
		Name: "fd",
		OSSupport: map[string]bool{
			"linux":   true,
			"darwin":  true,
			"windows": true,
		},
		Alternatives: []string{"find"},
	}
	
	// Network tools
	cm.tools["ping"] = &ToolCompatibility{
		Name: "ping",
		OSSupport: map[string]bool{
			"linux":   true,
			"darwin":  true,
			"windows": true,
		},
		FlagVariations: map[string]string{
			"-c": "-c", // count (Unix)
			"-n": "-n", // count (Windows)
			"-i": "-i", // interval (Unix)
			"-w": "-w", // timeout (varies by OS)
		},
	}
	
	cm.tools["curl"] = &ToolCompatibility{
		Name: "curl",
		OSSupport: map[string]bool{
			"linux":   true,
			"darwin":  true,
			"windows": true,
		},
		VersionRanges: map[string]string{
			"linux":   "7.0+",
			"darwin":  "7.0+",
			"windows": "7.0+",
		},
		Alternatives: []string{"wget", "http"},
	}
	
	// Archive tools
	cm.tools["tar"] = &ToolCompatibility{
		Name: "tar",
		OSSupport: map[string]bool{
			"linux":   true,
			"darwin":  true,
			"windows": false, // Not natively available
		},
		FlagVariations: map[string]string{
			"--exclude":     "--exclude",     // GNU tar
			"--wildcards":   "--wildcards",   // GNU tar
			"--transform":   "--transform",   // GNU tar
		},
	}
	
	cm.tools["7z"] = &ToolCompatibility{
		Name: "7z",
		OSSupport: map[string]bool{
			"linux":   true,
			"darwin":  true,
			"windows": true,
		},
		Alternatives: []string{"unzip", "tar"},
	}
	
	// Package managers
	cm.tools["apt"] = &ToolCompatibility{
		Name: "apt",
		OSSupport: map[string]bool{
			"linux": true, // Debian/Ubuntu only
		},
		Alternatives: []string{"apt-get", "aptitude"},
	}
	
	cm.tools["dnf"] = &ToolCompatibility{
		Name: "dnf",
		OSSupport: map[string]bool{
			"linux": true, // Fedora/RHEL 8+
		},
		Alternatives: []string{"yum"},
	}
	
	cm.tools["pacman"] = &ToolCompatibility{
		Name: "pacman",
		OSSupport: map[string]bool{
			"linux": true, // Arch Linux
		},
	}
	
	cm.tools["brew"] = &ToolCompatibility{
		Name: "brew",
		OSSupport: map[string]bool{
			"darwin": true,
			"linux":  true, // Homebrew on Linux
		},
	}
	
	// System monitoring
	cm.tools["htop"] = &ToolCompatibility{
		Name: "htop",
		OSSupport: map[string]bool{
			"linux":   true,
			"darwin":  true,
			"windows": false,
		},
		Alternatives: []string{"top"},
	}
	
	cm.tools["ps"] = &ToolCompatibility{
		Name: "ps",
		OSSupport: map[string]bool{
			"linux":   true,
			"darwin":  true,
			"windows": false,
		},
		FlagVariations: map[string]string{
			"aux":   "aux",   // BSD style
			"-ef":   "-ef",   // System V style
			"--forest": "--forest", // GNU ps only
		},
	}
}

// GetCompatibility returns compatibility information for a tool
func (cm *CompatibilityMatrix) GetCompatibility(tool string) *ToolCompatibility {
	return cm.tools[tool]
}

// GetBestTool finds the best available tool from a list of candidates
func (cm *CompatibilityMatrix) GetBestTool(candidates []string, osInfo *OSInfo) (string, error) {
	for _, tool := range candidates {
		if Has(tool) {
			if compat := cm.GetCompatibility(tool); compat != nil {
				if supported, ok := compat.OSSupport[osInfo.OS]; ok && supported {
					// Check version compatibility if specified
					if minVersion, ok := compat.VersionRanges[osInfo.OS]; ok && minVersion != "" {
						if toolVersion, err := GetToolVersion(tool); err == nil {
							if isVersionCompatible(toolVersion.Version, minVersion) {
								return tool, nil
							}
							continue // Try next candidate
						}
					}
					return tool, nil
				}
			} else {
				// No compatibility info, assume it works if it exists
				return tool, nil
			}
		}
	}
	
	return "", fmt.Errorf("no compatible tool found from candidates: %v", candidates)
}

// GetAlternatives returns alternative tools for a given tool
func (cm *CompatibilityMatrix) GetAlternatives(tool string) []string {
	if compat := cm.GetCompatibility(tool); compat != nil {
		return compat.Alternatives
	}
	return nil
}

// AdaptFlags adapts flags for the current OS and tool version
func (cm *CompatibilityMatrix) AdaptFlags(tool string, flags []string, osInfo *OSInfo) []string {
	compat := cm.GetCompatibility(tool)
	if compat == nil {
		return flags // No adaptation info available
	}
	
	adaptedFlags := make([]string, 0, len(flags))
	
	for _, flag := range flags {
		// Check if this flag has OS-specific variations
		if adaptation, ok := compat.FlagVariations[flag]; ok {
			// Additional logic could be added here to check tool version
			// and OS-specific availability
			adaptedFlags = append(adaptedFlags, adaptation)
		} else {
			adaptedFlags = append(adaptedFlags, flag)
		}
	}
	
	return adaptedFlags
}

// isVersionCompatible checks if a version meets the minimum requirement
func isVersionCompatible(currentVersion, minVersion string) bool {
	// Remove the '+' suffix from minVersion
	minVersion = strings.TrimSuffix(minVersion, "+")
	
	// Parse both versions
	currentMajor, currentMinor, currentPatch, currentOk := parseSemanticVersion(currentVersion)
	minMajor, minMinor, minPatch, minOk := parseSemanticVersion(minVersion)
	
	if !currentOk || !minOk {
		// If we can't parse versions, assume compatibility
		return true
	}
	
	// Compare major.minor.patch
	if currentMajor > minMajor {
		return true
	}
	if currentMajor < minMajor {
		return false
	}
	
	// Major versions equal, check minor
	if currentMinor > minMinor {
		return true
	}
	if currentMinor < minMinor {
		return false
	}
	
	// Major and minor equal, check patch
	return currentPatch >= minPatch
}

// SuggestCommand suggests the best command variant for the current system
func (cm *CompatibilityMatrix) SuggestCommand(candidates []string, osInfo *OSInfo) (*CommandSuggestion, error) {
	bestTool, err := cm.GetBestTool(candidates, osInfo)
	if err != nil {
		return nil, err
	}
	
	suggestion := &CommandSuggestion{
		Tool:         bestTool,
		Alternatives: cm.GetAlternatives(bestTool),
		OSInfo:       osInfo,
	}
	
	// Get tool version if possible
	if version, err := GetToolVersion(bestTool); err == nil {
		suggestion.Version = version
	}
	
	return suggestion, nil
}

// CommandSuggestion contains information about the suggested command
type CommandSuggestion struct {
	Tool         string
	Version      *ToolVersion
	Alternatives []string
	OSInfo       *OSInfo
}