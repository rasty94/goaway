package server

import (
	"context"
	"fmt"
	model "goaway/backend/dns/server/models"
	"goaway/backend/metrics"
	"goaway/backend/notification"
	"net"
	"net/netip"
	"strings"
	"time"

	"codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/dnsutil"
	"codeberg.org/miekg/dns/rdata"
)

var (
	blackholeIPv4 = netip.MustParseAddr("0.0.0.0")
	blackholeIPv6 = netip.MustParseAddr("::")
	IPv4Loopback  = netip.MustParseAddr("127.0.0.1")
)

const (
	unknownHostname = "unknown"
)

func trimDomainDot(name string) string {
	if name != "" && name[len(name)-1] == '.' {
		return name[:len(name)-1]
	}
	return name
}

func isPTRQuery(req *Request, domainName string) bool {
	return req.QType() == dns.TypePTR || strings.HasSuffix(domainName, "in-addr.arpa.")
}

func (s *DNSServer) checkAndUpdatePauseStatus() {
	if s.Config.DNS.Status.Paused &&
		s.Config.DNS.Status.PausedAt.After(s.Config.DNS.Status.PauseTime) {
		s.Config.DNS.Status.Paused = false
	}
}

func (s *DNSServer) Explain(domainName string, clientIP string) model.ExplainResult {
	domainName = trimDomainDot(domainName)
	parsedIP, err := netip.ParseAddr(clientIP)
	if err != nil {
		parsedIP = IPv4Loopback
	}
	client := s.getClientInfo(parsedIP)

	res := model.ExplainResult{
		Domain:   domainName,
		ClientIP: client.IP.String(),
		Status:   dns.RcodeToString[dns.RcodeSuccess],
	}

	if client.Bypass {
		res.Action = "allow"
		res.Reason = "Client is in bypass mode"
		return res
	}

	if s.Config.DNS.Status.Paused {
		res.Action = "allow"
		res.Reason = "DNS blocking is paused globally"
		return res
	}

	clientIPStr := client.IP.String()

	// 1. Check Advanced Policy Engine
	effectivePolicy := s.GroupService.GetEffectivePolicy(clientIPStr, client.Mac)
	blocked, action, policyName, pattern, isDryRun, safeSearch, category := s.PolicyService.ShouldBlockDetailed(clientIPStr, client.Mac, effectivePolicy.GroupIDs, domainName)
	if action != "" {
		res.Blocked = blocked
		res.Action = action
		if isDryRun {
			res.Action = action + " (DRY RUN)"
		}
		if safeSearch {
			res.Action += " + SafeSearch"
		}
		if category != "" {
			res.Reason = fmt.Sprintf("Advanced Policy Engine match (category: %s)", category)
		} else {
			res.Reason = "Advanced Policy Engine match"
		}
		res.PolicyName = policyName
		res.Matching = []string{pattern}
		return res
	}

	// 2. Fallback to Legacy/Global
	blockedDetail, pattern := s.BlacklistService.IsBlacklistedDetailed(domainName)
	globalBlocked := blockedDetail
	globalWhitelisted, whitePattern := s.WhitelistService.IsWhitelistedDetailed(domainName)

	blocked, groupAction, groupPattern := s.GroupService.ShouldBlockDetailed(
		clientIPStr,
		client.Mac,
		domainName,
		domainName, // full domain
		globalBlocked,
		globalWhitelisted,
	)

	res.Blocked = blocked
	res.Action = "allow"
	res.Reason = groupAction
	res.Matching = []string{groupPattern}

	if blocked {
		res.Action = "block"
	}

	if globalWhitelisted {
		res.Matching = append(res.Matching, "Global Whitelist: "+whitePattern)
	}
	if globalBlocked {
		res.Matching = append(res.Matching, "Global Blacklist: "+pattern)
	}

	return res
}

func (s *DNSServer) checkPolicyDecision(client *model.Client, domainName, fullName string) (bool, bool, string) {
	if client.Bypass {
		log.Debug("Allowing client '%s' to bypass %s", client.IP, fullName)
		return false, false, ""
	}

	if s.Config.DNS.Status.Paused {
		return false, false, ""
	}

	clientIPStr := client.IP.String()

	// 1. Check Advanced Policy Engine (EPIC-02)
	effectivePolicy := s.GroupService.GetEffectivePolicy(clientIPStr, client.Mac)
	blocked, action, policyName, isDryRun, safeSearch, category := s.PolicyService.ShouldBlock(clientIPStr, client.Mac, effectivePolicy.GroupIDs, domainName)
	if action != "" {
		if blocked {
			if isDryRun {
				log.Debug("[DRY RUN] Policy '%s' would %s %s for %s", policyName, action, domainName, client.IP)
				return false, safeSearch, category
			}
			log.Debug("Advanced Policy Engine: '%s' blocking %s for %s (action: %s, category: %s)", policyName, domainName, client.IP, action, category)
			return true, safeSearch, category
		}
		// Policy allows, but it might have SafeSearch
		return false, safeSearch, category
	}

	// 2. Fallback to Legacy Group/Global Logic
	globalBlocked := s.BlacklistService.IsBlacklisted(domainName)
	globalWhitelisted := s.WhitelistService.IsWhitelisted(fullName)

	if s.GroupService != nil {
		return s.GroupService.ShouldBlock(
			clientIPStr,
			client.Mac,
			domainName,
			fullName,
			globalBlocked,
			globalWhitelisted,
		), false, "" // Legacy doesn't support SafeSearch or カテゴリ counts here
	}

	return globalBlocked && !globalWhitelisted, false, ""
}

