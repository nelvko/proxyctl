package redact

import (
	"errors"
	"strings"
	"testing"
)

func TestHumanURLsHideEveryCredentialLocation(t *testing.T) {
	source := "https://user:password@example.com/private-token?token=secret#fragment"
	for _, value := range []string{URL(source), Text("download failed: " + source), Error(errors.New(source)).Error()} {
		for _, secret := range []string{"user:", "password", "private-token", "secret", "fragment"} {
			if strings.Contains(value, secret) {
				t.Fatalf("leaked %q: %s", secret, value)
			}
		}
		if !strings.Contains(value, "example.com") {
			t.Fatal("lost useful origin")
		}
	}
	sentinel := errors.New("sentinel")
	if !errors.Is(Error(sentinel), sentinel) {
		t.Fatal("error identity lost")
	}
}

func TestRedactionIsIdempotent(t *testing.T) {
	source := "https://user:password@example.com/token?token=secret"
	once := URL(source)
	if twice := Text(once); twice != once {
		t.Fatalf("redacting twice lost context: %q => %q", once, twice)
	}
}
