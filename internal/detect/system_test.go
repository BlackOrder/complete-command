package detect

import (
	"runtime"
	"testing"
)

func TestGetOSInfo(t *testing.T) {
	info := GetOSInfo()
	
	if info == nil {
		t.Fatal("GetOSInfo returned nil")
	}
	
	if info.OS != runtime.GOOS {
		t.Errorf("Expected OS %s, got %s", runtime.GOOS, info.OS)
	}
	
	if info.Arch != runtime.GOARCH {
		t.Errorf("Expected arch %s, got %s", runtime.GOARCH, info.Arch)
	}
	
	// OS should be one of the known values
	validOS := map[string]bool{
		"linux":   true,
		"darwin":  true,
		"windows": true,
	}
	
	if !validOS[info.OS] {
		t.Errorf("Unexpected OS: %s", info.OS)
	}
}

func TestParseSemanticVersion(t *testing.T) {
	tests := []struct {
		input    string
		major    int
		minor    int
		patch    int
		expected bool
	}{
		{"1.2.3", 1, 2, 3, true},
		{"10.15.7", 10, 15, 7, true},
		{"2.1", 2, 1, 0, true},
		{"3.0.1-ubuntu", 3, 0, 1, true},
		{"invalid", 0, 0, 0, false},
		{"1", 0, 0, 0, false},
	}
	
	for _, test := range tests {
		major, minor, patch, ok := parseSemanticVersion(test.input)
		
		if ok != test.expected {
			t.Errorf("parseSemanticVersion(%s): expected ok=%v, got %v", 
				test.input, test.expected, ok)
			continue
		}
		
		if test.expected {
			if major != test.major || minor != test.minor || patch != test.patch {
				t.Errorf("parseSemanticVersion(%s): expected %d.%d.%d, got %d.%d.%d",
					test.input, test.major, test.minor, test.patch, major, minor, patch)
			}
		}
	}
}

func TestParseVersionOutput(t *testing.T) {
	tests := []struct {
		tool     string
		output   string
		expected string
	}{
		{"grep", "grep (GNU grep) 3.7", "3.7"},
		{"curl", "curl 7.68.0 (x86_64-pc-linux-gnu)", "7.68.0"},
		{"git", "git version 2.25.1", "2.25.1"},
		{"node", "v16.14.0", "16.14.0"},
		{"python", "Python 3.8.10", "3.8.10"},
		{"invalid", "no version here", ""},
	}
	
	for _, test := range tests {
		result := parseVersionOutput(test.tool, test.output)
		if result != test.expected {
			t.Errorf("parseVersionOutput(%s, %q): expected %q, got %q",
				test.tool, test.output, test.expected, result)
		}
	}
}

func TestGetToolVersion(t *testing.T) {
	// Test with a tool that should exist on most systems
	if Has("grep") {
		version, err := GetToolVersion("grep")
		if err != nil {
			t.Errorf("GetToolVersion(grep) failed: %v", err)
		}
		
		if version == nil {
			t.Error("GetToolVersion(grep) returned nil version")
		} else {
			if version.Name != "grep" {
				t.Errorf("Expected name 'grep', got %s", version.Name)
			}
			
			if version.Version == "" {
				t.Error("Version string is empty")
			}
		}
	}
	
	// Test with non-existent tool
	_, err := GetToolVersion("definitely_nonexistent_tool")
	if err == nil {
		t.Error("Expected error for nonexistent tool")
	}
}

func TestParseOSRelease(t *testing.T) {
	testContent := `NAME="Ubuntu"
VERSION="20.04.3 LTS (Focal Fossa)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 20.04.3 LTS"
VERSION_ID="20.04"
HOME_URL="https://www.ubuntu.com/"
SUPPORT_URL="https://help.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
VERSION_CODENAME=focal
UBUNTU_CODENAME=focal`

	id, version := parseOSRelease(testContent)
	
	if id != "ubuntu" {
		t.Errorf("Expected ID 'ubuntu', got '%s'", id)
	}
	
	if version != "20.04" {
		t.Errorf("Expected version '20.04', got '%s'", version)
	}
}

func TestParseLSBRelease(t *testing.T) {
	testContent := `DISTRIB_ID=Ubuntu
DISTRIB_RELEASE=18.04
DISTRIB_CODENAME=bionic
DISTRIB_DESCRIPTION="Ubuntu 18.04.6 LTS"`

	id, version := parseLSBRelease(testContent)
	
	if id != "ubuntu" {
		t.Errorf("Expected ID 'ubuntu', got '%s'", id)
	}
	
	if version != "18.04" {
		t.Errorf("Expected version '18.04', got '%s'", version)
	}
}