func (s *DNSServer) processQuery(request *Request) model.RequestLogEntry {
	start := time.Now()
	domainName := trimDomainDot(request.QName())
	clientIP := request.Client.IP.String()

	metrics.TotalQueries.WithLabelValues(clientIP, request.QTypeStr()).Inc()

	if isPTRQuery(request, domainName) {
		entry := s.handlePTRQuery(request)
		return s.finalizeDNSSECStatus(entry, clientIP)
	}

	if ip, found := s.reverseHostnameLookup(domainName); found {
		entry := s.respondWithHostnameA(request, ip)
		return s.finalizeDNSSECStatus(entry, clientIP)
	}

	s.checkAndUpdatePauseStatus()

	blocked, safeSearch, category := s.checkPolicyDecision(request.Client, domainName, request.QName())

	if blocked {
		metrics.BlockedQueries.WithLabelValues(clientIP, domainName).Inc()
		if category != "" {
			metrics.CategoriesBlocked.WithLabelValues(clientIP, category).Inc()
		}
		metrics.DNSLatency.WithLabelValues(clientIP, "blocked").Observe(time.Since(start).Seconds())
		entry := s.handleBlacklisted(request)
		return s.finalizeDNSSECStatus(entry, clientIP)
	}

	if safeSearch {
		if val, redirected := s.applySafeSearch(request); redirected {
			metrics.DNSLatency.WithLabelValues(clientIP, "safesearch").Observe(time.Since(start).Seconds())
			return s.finalizeDNSSECStatus(val, clientIP)
		}
	}

	if isLocalLookup(domainName) {
		val, err := s.LocalForwardLookup(request)
		if err != nil {
			log.Debug("Reverse lookup failed for %s: %v", domainName, err)
		} else {
			metrics.DNSLatency.WithLabelValues(clientIP, "local").Observe(time.Since(start).Seconds())
			return s.finalizeDNSSECStatus(val, clientIP)
		}
	}

	entry := s.handleStandardQuery(request)
	status := "allowed"
	if entry.Cached {
		status = "cached"
		metrics.CachedQueries.WithLabelValues(clientIP, domainName).Inc()
		if entry.Stale {
			metrics.StaleQueries.WithLabelValues(clientIP, domainName).Inc()
		}
		if entry.PrefetchHit {
			metrics.PrefetchHitQueries.WithLabelValues(clientIP, domainName).Inc()
		}
	}
	metrics.DNSLatency.WithLabelValues(clientIP, status).Observe(time.Since(start).Seconds())
	return s.finalizeDNSSECStatus(entry, clientIP)
}

func (s *DNSServer) finalizeDNSSECStatus(entry model.RequestLogEntry, clientIP string) model.RequestLogEntry {
	if entry.DNSSECStatus == "" {
		entry.DNSSECStatus = s.defaultDNSSECStatus()
	}

	metrics.DNSSECResponses.WithLabelValues(clientIP, entry.DNSSECStatus).Inc()
	return entry
}

func getLocalIP() (netip.Addr, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return netip.Addr{}, err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipv4 := ipnet.IP.To4(); ipv4 != nil {
				ip, _ := netip.AddrFromSlice(ipv4)
				return ip, nil
			}
		}
	}

	return IPv4Loopback, fmt.Errorf("no non-loopback IPv4 address found")
}

func (s *DNSServer) handlePTRQuery(request *Request) model.RequestLogEntry {
	ipParts := strings.TrimSuffix(request.QName(), ".in-addr.arpa.")
	parts := strings.Split(ipParts, ".")

	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	ipStr := strings.Join(parts, ".")

	if ipStr == IPv4Loopback.String() {
		return s.respondWithLocalhost(request)
	}

	if !isPrivateIP(ipStr) {
		return s.forwardPTRQueryUpstream(request)
	}

	hostname := s.RequestService.GetClientNameFromIP(ipStr)
	if hostname == unknownHostname {
		if ip, err := netip.ParseAddr(ipStr); err == nil {
			hostname = s.resolveHostname(ip)
		} else {
			log.Warning("Failed to parse IP for hostname lookup: %v", err)
			hostname = unknownHostname
		}
	}

	if hostname != unknownHostname {
		return s.respondWithHostnamePTR(request, hostname)
	}

	return s.forwardPTRQueryUpstream(request)
}

