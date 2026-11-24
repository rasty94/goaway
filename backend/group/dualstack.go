package group

import (
	"strings"
	"sync"
)

// DualStackCorrelator links IPv4 and IPv6 addresses belonging to the same device
// using shared MAC addresses or hostnames so dual-stack clients receive identical policies.
// Its zero value is ready to use.
type DualStackCorrelator struct {
	mu       sync.RWMutex
	macToIPs map[string]map[string]struct{}
	ipToMAC  map[string]string
	hostToIP map[string]map[string]struct{}
	ipToHost map[string]string
}

func isValidIdentity(val string) bool {
	v := strings.ToLower(strings.TrimSpace(val))
	return v != "" && v != "unknown" && v != "localhost"
}

// Record associates an IP address with its observed MAC address and/or hostname.
func (d *DualStackCorrelator) Record(ip, mac, hostname string) {
	ip = normalizeIdentifier(ip)
	if !isValidIdentity(ip) {
		return
	}

	mac = normalizeIdentifier(mac)
	hostname = strings.TrimSuffix(normalizeIdentifier(hostname), ".")

	d.mu.Lock()
	defer d.mu.Unlock()

	if isValidIdentity(mac) {
		if d.macToIPs == nil {
			d.macToIPs = make(map[string]map[string]struct{})
			d.ipToMAC = make(map[string]string)
		}
		if d.macToIPs[mac] == nil {
			d.macToIPs[mac] = make(map[string]struct{})
		}
		d.macToIPs[mac][ip] = struct{}{}
		d.ipToMAC[ip] = mac
	}

	if isValidIdentity(hostname) {
		if d.hostToIP == nil {
			d.hostToIP = make(map[string]map[string]struct{})
			d.ipToHost = make(map[string]string)
		}
		if d.hostToIP[hostname] == nil {
			d.hostToIP[hostname] = make(map[string]struct{})
		}
		d.hostToIP[hostname][ip] = struct{}{}
		d.ipToHost[ip] = hostname
	}
}

// CorrelatedIdentifiers returns all known sibling IPs and any resolved MAC for (ip, mac).
func (d *DualStackCorrelator) CorrelatedIdentifiers(ip, mac string) (ips []string, resolvedMAC string) {
	ip = normalizeIdentifier(ip)
	mac = normalizeIdentifier(mac)

	d.mu.RLock()
	defer d.mu.RUnlock()

	if !isValidIdentity(mac) && d.ipToMAC != nil {
		mac = d.ipToMAC[ip]
	}

	seen := map[string]struct{}{}
	addIP := func(candidate string) {
		if candidate == "" || candidate == ip {
			return
		}
		if _, ok := seen[candidate]; ok {
			return
		}
		seen[candidate] = struct{}{}
		ips = append(ips, candidate)
	}

	if isValidIdentity(mac) && d.macToIPs != nil {
		for peerIP := range d.macToIPs[mac] {
			addIP(peerIP)
		}
	}

	if d.ipToHost != nil {
		if host := d.ipToHost[ip]; host != "" {
			for peerIP := range d.hostToIP[host] {
				addIP(peerIP)
				if !isValidIdentity(mac) && d.ipToMAC != nil {
					if peerMAC := d.ipToMAC[peerIP]; isValidIdentity(peerMAC) {
						mac = peerMAC
					}
				}
			}
		}
	}

	return ips, mac
}
