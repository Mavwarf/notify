//go:build windows

package desktop

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

const protocolKey = `Software\Classes\notify`

// RegisterProtocol registers the notify:// URI protocol handler in the
// current user's registry. When Windows activates a notify:// URI, it
// launches exePath with --protocol and the URI as arguments.
func RegisterProtocol(exePath string) error {
	// Create notify key with URL Protocol marker.
	k, _, err := registry.CreateKey(registry.CURRENT_USER, protocolKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create protocol key: %w", err)
	}
	if err := k.SetStringValue("", "URL:notify Protocol"); err != nil {
		k.Close()
		return fmt.Errorf("set description: %w", err)
	}
	if err := k.SetStringValue("URL Protocol", ""); err != nil {
		k.Close()
		return fmt.Errorf("set URL Protocol: %w", err)
	}
	k.Close()

	// Create shell\open\command key with launch command.
	cmdKey, _, err := registry.CreateKey(registry.CURRENT_USER, protocolKey+`\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create command key: %w", err)
	}
	defer cmdKey.Close()

	cmd := fmt.Sprintf(`"%s" --protocol "%%1"`, exePath)
	if err := cmdKey.SetStringValue("", cmd); err != nil {
		return fmt.Errorf("set command: %w", err)
	}
	return nil
}

// UnregisterProtocol removes the notify:// URI protocol handler from
// the current user's registry.
func UnregisterProtocol() error {
	return registry.DeleteKey(registry.CURRENT_USER, protocolKey)
}

// IsProtocolRegistered checks whether the notify:// protocol handler
// is registered in the current user's registry.
func IsProtocolRegistered() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, protocolKey+`\shell\open\command`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	k.Close()
	return true
}
