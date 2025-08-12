package integration

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

// ToggleShellIntegration installs or uninstalls shell integration for the
// complete‑command tool. It detects the current shell from the SHELL
// environment variable and modifies the appropriate rc file in the user's
// home directory. The integration binds Ctrl+G to run the tool and insert
// the resulting command into the current prompt. On installation, it adds
// a BEGIN/END marked section; on uninstallation, it removes that section.
// It returns a message indicating whether the integration was installed
// or removed. Errors during reading or writing the rc file are returned.
func ToggleShellIntegration(install *bool) (string, error) {
    shellPath := os.Getenv("SHELL")
    shell := filepath.Base(shellPath)
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    var rcFile string
    // Determine which rc file to modify based on the shell.
    switch shell {
    case "bash":
        rcFile = filepath.Join(home, ".bashrc")
    case "zsh":
        rcFile = filepath.Join(home, ".zshrc")
    case "fish":
        rcFile = filepath.Join(home, ".config", "fish", "config.fish")
    default:
        // Unknown shell: default to bash rc.
        rcFile = filepath.Join(home, ".bashrc")
    }
    // Ensure directory exists for fish.
    if shell == "fish" {
        if err := os.MkdirAll(filepath.Join(home, ".config", "fish"), 0o755); err != nil {
            return "", err
        }
    }
    const beginMarker = "# BEGIN complete-command integration"
    const endMarker = "# END complete-command integration"
    // Read existing rc file if it exists.
    data, _ := os.ReadFile(rcFile)
    content := string(data)
    beginIdx := strings.Index(content, beginMarker)
    endIdx := strings.Index(content, endMarker)
    if beginIdx >= 0 && endIdx > beginIdx && (install == nil || !*install) {
        // Integration exists; remove it to uninstall.
        before := content[:beginIdx]
        after := content[endIdx+len(endMarker):]
        // Trim following newline if present.
        after = strings.TrimPrefix(after, "\n")
        newContent := strings.TrimRight(before, "\n") + "\n" + strings.TrimLeft(after, "\n")
        if err := os.WriteFile(rcFile, []byte(newContent), 0o644); err != nil {
            return "", err
        }
        return "Shell integration removed. Reload your shell for changes to take effect.", nil
    }
    if install != nil && !*install {
        // Integration not found; nothing to uninstall.
        return "Shell integration is not installed.", nil
    }
    // Integration not found; install it.
    exePath, err := os.Executable()
    if err != nil {
        exePath = "complete-command"
    }
    var integrationSnippet string
    switch shell {
    case "bash":
        integrationSnippet = fmt.Sprintf(`%s
cmdcraft() {
  local out
  out="$("%s" "$@")" || return
  [[ -z "$out" ]] && return
  READLINE_LINE="$out"
  READLINE_POINT=${#READLINE_LINE}
}
bind -x '"\C-g":cmdcraft'
%s
`, beginMarker, exePath, endMarker)
    case "zsh":
        integrationSnippet = fmt.Sprintf(`%s
cmdcraft() {
  local out
  out="$("%s" "$@")" || return
  [[ -z "$out" ]] && return
  LBUFFER="$out"
  RBUFFER=""
  zle redisplay
}
zle -N cmdcraft
bindkey '^G' cmdcraft
%s
`, beginMarker, exePath, endMarker)
    case "fish":
        integrationSnippet = fmt.Sprintf(`%s
function cmdcraft
    set -l out (%s)
    or return
    if test -n "$out"
        commandline -r -- $out
    end
end
bind \cg cmdcraft
%s
`, beginMarker, exePath, endMarker)
    default:
        // Fallback uses bash-style snippet.
        integrationSnippet = fmt.Sprintf(`%s
cmdcraft() {
  local out
  out="$("%s" "$@")" || return
  [[ -z "$out" ]] && return
  READLINE_LINE="$out"
  READLINE_POINT=${#READLINE_LINE}
}
bind -x '"\C-g":cmdcraft'
%s
`, beginMarker, exePath, endMarker)
    }
    // Append integration snippet to rc file.
    var builder strings.Builder
    if len(content) > 0 {
        builder.WriteString(strings.TrimRight(content, "\n"))
        builder.WriteString("\n")
    }
    builder.WriteString(integrationSnippet)
    builder.WriteString("\n")
    if err := os.WriteFile(rcFile, []byte(builder.String()), 0o644); err != nil {
        return "", err
    }
    return "Shell integration installed. Reload your shell for changes to take effect.", nil
}