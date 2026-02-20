package idle

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// IdleSeconds returns the number of seconds since the last keyboard or
// mouse input on Linux, using xprintidle (which returns milliseconds).
func IdleSeconds() (float64, error) {
	out, err := exec.Command("xprintidle").Output()
	if err != nil {
		return 0, fmt.Errorf("xprintidle: %w (is xprintidle installed?)", err)
	}

	ms, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing xprintidle output: %w", err)
	}

	return float64(ms) / 1000.0, nil
}
