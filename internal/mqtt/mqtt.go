package mqtt

import (
	"fmt"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

// Publish connects to an MQTT broker, publishes a message to the given
// topic, and disconnects. Each invocation creates a fresh connection —
// simple and stateless, matching the webhook pattern.
func Publish(broker, clientID, topic, message string, qos byte, retain bool, username, password string) error {
	opts := pahomqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID(clientID).
		SetConnectTimeout(5 * time.Second)

	if username != "" {
		opts.SetUsername(username)
	}
	if password != "" {
		opts.SetPassword(password)
	}

	client := pahomqtt.NewClient(opts)
	tok := client.Connect()
	if !tok.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("mqtt: connect timeout")
	}
	if tok.Error() != nil {
		return fmt.Errorf("mqtt: connect: %w", tok.Error())
	}
	defer client.Disconnect(250)

	pub := client.Publish(topic, qos, retain, message)
	if !pub.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("mqtt: publish timeout")
	}
	if pub.Error() != nil {
		return fmt.Errorf("mqtt: publish: %w", pub.Error())
	}
	return nil
}
