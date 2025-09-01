package detect

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

// OSInfo holds information about the operating system
type OSInfo struct {
	OS      string // linux, darwin, windows
	Distro  string // ubuntu, centos, arch, etc. (Linux only)
	Version string // version string
	Arch    string // amd64, arm64, etc.
}

// ToolVersion holds information about a tool's version
type ToolVersion struct {
	Name    string
	Version string
	Major   int
	Minor   int
	Patch   int
}

// GetOSInfo detects the current operating system and distribution
func GetOSInfo() *OSInfo {
	info := &OSInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	switch info.OS {
	case "linux":
		info.Distro, info.Version = detectLinuxDistro()
	case "darwin":
		info.Version = detectMacOSVersion()
	case "windows":
		info.Version = detectWindowsVersion()
	}

	return info
}

// detectLinuxDistro attempts to detect the Linux distribution
func detectLinuxDistro() (string, string) {
	// Try /etc/os-release first (modern standard)
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		return parseOSRelease(string(data))
	}

	// Try /etc/lsb-release (Ubuntu/Debian)
	if data, err := os.ReadFile("/etc/lsb-release"); err == nil {
		return parseLSBRelease(string(data))
	}

	// Try legacy files
	distroFiles := map[string]string{
		"/etc/redhat-release": "redhat",
		"/etc/centos-release": "centos",
		"/etc/fedora-release": "fedora",
		"/etc/arch-release":   "arch",
		"/etc/debian_version": "debian",
	}

	for file, distro := range distroFiles {
		if _, err := os.Stat(file); err == nil {
			if data, err := os.ReadFile(file); err == nil {
				return distro, strings.TrimSpace(string(data))
			}
		}
	}

	return "unknown", ""
}

// parseOSRelease parses /etc/os-release format
func parseOSRelease(content string) (string, string) {
	var id, version string
	scanner := bufio.NewScanner(strings.NewReader(content))
	
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ID=") {
			id = strings.Trim(strings.TrimPrefix(line, "ID="), `"`)
		} else if strings.HasPrefix(line, "VERSION_ID=") {
			version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), `"`)
		}
	}
	
	return id, version
}

// parseLSBRelease parses /etc/lsb-release format
func parseLSBRelease(content string) (string, string) {
	var id, version string
	scanner := bufio.NewScanner(strings.NewReader(content))
	
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "DISTRIB_ID=") {
			id = strings.ToLower(strings.TrimPrefix(line, "DISTRIB_ID="))
		} else if strings.HasPrefix(line, "DISTRIB_RELEASE=") {
			version = strings.TrimPrefix(line, "DISTRIB_RELEASE=")
		}
	}
	
	return id, version
}

// detectMacOSVersion gets macOS version
func detectMacOSVersion() string {
	if output, err := exec.Command("sw_vers", "-productVersion").Output(); err == nil {
		return strings.TrimSpace(string(output))
	}
	return ""
}

// detectWindowsVersion gets Windows version
func detectWindowsVersion() string {
	if output, err := exec.Command("ver").Output(); err == nil {
		return strings.TrimSpace(string(output))
	}
	return ""
}

// GetToolVersion attempts to get the version of a specific tool
func GetToolVersion(tool string) (*ToolVersion, error) {
	if !Has(tool) {
		return nil, fmt.Errorf("tool %s not found", tool)
	}

	// Common version flags to try
	versionFlags := []string{"--version", "-version", "-V", "-v"}
	
	for _, flag := range versionFlags {
		output, err := exec.Command(tool, flag).Output()
		if err != nil {
			continue
		}
		
		version := parseVersionOutput(tool, string(output))
		if version != "" {
			tv := &ToolVersion{
				Name:    tool,
				Version: version,
			}
			
			// Parse semantic version if possible
			if major, minor, patch, ok := parseSemanticVersion(version); ok {
				tv.Major = major
				tv.Minor = minor
				tv.Patch = patch
			}
			
			return tv, nil
		}
	}
	
	return nil, fmt.Errorf("could not determine version for %s", tool)
}

// parseVersionOutput extracts version from command output
func parseVersionOutput(tool, output string) string {
	lines := strings.Split(output, "\n")
	
	// Version regex patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`v?(\d+\.\d+\.\d+)`),           // semantic version
		regexp.MustCompile(`version\s+v?(\d+\.\d+\.\d+)`), // "version X.Y.Z"
		regexp.MustCompile(`(\d+\.\d+)`),                  // major.minor
		regexp.MustCompile(tool + `\s+v?(\d+\.\d+\.\d+)`), // tool vX.Y.Z
	}
	
	for _, line := range lines {
		line = strings.ToLower(strings.TrimSpace(line))
		for _, pattern := range patterns {
			if matches := pattern.FindStringSubmatch(line); len(matches) > 1 {
				return matches[1]
			}
		}
	}
	
	return ""
}

// parseSemanticVersion parses a semantic version string
func parseSemanticVersion(version string) (major, minor, patch int, ok bool) {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return 0, 0, 0, false
	}
	
	var err error
	if major, err = parseInt(parts[0]); err != nil {
		return 0, 0, 0, false
	}
	
	if minor, err = parseInt(parts[1]); err != nil {
		return 0, 0, 0, false
	}
	
	if len(parts) >= 3 {
		// Handle patch versions that might have additional info (e.g., "2-ubuntu")
		patchStr := strings.Split(parts[2], "-")[0]
		if patch, err = parseInt(patchStr); err != nil {
			patch = 0
		}
	}
	
	return major, minor, patch, true
}

// parseInt safely parses an integer from string
func parseInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	
	// Remove any non-digit characters from the start
	for i, r := range s {
		if r >= '0' && r <= '9' {
			s = s[i:]
			break
		}
	}
	
	// Find first non-digit character and truncate
	for i, r := range s {
		if r < '0' || r > '9' {
			s = s[:i]
			break
		}
	}
	
	if s == "" {
		return 0, fmt.Errorf("no digits found")
	}
	
	result := 0
	for _, r := range s {
		result = result*10 + int(r-'0')
	}
	
	return result, nil
}

// SupportsFlag checks if a tool supports a specific flag
func SupportsFlag(tool, flag string) bool {
	if !Has(tool) {
		return false
	}
	
	// Try running the tool with --help to see if the flag is mentioned
	if output, err := exec.Command(tool, "--help").Output(); err == nil {
		return strings.Contains(string(output), flag)
	}
	
	// Fallback: try the flag with a safe operation (if applicable)
	// This is tool-specific and would need more sophisticated logic
	return false
}