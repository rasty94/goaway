package dhcp

import (
	"fmt"
	"goaway/backend/database"
	"goaway/backend/logging"
	"goaway/backend/settings"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

var log = logging.GetLogger()

type Service struct {
	repository Repository
	config     *settings.Config

	mu        sync.Mutex
	running   bool
	listeners []net.PacketConn
}

func NewService(repo Repository, cfg *settings.Config) *Service {
	return &Service{repository: repo, config: cfg}
}

func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}
	if !s.config.DHCP.Enabled {
		return nil
	}

	listeners := make([]net.PacketConn, 0, 2)

	if s.config.DHCP.IPv4Enabled {
		addr := net.JoinHostPort(s.config.DHCP.Address, strconv.Itoa(s.config.DHCP.Ports.IPv4))
		conn, err := net.ListenPacket("udp4", addr)
		if err != nil {
			return fmt.Errorf("failed to start DHCPv4 listener on %s: %w", addr, err)
		}
		listeners = append(listeners, conn)
	}

	if s.config.DHCP.IPv6Enabled {
		addr := net.JoinHostPort("::", strconv.Itoa(s.config.DHCP.Ports.IPv6))
		conn, err := net.ListenPacket("udp6", addr)
		if err != nil {
			for _, c := range listeners {
				_ = c.Close()
			}
			return fmt.Errorf("failed to start DHCPv6 listener on %s: %w", addr, err)
		}
		listeners = append(listeners, conn)
	}

	if len(listeners) == 0 {
		return fmt.Errorf("dhcp is enabled but no protocol listener is configured")
	}

	s.listeners = listeners
	s.running = true

	for _, conn := range listeners {
		go s.serveLoop(conn)
	}

	log.Info("DHCP service started with %d listener(s)", len(listeners))
	return nil
}

func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, conn := range s.listeners {
		_ = conn.Close()
	}
	s.listeners = nil
	s.running = false
}

func (s *Service) Restart() error {
	s.Stop()
	return s.Start()
}

func (s *Service) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Service) serveLoop(conn net.PacketConn) {
	buf := make([]byte, 2048)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				s.mu.Lock()
				running := s.running
				s.mu.Unlock()
				if !running {
					return
				}
				continue
			}
			return
		}

		// Full DHCP offer/ack packet processing
		packet, err := dhcpv4.FromBytes(buf[:n])
		if err != nil {
			log.Debug("Failed to parse DHCPv4 packet: %v", err)
			continue
		}

		go s.handleIPv4(conn, addr, packet)
	}
}

func (s *Service) handleIPv4(conn net.PacketConn, addr net.Addr, packet *dhcpv4.DHCPv4) {
	if packet.OpCode != dhcpv4.OpcodeBootRequest {
		return
	}

	msgType := packet.MessageType()
	log.Debug("Received DHCPv4 %s from %s (%s)", msgType, packet.ClientHWAddr.String(), addr.String())

	var reply *dhcpv4.DHCPv4
	var err error

	switch msgType {
	case dhcpv4.MessageTypeDiscover:
		reply, err = s.handleDiscover(packet)
	case dhcpv4.MessageTypeRequest:
		reply, err = s.handleRequest(packet)
	case dhcpv4.MessageTypeRelease:
		err = s.handleRelease(packet)
		return
	case dhcpv4.MessageTypeDecline:
		err = s.handleDecline(packet)
		return
	default:
		log.Debug("Unsupported DHCPv4 message type: %s", msgType)
		return
	}

	if err != nil {
		log.Error("DHCP error handling %s: %v", msgType, err)
		return
	}

	if reply != nil {
		if _, err := conn.WriteTo(reply.ToBytes(), addr); err != nil {
			log.Error("Failed to send DHCPv4 reply: %v", err)
		}
	}
}

