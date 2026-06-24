package dnsmsg

import (
	"testing"

	"github.com/miekg/dns"
)

func TestExtractQName(t *testing.T) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn("example.com"), dns.TypeA)

	name := ExtractQName(msg)
	if name != "example.com" {
		t.Fatalf("expected example.com, got %s", name)
	}
}

func TestExtractQType(t *testing.T) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn("example.com"), dns.TypeMX)

	qtype := ExtractQType(msg)
	if qtype != dns.TypeMX {
		t.Fatalf("expected MX, got %d", qtype)
	}
}

func TestIsDotLocalPositive(t *testing.T) {
	if !IsDotLocal("myhost.local") {
		t.Fatal("expected myhost.local to match .local")
	}
	if !IsDotLocal("local") {
		t.Fatal("expected 'local' to match .local")
	}
}

func TestIsDotLocalNegative(t *testing.T) {
	if IsDotLocal("example.com") {
		t.Fatal("expected example.com NOT to match .local")
	}
}

func TestRefused(t *testing.T) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn("example.com"), dns.TypeA)

	refused := Refused(msg)
	if refused.Rcode != dns.RcodeRefused {
		t.Fatal("expected refused rcode")
	}
}