func isPrivateIP(ipStr string) bool {
	// net.IP.IsPrivate covers exactly RFC1918 (10/8, 172.16/12, 192.168/16),
	// so we no longer reparse three CIDRs on every query.
	ip := net.ParseIP(ipStr)
	return ip != nil && ip.IsPrivate()
}

func (s *DNSServer) respondWithLocalhost(request *Request) model.RequestLogEntry {
	request.Msg.Response = true
	request.Msg.Authoritative = false
	request.Msg.RecursionAvailable = true
	request.Msg.Rcode = dns.RcodeSuccess

	ptr := &dns.PTR{
		Hdr: dns.Header{
			Name:  request.QName(),
			TTL:   3600,
			Class: dns.ClassINET,
		},
		PTR: rdata.PTR{
			Ptr: "localhost.lan.",
		},
	}

	request.Msg.Answer = []dns.RR{ptr}

	request.Respond(s.NotificationService)
	return model.RequestLogEntry{
		Timestamp: request.Sent,
		Domain:    request.QName(),
		Status:    dnsutil.CodeToString(dns.RcodeSuccess),
		IP: []model.ResolvedIP{
			{
				IP:    IPv4Loopback,
				RType: "PTR",
			},
		},
		Blocked:           false,
		Cached:            false,
		ResponseTime:      time.Since(request.Sent),
		ClientInfo:        request.Client,
		QueryType:         "PTR",
		ResponseSizeBytes: request.Msg.Len(),
		Protocol:          request.Protocol,
	}
}

func (s *DNSServer) respondWithHostnameA(request *Request, hostIP netip.Addr) model.RequestLogEntry {
	request.Msg.Response = true
	request.Msg.Authoritative = false
	request.Msg.RecursionAvailable = true
	request.Msg.Rcode = dns.RcodeSuccess

	response := &dns.A{
		Hdr: dns.Header{
			Name:  request.QName(),
			TTL:   60,
			Class: dns.ClassINET,
		},
		A: rdata.A{
			Addr: hostIP,
		},
	}

	request.Msg.Answer = []dns.RR{response}
	request.Respond(s.NotificationService)
	return s.respondWithType(request, dns.TypeA, hostIP)
}

func (s *DNSServer) respondWithHostnamePTR(request *Request, hostname string) model.RequestLogEntry {
	request.Msg.Response = true
	request.Msg.Authoritative = false
	request.Msg.RecursionAvailable = true
	request.Msg.Rcode = dns.RcodeSuccess

	ptr := &dns.PTR{
		Hdr: dns.Header{
			Name:  request.QName(),
			TTL:   3600,
			Class: dns.ClassINET,
		},
		PTR: rdata.PTR{
			Ptr: hostname + ".",
		},
	}

	request.Msg.Answer = []dns.RR{ptr}
	request.Respond(s.NotificationService)
	ip, err := netip.ParseAddr(hostname)
	if err != nil {
		log.Warning("Not able to parse ip for hostname %s", hostname)
		return model.RequestLogEntry{
			Timestamp:         request.Sent,
			Domain:            request.QName(),
			Status:            dnsutil.CodeToString(dns.RcodeSuccess),
			IP:                []model.ResolvedIP{},
			Blocked:           false,
			Cached:            false,
			ResponseTime:      time.Since(request.Sent),
			ClientInfo:        request.Client,
			QueryType:         "PTR",
			ResponseSizeBytes: request.Msg.Len(),
			Protocol:          request.Protocol,
		}
	}
	return s.respondWithType(request, dns.TypePTR, ip)
}

func (s *DNSServer) respondWithType(request *Request, rType uint16, ip netip.Addr) model.RequestLogEntry {
	return model.RequestLogEntry{
		Domain:    request.QName(),
		Status:    dnsutil.CodeToString(dns.RcodeSuccess),
		QueryType: request.QTypeStr(),
		IP: []model.ResolvedIP{
			{
				IP:    ip,
				RType: dnsutil.TypeToString(rType),
			},
		},
		ResponseSizeBytes: request.Msg.Len(),
		Timestamp:         request.Sent,
		ResponseTime:      time.Since(request.Sent),
		Blocked:           false,
		Cached:            false,
		ClientInfo:        request.Client,
		Protocol:          request.Protocol,
	}
}