func (s *Service) handleDiscover(packet *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, error) {
	ip, err := s.allocateIP(packet.ClientHWAddr, packet.HostName())
	if err != nil {
		return nil, err
	}


	reply, err := dhcpv4.NewReplyFromRequest(packet,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
		dhcpv4.WithYourIP(ip),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(net.ParseIP(s.config.DHCP.Address))),
		dhcpv4.WithLeaseTime(uint32(s.config.DHCP.LeaseDuration)),
		dhcpv4.WithRouter(net.ParseIP(s.config.DHCP.Router)),
		dhcpv4.WithNetmask(net.CIDRMask(24, 32)), // Default mask if not configured, should be improved
		dhcpv4.WithDNS(s.getDNSServers()...),
		dhcpv4.WithDomainSearchList(s.config.DHCP.DomainSearch),
	)

	return reply, err
}

func (s *Service) handleRequest(packet *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, error) {
	requestedIP := packet.RequestedIPAddress()
	if requestedIP == nil {
		requestedIP = packet.ClientIPAddr
	}



	if requestedIP == nil || requestedIP.IsUnspecified() {
		return nil, fmt.Errorf("no requested IP address")
	}

	// Double check lease
	ip, err := s.confirmLease(packet.ClientHWAddr, requestedIP, packet.HostName())
	if err != nil {

		// Send NAK
		reply, _ := dhcpv4.NewReplyFromRequest(packet,
			dhcpv4.WithMessageType(dhcpv4.MessageTypeNak),
			dhcpv4.WithOption(dhcpv4.OptServerIdentifier(net.ParseIP(s.config.DHCP.Address))),
		)
		return reply, err
	}

	reply, err := dhcpv4.NewReplyFromRequest(packet,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
		dhcpv4.WithYourIP(ip),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(net.ParseIP(s.config.DHCP.Address))),
		dhcpv4.WithLeaseTime(uint32(s.config.DHCP.LeaseDuration)),
		dhcpv4.WithRouter(net.ParseIP(s.config.DHCP.Router)),
		dhcpv4.WithNetmask(net.CIDRMask(24, 32)),
		dhcpv4.WithDNS(s.getDNSServers()...),
		dhcpv4.WithDomainSearchList(s.config.DHCP.DomainSearch),
	)

	return reply, err
}

func (s *Service) handleRelease(packet *dhcpv4.DHCPv4) error {
	log.Info("DHCP Release from %s", packet.ClientHWAddr.String())
	// In a real implementation we would mark the IP as free.
	// For now we just allow the lease to expire or manually delete it from UI.
	return nil
}

func (s *Service) handleDecline(packet *dhcpv4.DHCPv4) error {
	log.Warning("DHCP Decline from %s for IP %s", packet.ClientHWAddr.String(), packet.RequestedIPAddress())
	return nil
}

func (s *Service) getDNSServers() []net.IP {
	var ips []net.IP
	for _, srv := range s.config.DHCP.DNSServers {
		ip := net.ParseIP(srv)
		if ip != nil {
			if ip.IsUnspecified() {
				// If 0.0.0.0 is configured, use the server's own address
				ips = append(ips, net.ParseIP(s.config.DHCP.Address))
			} else {
				ips = append(ips, ip)
			}
		}
	}
	if len(ips) == 0 {
		ips = append(ips, net.ParseIP(s.config.DHCP.Address))
	}
	return ips
}

func (s *Service) allocateIP(mac net.HardwareAddr, hostname string) (net.IP, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	macStr := strings.ToLower(mac.String())

	// 1. Check static leases
	staticLeases, _ := s.repository.ListStaticLeases()
	for _, l := range staticLeases {
		if strings.ToLower(l.MAC) == macStr && l.Enabled {
			return net.ParseIP(l.IP), nil
		}
	}

	// 2. Check existing active leases
	activeLeases, _ := s.repository.ListActiveLeases()
	for _, l := range activeLeases {
		if strings.ToLower(l.MAC) == macStr {
			// Update expiration and hostname
			l.ExpiresAt = time.Now().Add(time.Duration(s.config.DHCP.LeaseDuration) * time.Second)
			if hostname != "" {
				l.Hostname = hostname
			}
			_ = s.repository.CreateOrUpdateActiveLease(&l)
			return net.ParseIP(l.IP), nil
		}
	}


	// 3. Find a new IP in range
	start := net.ParseIP(s.config.DHCP.RangeStart).To4()
	end := net.ParseIP(s.config.DHCP.RangeEnd).To4()
	if start == nil || end == nil {
		return nil, fmt.Errorf("invalid DHCP range")
	}

	usedIPs := make(map[string]bool)
	for _, l := range staticLeases {
		usedIPs[net.ParseIP(l.IP).String()] = true
	}
	for _, l := range activeLeases {
		if l.ExpiresAt.After(time.Now()) {
			usedIPs[net.ParseIP(l.IP).String()] = true
		}
	}

	for ip := cloneIP(start); !ip.Equal(nextIP(end)); ip = nextIP(ip) {
		if !usedIPs[ip.String()] {
			newLease := &database.ActiveDHCPLease{
				MAC:       macStr,
				IP:        ip.String(),
				Hostname:  hostname,
				ExpiresAt: time.Now().Add(time.Duration(s.config.DHCP.LeaseDuration) * time.Second),
			}
			if err := s.repository.CreateOrUpdateActiveLease(newLease); err != nil {
				return nil, err
			}
			return ip, nil
		}
	}


	return nil, fmt.Errorf("no available IPs in range")
}

