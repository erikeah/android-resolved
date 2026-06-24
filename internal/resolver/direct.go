package resolver

import (
	"fmt"
	"net"
	"time"

	"github.com/miekg/dns"
)

type DirectResolver struct {
	addr   string
	proto  string
	client dns.Client
}

func NewDirectResolver(addr string, tcp bool, timeout time.Duration) *DirectResolver {
	proto := "udp"
	netProto := "udp"
	if tcp {
		proto = "tcp"
		netProto = "tcp"
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "53")
	}
	return &DirectResolver{
		addr:  addr,
		proto: proto,
		client: dns.Client{
			Net:     netProto,
			Timeout: timeout,
		},
	}
}

func (r *DirectResolver) Exchange(msg *dns.Msg) (*dns.Msg, error) {
	reply, _, err := r.client.Exchange(msg, r.addr)
	if err != nil {
		return nil, fmt.Errorf("direct %s exchange: %w", r.proto, err)
	}
	return reply, nil
}

func (r *DirectResolver) String() string {
	return fmt.Sprintf("direct(%s://%s)", r.proto, r.addr)
}