func (s *DNSServer) forwardPTRQueryUpstream(request *Request) model.RequestLogEntry {
	answers, _, status, dnssecStatus := s.QueryUpstream(request)
	request.Msg.Answer = append(request.Msg.Answer, answers...)

	if rcode, ok := dns.StringToRcode[status]; ok {
		request.Msg.Rcode = rcode
	} else {
		request.Msg.Rcode = dns.RcodeServerFailure
	}

	request.Msg.Response = true
	request.Msg.Authoritative = false
	request.Msg.RecursionAvailable = true

	var resolvedHostnames []model.ResolvedIP
	for _, answer := range answers {
		if ptr, ok := answer.(*dns.PTR); ok {
			if ip, err := netip.ParseAddr(ptr.Ptr); err == nil {
				resolvedHostnames = append(resolvedHostnames, model.ResolvedIP{
					IP:    ip,
					RType: "PTR",
				})
			} else {
				log.Warning("Failed to parse PTR data: %v", err)
			}
		}
	}

	request.Respond(s.NotificationService)
	return model.RequestLogEntry{
		Domain:            request.QName(),
		Status:            status,
		DNSSECStatus:      dnssecStatus,
		QueryType:         request.QTypeStr(),
		IP:                resolvedHostnames,
		ResponseSizeBytes: request.Msg.Len(),
		Timestamp:         request.Sent,
		ResponseTime:      time.Since(request.Sent),
		ClientInfo:        request.Client,
		Protocol:          request.Protocol,
	}
}

func (s *DNSServer) handleStandardQuery(request *Request) model.RequestLogEntry {
	answers, cached, stale, prefetchHit, status, dnssecStatus := s.Resolve(request)
	resolved := make([]model.ResolvedIP, 0, len(answers))

	request.Msg.Answer = answers
	request.Msg.Response = true
	request.Msg.Authoritative = false
	if request.Msg.RecursionDesired {
		request.Msg.RecursionAvailable = true
	}
	if rcode, ok := dns.StringToRcode[status]; ok {
		request.Msg.Rcode = rcode
	} else {
		request.Msg.Rcode = dns.RcodeServerFailure
	}

	for _, a := range answers {
		switch rr := a.(type) {
		case *dns.A:
			if ip, err := netip.ParseAddr(rr.A.String()); err == nil {
				resolved = append(resolved, model.ResolvedIP{
					IP:    ip,
					RType: "A",
				})
			} else {
				log.Warning("Failed to parse A record: %v", err)
			}
		case *dns.AAAA:
			if ip, err := netip.ParseAddr(rr.AAAA.String()); err == nil {
				resolved = append(resolved, model.ResolvedIP{
					IP:    ip,
					RType: "AAAA",
				})
			} else {
				log.Warning("Failed to parse AAAA record: %v", err)
			}
		case *dns.PTR:
			if ip, err := netip.ParseAddr(rr.Ptr); err == nil {
				resolved = append(resolved, model.ResolvedIP{
					IP:    ip,
					RType: "PTR",
				})
			} else {
				log.Warning("Failed to parse PTR record: %v", err)
			}
		case *dns.CNAME:
			if ip, err := netip.ParseAddr(rr.Target); err == nil {
				resolved = append(resolved, model.ResolvedIP{
					IP:    ip,
					RType: "CNAME",
				})
			} else {
				log.Warning("Failed to parse CNAME record: %v", err)
			}
		case *dns.SVCB:
			if ip, err := netip.ParseAddr(rr.Target); err == nil {
				resolved = append(resolved, model.ResolvedIP{
					IP:    ip,
					RType: "SVCB",
				})
			} else {
				log.Warning("Failed to parse SVCB record: %v", err)
			}
		case *dns.MX:
			if ip, err := netip.ParseAddr(rr.Mx); err == nil {
				resolved = append(resolved, model.ResolvedIP{
					IP:    ip,
					RType: "MX",
				})
			} else {
				log.Warning("Failed to parse MX record: %v", err)
			}
		case *dns.TXT:
			if ip, err := netip.ParseAddr(rr.Txt[0]); err == nil {
				resolved = append(resolved, model.ResolvedIP{
					IP:    ip,
					RType: "TXT",
				})
			} else {
				log.Warning("Failed to parse TXT record: %v", err)
			}
		case *dns.NS:
			if ip, err := netip.ParseAddr(rr.Ns); err == nil {
				resolved = append(resolved, model.ResolvedIP{
					IP:    ip,
					RType: "NS",
				})
			} else {
				log.Warning("Failed to parse NS record: %v", err)
			}
		case *dns.SOA:
			if ip, err := netip.ParseAddr(rr.Ns); err == nil {
				resolved = append(resolved, model.ResolvedIP{
					IP:    ip,
					RType: "SOA",
				})
			} else {
				log.Warning("Failed to parse SOA record: %v", err)
			}
		case *dns.SRV:
			if ip, err := netip.ParseAddr(fmt.Sprintf("%s:%d", rr.Target, rr.Port)); err == nil {
				resolved = append(resolved, model.ResolvedIP{
					IP:    ip,
					RType: "SRV",
				})
			} else {
				log.Warning("Failed to parse SRV record: %v", err)
			}
		case *dns.HTTPS:
			if ip, err := netip.ParseAddr(rr.Target); err == nil {
				resolved = append(resolved, model.ResolvedIP{
					IP:    ip,
					RType: "HTTPS",
				})
			} else {
				log.Warning("Failed to parse HTTPS record: %v", err)
			}
		case *dns.CAA:
			if ip, err := netip.ParseAddr(fmt.Sprintf("%s %d %s", rr.Tag, rr.Flag, rr.Value)); err == nil {
				resolved = append(resolved, model.ResolvedIP{
					IP:    ip,
					RType: "CAA",
				})
			} else {
				log.Warning("Failed to parse CAA record: %v", err)
			}
		case *dns.DNSKEY:
			if ip, err := netip.ParseAddr(fmt.Sprintf("flags:%d protocol:%d algorithm:%d", rr.Flags, rr.Protocol, rr.Algorithm)); err == nil {
				resolved = append(resolved, model.ResolvedIP{
					IP:    ip,
					RType: "DNSKEY",
				})
			} else {
				log.Warning("Failed to parse DNSKEY record: %v", err)
			}
		default:
			log.Warning("Unhandled record type '%s' while requesting '%s'", request.QTypeStr(), request.QName())
		}
	}

	request.Respond(s.NotificationService)
	return model.RequestLogEntry{
		Domain:            request.QName(),
		Status:            status,
		DNSSECStatus:      dnssecStatus,
		QueryType:         request.QTypeStr(),
		IP:                resolved,
		ResponseSizeBytes: request.Msg.Len(),
		Timestamp:         request.Sent,
		ResponseTime:      time.Since(request.Sent),
		Cached:            cached,
		Stale:             stale,
		PrefetchHit:       prefetchHit,
		ClientInfo:        request.Client,
		Protocol:          request.Protocol,
	}
}