func (s *Service) confirmLease(mac net.HardwareAddr, requested net.IP, hostname string) (net.IP, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	macStr := strings.ToLower(mac.String())
	reqStr := requested.String()


	// Static check
	staticLeases, _ := s.repository.ListStaticLeases()
	for _, l := range staticLeases {
		if strings.ToLower(l.MAC) == macStr && l.Enabled {
			if l.IP == reqStr {
				return requested, nil
			}
			return nil, fmt.Errorf("static lease mismatch")
		}
	}

	// Active check
	activeLeases, _ := s.repository.ListActiveLeases()
	for _, l := range activeLeases {
		if strings.ToLower(l.MAC) == macStr {
			if l.IP == reqStr {
				l.ExpiresAt = time.Now().Add(time.Duration(s.config.DHCP.LeaseDuration) * time.Second)
				if hostname != "" {
					l.Hostname = hostname
				}
				_ = s.repository.CreateOrUpdateActiveLease(&l)
				return requested, nil
			}
		}
	}


	// If not found but in range and free, we could assign it, but usually Request follows Offer.
	// For simplicity, if it was Offered it would be in ActiveLeases.
	return nil, fmt.Errorf("lease not found or expired")
}

func nextIP(ip net.IP) net.IP {
	next := cloneIP(ip)
	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] != 0 {
			break
		}
	}
	return next
}

func cloneIP(ip net.IP) net.IP {
	c := make(net.IP, len(ip))
	copy(c, ip)
	return c
}



func normalizeLease(lease *database.StaticDHCPLease) {
	lease.MAC = strings.ToLower(strings.TrimSpace(lease.MAC))
	lease.IP = strings.TrimSpace(lease.IP)
	lease.Hostname = strings.TrimSpace(lease.Hostname)
}

func (s *Service) validateLease(lease *database.StaticDHCPLease) error {
	normalizeLease(lease)

	if lease.MAC == "" {
		return fmt.Errorf("mac is required")
	}
	if _, err := net.ParseMAC(lease.MAC); err != nil {
		return fmt.Errorf("invalid mac address")
	}
	if lease.IP == "" {
		return fmt.Errorf("ip is required")
	}
	ip := net.ParseIP(lease.IP)
	if ip == nil {
		return fmt.Errorf("invalid ip address")
	}
	if ip.To4() == nil {
		return fmt.Errorf("only IPv4 static leases are currently supported")
	}

	return nil
}

func (s *Service) ListStaticLeases() ([]database.StaticDHCPLease, error) {
	return s.repository.ListStaticLeases()
}

func (s *Service) CreateStaticLease(lease *database.StaticDHCPLease) error {
	if err := s.validateLease(lease); err != nil {
		return err
	}
	return s.repository.CreateStaticLease(lease)
}

func (s *Service) UpdateStaticLease(id uint, lease *database.StaticDHCPLease) error {
	if err := s.validateLease(lease); err != nil {
		return err
	}
	return s.repository.UpdateStaticLease(id, lease)
}

func (s *Service) DeleteStaticLease(id uint) error {
	return s.repository.DeleteStaticLease(id)
}

func (s *Service) ListActiveLeases() ([]database.ActiveDHCPLease, error) {
	return s.repository.ListActiveLeases()
}

