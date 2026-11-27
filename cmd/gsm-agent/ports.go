package main

import (
	"net"
	"strconv"
)

// portAllocator hands out UDP ports from an inclusive range. It is not
// safe for concurrent use; the agent guards it with its mutex.
type portAllocator struct {
	min, max int
	used     map[int]string
	// probe, when set, reports whether a port is actually bindable so ports
	// held by stray processes are skipped.
	probe func(port int) bool
}

func newPortAllocator(min, max int) *portAllocator {
	return &portAllocator{min: min, max: max, used: make(map[int]string)}
}

// Claim reserves the lowest free port for id.
func (p *portAllocator) Claim(id string) (int, bool) {
	for port := p.min; port <= p.max; port++ {
		if _, taken := p.used[port]; taken {
			continue
		}
		if p.probe != nil && !p.probe(port) {
			continue
		}
		p.used[port] = id
		return port, true
	}
	return 0, false
}

// ClaimSpecific reserves port for id; it succeeds when the port is in range
// and free or already held by id.
func (p *portAllocator) ClaimSpecific(port int, id string) bool {
	if port < p.min || port > p.max {
		return false
	}
	if owner, taken := p.used[port]; taken {
		return owner == id
	}
	p.used[port] = id
	return true
}

// Release frees port when it is held by id.
func (p *portAllocator) Release(port int, id string) {
	if owner, ok := p.used[port]; ok && owner == id {
		delete(p.used, port)
	}
}

// Free returns the number of unclaimed ports.
func (p *portAllocator) Free() int {
	return (p.max - p.min + 1) - len(p.used)
}

// udpProbe returns a probe that binds host:port over UDP and releases it.
func udpProbe(host string) func(int) bool {
	return func(port int) bool {
		conn, err := net.ListenPacket("udp", net.JoinHostPort(host, strconv.Itoa(port)))
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}
}