func (s *DNSServer) Resolve(req *Request) ([]dns.RR, bool, bool, bool, string, string) {
	cacheKey := req.QName() + ":" + req.QTypeStr()
	var staleCandidate []dns.RR
	var staleDNSSECStatus string
	var staleSource string
	var hasStaleCandidate bool

	if s.Config.DNS.CacheEnabled {
		if cached, found := s.DomainCache.Load(cacheKey); found {
			if ipAddresses, dnssecStatus, source, valid := s.getCachedRecord(cached); valid {
				if dnssecStatus == "" {
					dnssecStatus = s.defaultDNSSECStatus()
				}
				return ipAddresses, true, false, source == "prefetch", dnsutil.CodeToString(dns.RcodeSuccess), dnssecStatus
			}

			if staleRecords, dnssecStatus, source, staleValid := s.getStaleRecord(cached); staleValid {
				staleCandidate = staleRecords
				staleDNSSECStatus = dnssecStatus
				staleSource = source
				hasStaleCandidate = true
			}
		}
	}

	if answers, ttl, status, dnssecStatus := s.resolveResolution(req.QName()); len(answers) > 0 {
		s.CacheRecord(cacheKey, req.QName(), answers, ttl, dnssecStatus)
		return answers, false, false, false, status, dnssecStatus
	}

	answers, ttl, status, dnssecStatus := s.resolveCNAMEChain(req, make(map[string]bool))
	if len(answers) > 0 {
		s.CacheRecord(cacheKey, req.QName(), answers, ttl, dnssecStatus)
		return answers, false, false, false, status, dnssecStatus
	}

	if hasStaleCandidate && status == dnsutil.CodeToString(dns.RcodeServerFailure) {
		if staleDNSSECStatus == "" {
			staleDNSSECStatus = s.defaultDNSSECStatus()
		}
		return staleCandidate, true, true, staleSource == "prefetch", dns.RcodeToString[dns.RcodeSuccess], staleDNSSECStatus
	}

	return answers, false, false, false, status, dnssecStatus
}

