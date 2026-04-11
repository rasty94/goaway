package cluster

import (
	"net"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// DNSProxy implements a simple DNS proxy that balances requests among cluster nodes
type DNSProxy struct {
	cluster    *Service
	addr       string
	dnsPort    int
	
	mu         sync.RWMutex
	currIndex  int
	
	// Metrics
	totalRequests  uint64
	nodeRequests   map[string]uint64
	errorRequests  uint64
}

func NewDNSProxy(cluster *Service, addr string, port int) *DNSProxy {
	return &DNSProxy{
		cluster:      cluster,
		addr:         addr,
		dnsPort:      port,
		nodeRequests: make(map[string]uint64),
	}
}

func (p *DNSProxy) Start() error {
	log.Info("[HA/Proxy] Starting DNS Cluster Proxy on %s (Forwarding to Cluster)", p.addr)

	udpServer := &dns.Server{
		Addr: p.addr,
		Net:  "udp",
		Handler: dns.HandlerFunc(p.HandleDNSRequest),
	}

	tcpServer := &dns.Server{
		Addr: p.addr,
		Net:  "tcp",
		Handler: dns.HandlerFunc(p.HandleDNSRequest),
	}

	go func() {
		if err := udpServer.ListenAndServe(); err != nil {
			log.Error("[HA/Proxy] UDP Proxy failed: %v", err)
		}
	}()

	go func() {
		if err := tcpServer.ListenAndServe(); err != nil {
			log.Error("[HA/Proxy] TCP Proxy failed: %v", err)
		}
	}()

	return nil
}

func (p *DNSProxy) HandleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	p.mu.Lock()
	p.totalRequests++
	p.mu.Unlock()

	nodes := p.getHealthyNodes()
	if len(nodes) == 0 {
		log.Warning("[HA/Proxy] No healthy nodes available to proxy request")
		dns.HandleFailed(w, r)
		return
	}

	// Sticky Sessions: Use Source IP Hashing to select a node consistently
	clientIP, _, _ := net.SplitHostPort(w.RemoteAddr().String())
	node := p.selectNodeForIP(clientIP, nodes)

	// Forward to node
	target := node.IP
	if target == "" {
		target = node.Address
	}

	targetAddr := net.JoinHostPort(target, "53")

	// Update node metric
	p.mu.Lock()
	p.nodeRequests[target]++
	p.mu.Unlock()

	c := new(dns.Client)
	c.Timeout = 2 * time.Second
	
	resp, _, err := c.Exchange(r, targetAddr)
	if err != nil {
		log.Debug("[HA/Proxy] Node %s failed (Sticky node), retrying with next healthy node...", targetAddr)
		// Fallback to Round Robin if sticky node failed
		p.mu.Lock()
		p.errorRequests++
		p.currIndex++
		node = nodes[p.currIndex % len(nodes)]
		p.mu.Unlock()

		target = node.IP
		if target == "" {
			target = node.Address
		}
		targetAddr = net.JoinHostPort(target, "53")
		
		p.mu.Lock()
		p.nodeRequests[target]++
		p.mu.Unlock()

		resp, _, err = c.Exchange(r, targetAddr)
	}

	if err != nil {
		log.Error("[HA/Proxy] Proxy forwarding failed (all attempts): %v", err)
		dns.HandleFailed(w, r)
		return
	}

	if resp != nil {
		if err := w.WriteMsg(resp); err != nil {
			log.Error("[HA/Proxy] Failed to write message: %v", err)
		}
	}
}

func (p *DNSProxy) selectNodeForIP(ip string, nodes []*ClusterNode) *ClusterNode {
	// Simple IP Hash affinity
	var hash uint32
	for i := 0; i < len(ip); i++ {
		hash = 31*hash + uint32(ip[i])
	}
	
	return nodes[int(hash)%len(nodes)]
}

func (p *DNSProxy) GetStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"totalRequests": p.totalRequests,
		"nodeRequests":  p.nodeRequests,
		"errorRequests": p.errorRequests,
	}
}

func (p *DNSProxy) getHealthyNodes() []*ClusterNode {
	p.cluster.peersMu.RLock()
	defer p.cluster.peersMu.RUnlock()

	var healthy []*ClusterNode
	for _, node := range p.cluster.peers {
		if node.Status == StateOnline {
			healthy = append(healthy, node)
		}
	}
	return healthy
}
