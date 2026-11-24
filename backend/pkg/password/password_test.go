package password

import (
	"strings"
	"testing"
)

func TestHashVerify(t *testing.T) {
	hash, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(hash, "argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatalf("unexpected PHC prefix: %s", hash)
	}
	if !Verify(hash, "correct horse battery staple") {
		t.Fatal("Verify returned false for the correct password")
	}
	if Verify(hash, "wrong") {
		t.Fatal("Verify returned true for a wrong password")
	}
	if Verify(hash, "") {
		t.Fatal("Verify returned true for an empty password")
	}
}

func TestHashRejectsEmpty(t *testing.T) {
	if _, err := Hash(""); err == nil {
		t.Fatal("Hash accepted an empty password")
	}
}

func TestVerifyMalformed(t *testing.T) {
	cases := []string{
		"",
		"plaintext",
		"argon2id$v=19$m=65536,t=3,p=2$only-four",
		"argon2id$v=19$m=0,t=3,p=2$c2FsdA$aGFzaA",
		"argon2id$v=19$m=65536,t=3,p=2$!!!$aGFzaA",
		"bcrypt$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA",
	}
	for _, encoded := range cases {
		if Verify(encoded, "x") {
			t.Errorf("Verify(%q) returned true", encoded)
		}
	}
}

func TestParseMemory(t *testing.T) {
	hash, err := Hash("x")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	m, err := ParseMemory(hash)
	if err != nil {
		t.Fatalf("ParseMemory: %v", err)
	}
	if m != 65536 {
		t.Fatalf("memory = %d, want 65536", m)
	}
	if _, err := ParseMemory("bad"); err == nil {
		t.Fatal("ParseMemory accepted an invalid hash")
	}
}
