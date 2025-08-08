package detect

import "os/exec"

// Has reports whether the given executable is available on the system's PATH.
func Has(bin string) bool {
    _, err := exec.LookPath(bin)
    return err == nil
}
