package protocol

import (
	"fmt"
	"strings"
	"testing"
)

func TestMessageRegistryInvariants(t *testing.T) {
	t.Parallel()

	names := make(map[string]struct{}, len(messages))
	identities := make(map[string]struct{}, len(messages))
	for _, message := range messages {
		if message.Name == "" {
			t.Fatal("message with empty name")
		}
		if message.ID <= 0 {
			t.Errorf("%s ID = %d, want positive", message.Name, message.ID)
		}
		if _, duplicate := names[message.Name]; duplicate {
			t.Errorf("duplicate message name %q", message.Name)
		}
		names[message.Name] = struct{}{}

		identity := fmt.Sprintf("%d:%d", message.Direction, message.ID)
		if _, duplicate := identities[identity]; duplicate {
			t.Errorf("duplicate %s message ID %d", message.Direction, message.ID)
		}
		identities[identity] = struct{}{}

		wantPrefix := "Out"
		if message.Direction == ServerToClient {
			wantPrefix = "In"
		} else if message.Direction != ClientToServer {
			t.Errorf("%s has invalid direction %d", message.Name, message.Direction)
		}
		if !strings.HasPrefix(message.Name, wantPrefix) {
			t.Errorf("%s direction = %s, want %s name prefix", message.Name, message.Direction, wantPrefix)
		}

		got, ok := Lookup(message.Direction, message.ID)
		if !ok || got != message {
			t.Errorf("Lookup(%s, %d) = %#v, %t; want %#v, true", message.Direction, message.ID, got, ok, message)
		}
	}
}

func TestMessagesReturnsCopy(t *testing.T) {
	t.Parallel()

	got := Messages()
	got[0] = Message{}
	if messages[0] == (Message{}) {
		t.Fatal("Messages exposed mutable registry storage")
	}
}
