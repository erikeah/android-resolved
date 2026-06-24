package server

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/anomalyco/android-resolved/internal/cache"
	"github.com/anomalyco/android-resolved/internal/dnsmsg"
	"github.com/anomalyco/android-resolved/internal/resolver"
	"github.com/anomalyco/android-resolved/internal/router"
	"github.com/anomalyco/android-resolved/internal/stats"
	"github.com/miekg/dns"
)

type TunHandler struct {
	fd      *os.File
	router  router.Router
	cache   *cache.Cache
	stats   *stats.StatsTracker
	timeout time.Duration
	quit    chan struct{}
	wg      sync.WaitGroup
}

func NewTunHandler(fd int, rtr router.Router, c *cache.Cache, stats *stats.StatsTracker, timeout time.Duration) (*TunHandler, error) {
	f := os.NewFile(uintptr(fd), "tun")
	if f == nil {
		return nil, fmt.Errorf("invalid tun fd: %d", fd)
	}
	return &TunHandler{
		fd:      f,
		router:  rtr,
		cache:   c,
		stats:   stats,
		timeout: timeout,
		quit:    make(chan struct{}),
	}, nil
}

func (h *TunHandler) Start() {
	h.wg.Add(1)
	go h.run()
}

func (h *TunHandler) Stop() {
	close(h.quit)
	h.fd.Close()
	h.wg.Wait()
}

func (h *TunHandler) run() {
	defer h.wg.Done()
	buf := make([]byte, 2048)
	for {
		select {
		case <-h.quit:
			return
		default:
		}
		n, err := h.fd.Read(buf)
		if err != nil {
			select {
			case <-h.quit:
				return
			default:
			}
			log.Printf("tun read error: %v", err)
			return
		}
		resp := h.handlePacket(buf[:n])
		if resp != nil {
			if _, err := h.fd.Write(resp); err != nil {
				log.Printf("tun write error: %v", err)
			}
		}
	}
}

func (h *TunHandler) handlePacket(pkt []byte) []byte {
	if len(pkt) < 20 {
		return nil
	}

	versionIhl := pkt[0]
	ihl := int(versionIhl&0x0f) * 4
	if ihl < 20 || ihl > len(pkt) {
		return nil
	}

	totalLen := int(binary.BigEndian.Uint16(pkt[2:4]))
	if totalLen > len(pkt) {
		totalLen = len(pkt)
	}

	if pkt[9] != 17 {
		return nil
	}

	udpStart := ihl
	if udpStart+8 > totalLen {
		return nil
	}

	srcPort := int(binary.BigEndian.Uint16(pkt[udpStart : udpStart+2]))
	dstPort := int(binary.BigEndian.Uint16(pkt[udpStart+2 : udpStart+4]))
	if dstPort != 53 {
		return nil
	}
	udpLen := int(binary.BigEndian.Uint16(pkt[udpStart+4 : udpStart+6]))

	dnsStart := udpStart + 8
	dnsLen := udpLen - 8
	if dnsLen <= 0 || dnsStart+dnsLen > totalLen {
		return nil
	}

	msg := new(dns.Msg)
	if err := msg.Unpack(pkt[dnsStart : dnsStart+dnsLen]); err != nil {
		return nil
	}
	if len(msg.Question) == 0 {
		return nil
	}

	qname := dnsmsg.ExtractQName(msg)
	qtype := dnsmsg.ExtractQType(msg)

	h.stats.RecordQuery()

	if cached := h.cache.Get(qname, qtype); cached != nil {
		h.stats.RecordCacheHit()
		cached.Id = msg.Id
		return h.buildResponse(pkt, ihl, udpStart, srcPort, dstPort, cached)
	}
	h.stats.RecordCacheMiss()

	match := h.router.Route(qname)
	if match == nil {
		return h.buildResponse(pkt, ihl, udpStart, srcPort, dstPort, dnsmsg.Refused(msg))
	}

	reslv, err := h.buildResolver(match)
	if err != nil {
		return h.buildResponse(pkt, ihl, udpStart, srcPort, dstPort, dnsmsg.ServFail(msg))
	}

	reply, err := reslv.Exchange(msg)
	if err != nil {
		log.Printf("tun resolve %s: %v", qname, err)
		return h.buildResponse(pkt, ihl, udpStart, srcPort, dstPort, dnsmsg.ServFail(msg))
	}

	h.stats.RecordUpstream(match.Upstream)
	h.cache.Set(qname, qtype, reply)
	return h.buildResponse(pkt, ihl, udpStart, srcPort, dstPort, reply)
}

func (h *TunHandler) buildResponse(orig []byte, ihl, udpStart, srcPort, dstPort int, dnsMsg *dns.Msg) []byte {
	dnsResp, err := dnsMsg.Pack()
	if err != nil || dnsResp == nil {
		return nil
	}

	udpLen := 8 + len(dnsResp)
	ipTotalLen := ihl + udpLen

	buf := make([]byte, ipTotalLen)

	buf[0] = 0x45
	buf[1] = 0
	binary.BigEndian.PutUint16(buf[2:4], uint16(ipTotalLen))
	binary.BigEndian.PutUint16(buf[4:6], 0)
	binary.BigEndian.PutUint16(buf[6:8], 0)
	buf[8] = 64
	buf[9] = 17
	binary.BigEndian.PutUint16(buf[10:12], 0)
	copy(buf[12:16], orig[16:20])
	copy(buf[16:20], orig[12:16])

	var sum uint32
	for i := 0; i < ihl; i += 2 {
		sum += uint32(binary.BigEndian.Uint16(buf[i : i+2]))
	}
	for sum > 0xffff {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	binary.BigEndian.PutUint16(buf[10:12], uint16(^sum))

	binary.BigEndian.PutUint16(buf[udpStart:udpStart+2], uint16(dstPort))
	binary.BigEndian.PutUint16(buf[udpStart+2:udpStart+4], uint16(srcPort))
	binary.BigEndian.PutUint16(buf[udpStart+4:udpStart+6], uint16(udpLen))
	binary.BigEndian.PutUint16(buf[udpStart+6:udpStart+8], 0)

	copy(buf[udpStart+8:], dnsResp)

	return buf
}

func (h *TunHandler) buildResolver(match *router.Match) (resolver.Resolver, error) {
	return resolver.NewFromMatch(match, h.timeout)
}
