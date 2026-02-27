package mqtt

import "testing"

func TestPublishBadBroker(t *testing.T) {
	// Connecting to a non-existent broker should return a connect error.
	err := Publish("tcp://127.0.0.1:19999", "test-client", "test/topic", "hello", 0, false, "", "")
	if err == nil {
		t.Fatal("expected error for unreachable broker")
	}
}

func TestPublishBadScheme(t *testing.T) {
	// A completely invalid broker URL should fail.
	err := Publish("not-a-url", "test-client", "test/topic", "hello", 0, false, "", "")
	if err == nil {
		t.Fatal("expected error for invalid broker URL")
	}
}
