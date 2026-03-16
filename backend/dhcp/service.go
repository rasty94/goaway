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

		// Placeholder parser: keeps server loop healthy while lease-management APIs are functional.
		// Full DHCP offer/ack packet processing is the next iteration.
		log.Debug("Received DHCP datagram (%d bytes) from %s", n, addr.String())
	}
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