func (s *DNSServer) resolveResolution(domain string) ([]dns.RR, uint32, string, string) {
	var (
		records []dns.RR
		// #nosec G115 - CacheTTL is validated
		ttl          = uint32(s.Config.DNS.CacheTTL)
		status       = dnsutil.CodeToString(dns.RcodeSuccess)
		dnssecStatus = s.defaultDNSSECStatus()
	)

	res, err := s.ResolutionService.GetResolution(domain)
	if err != nil {
		log.Error("Database lookup error for domain (%s): %v", domain, err)
		return nil, 0, dnsutil.CodeToString(dns.RcodeServerFailure), dnssecStatus
	}

	if res.Value == "" {
		return nil, 0, dnsutil.CodeToString(dns.RcodeNameError), dnssecStatus
	}

	switch strings.ToUpper(res.Type) {
	case "A":
		if ip, err := netip.ParseAddr(res.Value); err == nil && ip.Is4() {
			records = append(records, &dns.A{
				Hdr: dns.Header{Name: dnsutil.Fqdn(domain), TTL: ttl, Class: dns.ClassINET},
				A:   rdata.A{Addr: ip},
			})
		}
	case "AAAA":
		if ip, err := netip.ParseAddr(res.Value); err == nil && !ip.Is4() {
			records = append(records, &dns.AAAA{
				Hdr:  dns.Header{Name: dnsutil.Fqdn(domain), TTL: ttl, Class: dns.ClassINET},
				AAAA: rdata.AAAA{Addr: ip},
			})
		}
	case "CNAME":
		records = append(records, &dns.CNAME{
			Hdr:   dns.Header{Name: dnsutil.Fqdn(domain), TTL: ttl, Class: dns.ClassINET},
			CNAME: rdata.CNAME{Target: dnsutil.Fqdn(res.Value)},
		})
	default:
		// Fallback to auto-detection if type unspecified
		if ip, err := netip.ParseAddr(res.Value); err == nil {
			if ip.Is4() {
				records = append(records, &dns.A{
					Hdr: dns.Header{Name: dnsutil.Fqdn(domain), TTL: ttl, Class: dns.ClassINET},
					A:   rdata.A{Addr: ip},
				})
			} else {
				records = append(records, &dns.AAAA{
					Hdr:  dns.Header{Name: dnsutil.Fqdn(domain), TTL: ttl, Class: dns.ClassINET},
					AAAA: rdata.AAAA{Addr: ip},
				})
			}
		}
	}

	if len(records) == 0 {
		status = dnsutil.CodeToString(dns.RcodeNameError)
	}

	return records, ttl, status, dnssecStatus
}

func (s *DNSServer) resolveCNAMEChain(req *Request, visited map[string]bool) ([]dns.RR, uint32, string, string) {
	if visited[req.QName()] {
		return nil, 0, dnsutil.CodeToString(dns.RcodeServerFailure), s.defaultDNSSECStatus()
	}
	visited[req.QName()] = true

	answers, ttl, status, dnssecStatus := s.QueryUpstream(req)
	if len(answers) > 0 {
		for _, answer := range answers {
			if _, ok := answer.(*dns.CNAME); ok {
				targetAnswers, targetTTL, targetStatus, targetDNSSECStatus := s.resolveCNAMEChain(req, visited)
				if len(targetAnswers) > 0 {
					minTTL := min(targetTTL, ttl)
					if targetDNSSECStatus == DNSSECStatusBogus {
						dnssecStatus = DNSSECStatusBogus
					}
					return append(answers, targetAnswers...), minTTL, targetStatus, dnssecStatus
				}
				return answers, ttl, status, dnssecStatus
			}
		}
	}

	return answers, ttl, status, dnssecStatus
}

