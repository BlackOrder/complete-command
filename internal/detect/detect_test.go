package detect

import "testing"

func TestHas(t *testing.T) {
    // At least one of sh or bash should exist on most systems used for testing.
    if !Has("sh") && !Has("bash") {
        t.Errorf("expected 'sh' or 'bash' to be present")
    }
    // A likely nonexistent command should not be present.
    if Has("unlikely_nonexistent_command_name") {
        t.Errorf("expected nonexistent command to be absent")
    }
}