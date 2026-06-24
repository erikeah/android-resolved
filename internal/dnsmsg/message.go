package dnsmsg

import (
	"strings"

	"github.com/miekg/dns"
)

func ExtractQName(msg *dns.Msg) string {
	if len(msg.Question) == 0 {
		return ""
	}
	return strings.TrimSuffix(msg.Question[0].Name, ".")
}

func ExtractQType(msg *dns.Msg) uint16 {
	if len(msg.Question) == 0 {
		return dns.TypeNone
	}
	return msg.Question[0].Qtype
}

func IsDotLocal(name string) bool {
	return strings.HasSuffix(name, ".local") || name == "local"
}

func Refused(msg *dns.Msg) *dns.Msg {
	reply := new(dns.Msg)
	reply.SetReply(msg)
	reply.Rcode = dns.RcodeRefused
	return reply
}

func ServFail(msg *dns.Msg) *dns.Msg {
	reply := new(dns.Msg)
	reply.SetReply(msg)
	reply.Rcode = dns.RcodeServerFailure
	return reply
}

func Question(name string, qtype uint16) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(name), qtype)
	return msg
}
