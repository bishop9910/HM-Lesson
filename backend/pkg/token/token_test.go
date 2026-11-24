package token

import (
	"strings"
	"testing"
	"time"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestNewIssuerValidation(t *testing.T) {
	if _, err := NewIssuer("short", time.Hour); err == nil {
		t.Fatal("NewIssuer accepted a short secret")
	}
	if _, err := NewIssuer(testSecret, 0); err == nil {
		t.Fatal("NewIssuer accepted a zero TTL")
	}
}

func TestIssueAndParse(t *testing.T) {
	issuer, err := NewIssuer(testSecret, time.Hour)
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	raw, exp, err := issuer.Issue("staff-1", "owner", "王老师", "jti-1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if raw == "" || exp.Before(time.Now()) {
		t.Fatalf("unexpected token/exp: %q %v", raw, exp)
	}
	claims, err := issuer.Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if claims.Subject != "staff-1" || claims.Role != "owner" || claims.Name != "王老师" || claims.ID != "jti-1" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestParseExpired(t *testing.T) {
	issuer, _ := NewIssuer(testSecret, time.Minute)
	raw, _, _ := issuer.Issue("staff-1", "owner", "王老师", "jti-1")
	issuer.now = func() time.Time { return time.Now().Add(2 * time.Minute) }
	if _, err := issuer.Parse(raw); err != ErrExpired {
		t.Fatalf("Parse error = %v, want ErrExpired", err)
	}
}

func TestParseTampered(t *testing.T) {
	issuer, _ := NewIssuer(testSecret, time.Hour)
	raw, _, _ := issuer.Issue("staff-1", "owner", "王老师", "jti-1")
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d parts", len(parts))
	}
	parts[1] = parts[1][:len(parts[1])-1] + "A"
	if _, err := issuer.Parse(strings.Join(parts, ".")); err != ErrInvalid {
		t.Fatalf("tampered payload error = %v, want ErrInvalid", err)
	}

	other, _ := NewIssuer("abcdef0123456789abcdef0123456789", time.Hour)
	if _, err := other.Parse(raw); err != ErrInvalid {
		t.Fatalf("wrong-secret error = %v, want ErrInvalid", err)
	}
}

func TestParseMalformed(t *testing.T) {
	issuer, _ := NewIssuer(testSecret, time.Hour)
	for _, raw := range []string{"", "a.b", "a.b.c.d", "..", "a.@@@.c"} {
		if _, err := issuer.Parse(raw); err == nil {
			t.Errorf("Parse(%q) returned nil error", raw)
		}
	}
}
