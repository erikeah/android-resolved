package resolver

import (
	"github.com/miekg/dns"
)

type Resolver interface {
	Exchange(msg *dns.Msg) (*dns.Msg, error)
	String() string
}