func (s *DNSServer) QueryUpstream(req *Request) ([]dns.RR, uint32, string, string) {
	resultCh := make(chan *dns.Msg, 1)
	errCh := make(chan error, 1)

	go func() {
		go s.WSCom(communicationMessage{IP: "", Client: false, Upstream: true, DNS: false})

		upstreamMsg := dns.NewMsg(req.Question.Header().Name, req.QType())
		upstreamMsg.RecursionDesired = true
		upstreamMsg.ID = dns.ID()
		if s.dnssecMode() != "off" {
			upstreamMsg.Security = true
			upstreamMsg.UDPSize = 1232
		}

		var in *dns.Msg
		var err error

		// Check conditional forwarders first
		queryDomain := strings.TrimSuffix(req.QName(), ".")
		isForwarded := false
		for _, cf := range s.Config.DNS.ConditionalForwarders {
			cfDomain := strings.TrimSuffix(cf.Domain, ".")
			if queryDomain == cfDomain || strings.HasSuffix(queryDomain, "."+cfDomain) {
				log.Debug("Conditional forwarding %s -> %s", queryDomain, cf.Upstream)
				in, err = s.exchangeWithProtocol(upstreamMsg, cf.Upstream, "udp")
				isForwarded = true
				break
			}
		}

		if !isForwarded {
			// Iterate over enabled upstreams
			for _, upstream := range s.Config.DNS.Upstream.Servers {
				if !upstream.Enabled {
					continue
				}

				log.Debug("Sending query to '%s' (%s) using %s", upstream.Name, upstream.Address, upstream.Protocol)
				in, err = s.exchangeWithProtocol(upstreamMsg, upstream.Address, upstream.Protocol)
				if err == nil && in != nil {
					break
				}
				log.Warning("Upstream '%s' failed: %v", upstream.Name, err)
			}
		}

		if err != nil {
			errCh <- err
			return
		}

		if in == nil {
			errCh <- fmt.Errorf("no response from any upstream")
			return
		}

		resultCh <- in
	}()

	select {
	case in := <-resultCh:
		go s.WSCom(communicationMessage{IP: "", Client: false, Upstream: false, DNS: true})
		dnssecStatus := s.classifyDNSSECResponse(in, nil)
		if s.dnssecMode() == "strict" && dnssecStatus == DNSSECStatusBogus {
			return nil, 0, dns.RcodeToString[dns.RcodeServerFailure], dnssecStatus
		}

		status := dnsutil.CodeToString(dns.RcodeServerFailure)
		if statusStr, ok := dns.RcodeToString[in.Rcode]; ok {
			status = statusStr
		}

		var ttl uint32 = 3600
		if len(in.Answer) > 0 {
			ttl = in.Answer[0].Header().TTL
			for _, a := range in.Answer {
				if a.Header().TTL < ttl {
					ttl = a.Header().TTL
				}
			}
		} else if len(in.Ns) > 0 {
			ttl = in.Ns[0].Header().TTL
		}

		if len(in.Ns) > 0 {
			req.Msg.Ns = make([]dns.RR, len(in.Ns))
			copy(req.Msg.Ns, in.Ns)
		}
		req.Msg.AuthenticatedData = in.AuthenticatedData
		req.Msg.CheckingDisabled = in.CheckingDisabled
		if len(in.Extra) > 0 {
			req.Msg.Extra = make([]dns.RR, len(in.Extra))
			copy(req.Msg.Extra, in.Extra)
		}

		return in.Answer, ttl, status, dnssecStatus

	case err := <-errCh:
		dnssecStatus := s.classifyDNSSECResponse(nil, err)
		log.Warning("Upstream resolution error for domain (%s): %v", req.QName(), err)
		s.NotificationService.SendNotification(
			notification.SeverityWarning,
			notification.CategoryDNS,
			fmt.Sprintf("Upstream resolution error for domain (%s)", req.QName()),
		)
		return nil, 0, dnsutil.CodeToString(dns.RcodeServerFailure), dnssecStatus

	case <-time.After(5 * time.Second):
		dnssecStatus := s.classifyDNSSECResponse(nil, fmt.Errorf("timeout"))
		log.Warning("Upstream lookup for %s timed out", req.QName())
		return nil, 0, dnsutil.CodeToString(dns.RcodeServerFailure), dnssecStatus
	}
}

func (s *DNSServer) LocalForwardLookup(req *Request) (model.RequestLogEntry, error) {
	hostname := strings.ReplaceAll(req.Question.Header().Name, ".in-addr.arpa.", "")
	hostname = strings.ReplaceAll(hostname, ".ip6.arpa.", "")
	if !strings.HasSuffix(hostname, ".") {
		hostname += "."
	}

	queryType := req.QType()
	if queryType == 0 {
		queryType = dns.TypeA
	}

	dnsMsg := dns.NewMsg(hostname, queryType)
	client := &dns.Client{}
	start := time.Now()
	log.Debug("Performing local forward lookup for %s", hostname)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	in, _, err := client.Exchange(ctx, dnsMsg, "udp", s.Config.DNS.Gateway)
	responseTime := time.Since(start)

	if err != nil {
		log.Error("DNS exchange error for %s: %v", hostname, err)
		return model.RequestLogEntry{}, fmt.Errorf("forward DNS query failed: %w", err)
	}

	if in.Rcode != dns.RcodeSuccess {
		status := dnsutil.CodeToString(in.Rcode)
		log.Info("DNS query for %s returned status %s", hostname, status)
		return model.RequestLogEntry{}, fmt.Errorf("forward lookup failed with status: %s", status)
	}

	var ips []model.ResolvedIP
	for _, answer := range in.Answer {
		if a, ok := answer.(*dns.A); ok {
			ips = append(ips, model.ResolvedIP{IP: a.Addr})
		}
	}

	if len(ips) == 0 && queryType == dns.TypeA {
		return model.RequestLogEntry{}, fmt.Errorf("no A records found for hostname: %s", hostname)
	}

	req.Msg.Rcode = in.Rcode
	req.Msg.Answer = in.Answer

	req.Respond(s.NotificationService)
	entry := model.RequestLogEntry{
		Domain:            req.Question.Header().Name,
		Status:            dnsutil.CodeToString(in.Rcode),
		QueryType:         dnsutil.TypeToString(queryType),
		IP:                ips,
		ResponseSizeBytes: in.Len(),
		Timestamp:         start,
		ResponseTime:      responseTime,
		Blocked:           false,
		Cached:            false,
		ClientInfo:        req.Client,
		Protocol:          model.UDP,
	}

	return entry, nil
}

func isLocalLookup(qname string) bool {
	return strings.HasSuffix(qname, ".in-addr.arpa.") || strings.HasSuffix(qname, ".ip6.arpa.")
}

