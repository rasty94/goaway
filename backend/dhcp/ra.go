package dhcp

import (
	"encoding/binary"
	"net"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
)

// RouterAdvertiser handles periodic Sending of ICMPv6 RA packets
type RouterAdvertiser struct {
	ifaceName string
	prefix    net.IPNet
	dnsIps    []net.IP
	stopChan  chan struct{}
}

func NewRouterAdvertiser(iface string, prefix net.IPNet, dns []net.IP) *RouterAdvertiser {
	return &RouterAdvertiser{
		ifaceName: iface,
		prefix:    prefix,
		dnsIps:    dns,
		stopChan:  make(chan struct{}),
	}
}

func (ra *RouterAdvertiser) Start() error {
	iface, err := net.InterfaceByName(ra.ifaceName)
	if err != nil {
		return err
	}

	conn, err := icmp.ListenPacket("ip6:58", "::")
	if err != nil {
		return err
	}

	pc := ipv6.NewPacketConn(conn)
	if err := pc.SetMulticastInterface(iface); err != nil {
		_ = conn.Close()
		return err
	}

	go ra.loop(pc)
	return nil
}

func (ra *RouterAdvertiser) Stop() {
	close(ra.stopChan)
}

func (ra *RouterAdvertiser) loop(pc *ipv6.PacketConn) {
	defer pc.Close()
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	// Initial Advertisement
	if err := ra.sendRA(pc); err != nil {
		log.Error("Failed to send initial RA: %v", err)
	}

	for {
		select {
		case <-ra.stopChan:
			return
		case <-ticker.C:
			if err := ra.sendRA(pc); err != nil {
				log.Error("Failed to send periodic RA: %v", err)
			}
		}
	}
}

func (ra *RouterAdvertiser) sendRA(pc *ipv6.PacketConn) error {
	// Construct ICMPv6 Router Advertisement
	// This is a minimal implementation. 
	// Real-world RAs often include Prefix Information, RDNSS, etc.
	
	// Message body
	// 1 byte: Current Hop Limit (e.g., 64)
	// 1 byte: M (Managed) and O (Other) flags
	// 2 bytes: Router Lifetime (seconds)
	// 4 bytes: Reachable Time
	// 4 bytes: Retrans Timer
	
	body := make([]byte, 12)
	body[0] = 64   // Hop limit
	body[1] = 0xC0 // Managed Address and Other Config flags (0xC0 = Both set)
	binary.BigEndian.PutUint16(body[2:4], 1800) // Router Lifetime

	msg := icmp.Message{
		Type: ipv6.ICMPTypeRouterAdvertisement,
		Code: 0,
		Body: &icmp.RawBody{Data: body},
	}

	// Add options (Prefix, RDNSS)
	// Prefix Information Option (Type 3)
	prefixOpt := make([]byte, 32)
	prefixOpt[0] = 3 // Type
	prefixOpt[1] = 4 // Length (in 8-byte units, 4 * 8 = 32)
	prefixOpt[2] = uint8(ra.prefix.Mask[0]*8 + ra.prefix.Mask[1]) // Not quite right, but simplified
	ones, _ := ra.prefix.Mask.Size()
	// #nosec G115 - IPv6 prefix size is within uint8 range (0-128)
	prefixOpt[2] = uint8(ones) // Prefix Length
	prefixOpt[3] = 0xC0 // L (on-link) and A (autonomous) flags
	binary.BigEndian.PutUint32(prefixOpt[4:8], 2592000) // Valid Lifetime
	binary.BigEndian.PutUint32(prefixOpt[8:12], 604800)  // Preferred Lifetime
	copy(prefixOpt[16:32], ra.prefix.IP.To16())

	// Append options to body
	msg.Body.(*icmp.RawBody).Data = append(msg.Body.(*icmp.RawBody).Data, prefixOpt...)

	// RDNSS Option (Type 25)
	if len(ra.dnsIps) > 0 {
		rdnssLen := 1 + 2*len(ra.dnsIps) // 1 (header) + 2*N (each IP is 16 bytes = 2 units)
		rdnssOpt := make([]byte, 8+16*len(ra.dnsIps))
		rdnssOpt[0] = 25 // Type RDNSS
		// #nosec G115 - rdnssLen is within uint8 range
		rdnssOpt[1] = uint8(rdnssLen)
		binary.BigEndian.PutUint32(rdnssOpt[4:8], 1200) // Lifetime
		for i, dns := range ra.dnsIps {
			copy(rdnssOpt[8+16*i:24+16*i], dns.To16())
		}
		msg.Body.(*icmp.RawBody).Data = append(msg.Body.(*icmp.RawBody).Data, rdnssOpt...)
	}

	mb, err := msg.Marshal(nil)
	if err != nil {
		return err
	}

	dst := &net.IPAddr{IP: net.ParseIP("ff02::1")} // All Nodes multicast
	_, err = pc.WriteTo(mb, nil, dst)
	return err
}
