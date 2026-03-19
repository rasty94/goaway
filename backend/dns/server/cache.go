package server

import (
	"time"

	"github.com/miekg/dns"
)

type CachedRecord struct {
	ExpiresAt    time.Time
	StaleUntil   time.Time
	CachedAt     time.Time
	Key          string
	Domain       string
	Source       string
	DNSSECStatus string
	IPAddresses  []dns.RR
	OriginalTTL  uint32
}

func (s *DNSServer) getCachedRecord(cached interface{}) ([]dns.RR, string, string, bool) {
	cachedRecord, ok := cached.(CachedRecord)
	if !ok {
		return nil, "", "", false
	}

	now := time.Now()
	if now.Before(cachedRecord.ExpiresAt) {
		remainingSeconds := uint32(cachedRecord.ExpiresAt.Sub(now).Seconds())
		updatedRecords := make([]dns.RR, len(cachedRecord.IPAddresses))

		for i, rr := range cachedRecord.IPAddresses {
			if rr.Header().Ttl != remainingSeconds {
				clone := dns.Copy(rr)
				clone.Header().Ttl = remainingSeconds
				updatedRecords[i] = clone
			} else {
				updatedRecords[i] = rr
			}
		}

		return updatedRecords, cachedRecord.DNSSECStatus, cachedRecord.Source, true
	}

	return nil, "", "", false
}

func (s *DNSServer) getStaleRecord(cached interface{}) ([]dns.RR, string, string, bool) {
	cachedRecord, ok := cached.(CachedRecord)
	if !ok {
		return nil, "", "", false
	}

	now := time.Now()
	if now.After(cachedRecord.ExpiresAt) && now.Before(cachedRecord.StaleUntil) {
		updatedRecords := make([]dns.RR, len(cachedRecord.IPAddresses))
		for i, rr := range cachedRecord.IPAddresses {
			clone := dns.Copy(rr)
			clone.Header().Ttl = 30
			updatedRecords[i] = clone
		}

		return updatedRecords, cachedRecord.DNSSECStatus, cachedRecord.Source, true
	}

	if cachedRecord.Key != "" && now.After(cachedRecord.StaleUntil) {
		log.Debug("Stale window expired, removing %s from cache", cachedRecord.Key)
		s.DomainCache.Delete(cachedRecord.Key)
	}

	return nil, "", "", false
}

func (s *DNSServer) RemoveCachedDomain(domain string) {
	if domain == "" {
		return
	}

	s.DomainCache.Range(func(key, value interface{}) bool {
		cachedRecord, ok := value.(CachedRecord)
		if !ok || cachedRecord.Domain != domain+"." {
			return true
		}

		log.Debug("Removing cached record for domain %s", domain)
		s.DomainCache.Delete(key)
		return true
	})
}

func (s *DNSServer) CacheRecord(cacheKey, domain string, ipAddresses []dns.RR, ttl uint32, dnssecStatus string) {
	s.CacheRecordWithSource(cacheKey, domain, ipAddresses, ttl, dnssecStatus, "upstream")
}

func (s *DNSServer) CacheRecordWithSource(cacheKey, domain string, ipAddresses []dns.RR, ttl uint32, dnssecStatus, source string) {
	if len(ipAddresses) == 0 || !s.Config.DNS.CacheEnabled {
		return
	}

	cacheTTL := time.Duration(s.Config.DNS.CacheTTL) * time.Second
	if ttl > 0 {
		recordTTL := time.Duration(ttl) * time.Second
		if recordTTL < cacheTTL {
			cacheTTL = recordTTL
		}
	}

	now := time.Now()
	staleWindow := cacheTTL
	if staleWindow < 30*time.Second {
		staleWindow = 30 * time.Second
	}

	s.DomainCache.Store(cacheKey, CachedRecord{
		IPAddresses:  ipAddresses,
		ExpiresAt:    now.Add(cacheTTL),
		StaleUntil:   now.Add(cacheTTL).Add(staleWindow),
		CachedAt:     now,
		OriginalTTL:  ttl,
		Key:          cacheKey,
		Domain:       domain,
		Source:       source,
		DNSSECStatus: dnssecStatus,
	})
}
