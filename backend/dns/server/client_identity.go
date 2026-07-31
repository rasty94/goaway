package server

// Client identification: turning a source IP into a hostname, MAC and vendor.
// Split out of handler.go, which is about answering queries rather than about
// working out who asked.

import (
	"bufio"
	"context"
	arp "goaway/backend/dns"
	model "goaway/backend/dns/server/models"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

func (s *DNSServer) reverseHostnameLookup(requestedHostname string) (netip.Addr, bool) {
	trimmed := strings.TrimSuffix(requestedHostname, ".")
	if value, ok := s.clientHostnameCache.Load(trimmed); ok {
		if client, ok := value.(*model.Client); ok {
			return client.IP, true
		}
	}

	return netip.Addr{}, false
}

func (s *DNSServer) getClientInfo(clientIP netip.Addr) *model.Client {
	var isLoopback = clientIP.IsLoopback()
	if isLoopback {
		if localIP, err := getLocalIP(); err == nil {
			clientIP = localIP
		} else {
			log.Warning("Failed to get local IP: %v", err)
			clientIP = IPv4Loopback
		}
	}

	if loaded, ok := s.clientIPCache.Load(clientIP); ok {
		if client, ok := loaded.(*model.Client); ok {
			return client
		}
	}

	macAddress := arp.GetMacAddress(clientIP)
	hostname := s.resolveHostname(clientIP)

	if isLoopback {
		if h, err := os.Hostname(); err == nil {
			hostname = h
		} else {
			hostname = "localhost"
		}
	}

	vendor := s.lookupVendor(clientIP.String(), macAddress)
	client := &model.Client{
		IP:       clientIP,
		LastSeen: time.Now(),
		Name:     hostname,
		Mac:      macAddress,
		Vendor:   vendor,
		Bypass:   false,
	}

	log.Debug("Saving new client: %s", client.IP)
	_ = s.PopulateClientCaches()

	return client
}

func (s *DNSServer) lookupVendor(clientIP, macAddress string) string {
	if macAddress == unknownHostname {
		return ""
	}

	vendor, err := s.MACService.FindVendor(macAddress)
	if err == nil && vendor != "" {
		return vendor
	}

	log.Debug("Lookup vendor for mac %s", macAddress)
	vendor, err = arp.GetMacVendor(macAddress)
	if err != nil {
		log.Debug(
			"Was not able to find vendor for addr '%s' with MAC '%s'. %v",
			clientIP, macAddress, err,
		)
		return ""
	}

	s.MACService.SaveMac(clientIP, macAddress, vendor)
	return vendor
}

func (s *DNSServer) resolveHostname(clientIP netip.Addr) string {
	if clientIP.IsLoopback() {
		hostname, err := os.Hostname()
		if err == nil {
			return hostname
		}
	}

	if hostname := s.reverseDNSLookup(clientIP); hostname != unknownHostname {
		return hostname
	}

	if hostname := s.avahiLookup(clientIP); hostname != unknownHostname {
		return hostname
	}

	if hostname := s.sshBannerLookup(clientIP); hostname != unknownHostname {
		return hostname
	}

	return unknownHostname
}

func (s *DNSServer) avahiLookup(clientIP netip.Addr) string {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	// #nosec G204,G702 - clientIP is a validated netip.Addr
	cmd := exec.CommandContext(ctx, "avahi-resolve-address", clientIP.String())
	output, err := cmd.Output()
	if err == nil {
		lines := strings.SplitSeq(string(output), "\n")
		for line := range lines {
			if strings.Contains(line, clientIP.String()) {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					hostname := strings.TrimSuffix(parts[1], ".local")
					if hostname != "" && hostname != clientIP.String() {
						log.Debug("Found hostname via avahi-resolve: %s -> %s", clientIP, hostname)
						return hostname
					}
				}
			}
		}
	}

	return unknownHostname
}

func (s *DNSServer) reverseDNSLookup(clientIP netip.Addr) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 2 * time.Second,
			}
			gateway := s.Config.DNS.Gateway
			if _, _, err := net.SplitHostPort(gateway); err != nil {
				gateway = net.JoinHostPort(gateway, "53")
			}
			return d.DialContext(ctx, "udp", gateway)
		},
	}

	if hostnames, err := resolver.LookupAddr(ctx, clientIP.String()); err == nil && len(hostnames) > 0 {
		hostname := strings.TrimSuffix(hostnames[0], ".")
		if hostname != clientIP.String() &&
			!strings.Contains(hostname, "in-addr.arpa") && !strings.HasPrefix(hostname, clientIP.String()) {
			log.Debug("Found hostname via reverse DNS: %s -> %s", clientIP, hostname)
			return hostname
		}
	}
	return unknownHostname
}

func (s *DNSServer) sshBannerLookup(clientIP netip.Addr) string {
	// #nosec G704 - clientIP is a validated netip.Addr and lookup is within local network context
	conn, err := net.DialTimeout("tcp", clientIP.String()+":22", 1*time.Second)
	if err != nil {
		return unknownHostname
	}
	defer func() {
		_ = conn.Close()
	}()

	err = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if err != nil {
		log.Warning("Failed to set deadline for SSH banner lookup: %v", err)
		_ = conn.Close()
		return unknownHostname
	}

	reader := bufio.NewReader(conn)
	banner, err := reader.ReadString('\n')
	if err != nil {
		return unknownHostname
	}

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`SSH-2\.0-OpenSSH_[0-9.]+.*?(\w+)`),
		regexp.MustCompile(`SSH.*?(\w+)\.local`),
		regexp.MustCompile(`(\w+)@(\w+)`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(banner)
		if len(matches) > 1 {
			hostname := matches[1]
			if hostname != clientIP.String() && len(hostname) > 1 && hostname != "SSH" {
				log.Debug("Found hostname via SSH banner: %s -> %s", clientIP, hostname)
				return hostname
			}
		}
	}

	return unknownHostname
}
