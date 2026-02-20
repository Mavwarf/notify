package idle

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
)

var hIDIdleTimeRe = regexp.MustCompile(`"HIDIdleTime"\s*=\s*(\d+)`)

// IdleSeconds returns the number of seconds since the last keyboard or
// mouse input on macOS, by parsing HIDIdleTime from ioreg.
func IdleSeconds() (float64, error) {
	out, err := exec.Command("ioreg", "-c", "IOHIDSystem", "-d", "4").Output()
	if err != nil {
		return 0, fmt.Errorf("ioreg: %w", err)
	}

	m := hIDIdleTimeRe.FindSubmatch(out)
	if m == nil {
		return 0, fmt.Errorf("HIDIdleTime not found in ioreg output")
	}

	ns, err := strconv.ParseUint(string(m[1]), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing HIDIdleTime: %w", err)
	}

	return float64(ns) / 1e9, nil
}