func (s *DNSServer) handleBlacklisted(req *Request) model.RequestLogEntry {
	req.Msg.Response = true
	req.Msg.Authoritative = false
	req.Msg.RecursionAvailable = true
	req.Msg.Rcode = dns.RcodeSuccess

	var resolved []model.ResolvedIP
	// #nosec G115 - CacheTTL is validated
	cacheTTL := uint32(s.Config.DNS.CacheTTL)

	switch req.QType() {
	case dns.TypeA:
		req.Msg.Answer = []dns.RR{&dns.A{
			Hdr: dns.Header{
				Name:  req.Question.Header().Name,
				Class: dns.ClassINET,
				TTL:   cacheTTL,
			},
			A: rdata.A{Addr: blackholeIPv4},
		}}
		resolved = []model.ResolvedIP{{IP: blackholeIPv4, RType: "A"}}
	case dns.TypeAAAA:
		req.Msg.Answer = []dns.RR{&dns.AAAA{
			Hdr: dns.Header{
				Name:  req.Question.Header().Name,
				TTL:   cacheTTL,
				Class: dns.ClassINET,
			},
			AAAA: rdata.AAAA{Addr: blackholeIPv6},
		}}
		resolved = []model.ResolvedIP{{IP: blackholeIPv6, RType: "AAAA"}}
	default:
		req.Msg.Rcode = dns.RcodeNameError
		req.Msg.Answer = nil
		resolved = nil
	}

	if len(req.Msg.Question) == 0 {
		return model.RequestLogEntry{
			Domain: "unknown",
		}
	}

	req.Respond(s.NotificationService)
	return model.RequestLogEntry{
		Domain:            req.Question.Header().Name,
		Status:            dnsutil.CodeToString(req.Msg.Rcode),
		QueryType:         req.QTypeStr(),
		IP:                resolved,
		ResponseSizeBytes: req.Msg.Len(),
		Timestamp:         req.Sent,
		ResponseTime:      time.Since(req.Sent),
		Blocked:           true,
		Cached:            false,
		ClientInfo:        req.Client,
		Protocol:          req.Protocol,
	}
}

func (s *DNSServer) applySafeSearch(request *Request) (model.RequestLogEntry, bool) {
	domain := strings.ToLower(trimDomainDot(request.QName()))
	qType := request.QType()

	if qType != dns.TypeA && qType != dns.TypeAAAA {
		return model.RequestLogEntry{}, false
	}

	var targetIP string

	if strings.Contains(domain, "google.") {
		targetIP = "216.239.38.120"
	} else if strings.Contains(domain, "youtube.") || strings.HasSuffix(domain, "youtubei.googleapis.com") || strings.HasSuffix(domain, "youtube.googleapis.com") {
		targetIP = "216.239.38.119"
	} else if strings.Contains(domain, "bing.com") {
		targetIP = "204.79.197.220"
	} else if strings.Contains(domain, "duckduckgo.com") {
		targetIP = "52.142.124.215"
	}

	if targetIP == "" {
		return model.RequestLogEntry{}, false
	}

	if qType == dns.TypeAAAA {
		if strings.Contains(domain, "google.") || strings.Contains(domain, "youtube.") {
			targetIP = "2001:4860:4802:32::78"
		} else {
			return s.respondWithNoData(request), true
		}
	}

	request.Msg.Response = true
	request.Msg.Rcode = dns.RcodeSuccess

	ip := netip.MustParseAddr(targetIP)
	hdr := dns.Header{Name: request.QName(), Class: dns.ClassINET, TTL: 60}
	var rr dns.RR
	if qType == dns.TypeA {
		rr = &dns.A{Hdr: hdr, A: rdata.A{Addr: ip}}
	} else {
		rr = &dns.AAAA{Hdr: hdr, AAAA: rdata.AAAA{Addr: ip}}
	}

	request.Msg.Answer = []dns.RR{rr}
	request.Respond(s.NotificationService)

	return model.RequestLogEntry{
		Domain:            request.QName(),
		Status:            dnsutil.CodeToString(dns.RcodeSuccess),
		QueryType:         request.QTypeStr(),
		IP:                []model.ResolvedIP{{IP: ip, RType: request.QTypeStr()}},
		ResponseSizeBytes: request.Msg.Len(),
		Timestamp:         request.Sent,
		ResponseTime:      time.Since(request.Sent),
		Blocked:           false,
		ClientInfo:        request.Client,
		Protocol:          request.Protocol,
	}, true
}

func (s *DNSServer) respondWithNoData(request *Request) model.RequestLogEntry {
	request.Msg.Response = true
	request.Msg.Rcode = dns.RcodeSuccess
	request.Respond(s.NotificationService)
	return model.RequestLogEntry{
		Domain:     request.QName(),
		Status:     dnsutil.CodeToString(dns.RcodeSuccess),
		QueryType:  request.QTypeStr(),
		Timestamp:  request.Sent,
		ClientInfo: request.Client,
	}
}
