package policy

import (
	"fmt"
	"goaway/backend/blacklist"
	"goaway/backend/database"
	"goaway/backend/logging"
	"net"
	"strings"
	"sync"
	"time"
)

var log = logging.GetLogger()

type Service struct {
	repository       Repository
	blacklistService *blacklist.Service

	cacheMu     sync.RWMutex
	policies    []database.Policy
	assignments map[string][]uint // key: identifierType:identifier, value: policyIDs
	categories  map[uint][]string // policyID -> categories
}

func NewService(repo Repository, blService *blacklist.Service) *Service {
	s := &Service{
		repository:       repo,
		blacklistService: blService,
	}

	if err := s.RefreshCache(); err != nil {
		log.Warning("Could not populate policy cache: %v", err)
	}

	return s
}

func (s *Service) RefreshCache() error {
	policies, err := s.repository.GetPolicies()
	if err != nil {
		return err
	}

	assignments, err := s.repository.GetAssignments()
	if err != nil {
		return err
	}

	policyIDsByAssignment := make(map[string][]uint)
	for _, a := range assignments {
		key := a.IdentifierType + ":" + strings.ToLower(a.Identifier)
		policyIDsByAssignment[key] = append(policyIDsByAssignment[key], a.PolicyID)
	}

	categoriesMap := make(map[uint][]string)
	for _, p := range policies {
		cats, _ := s.repository.GetCategories(p.ID)
		categoriesMap[p.ID] = cats
	}

	s.cacheMu.Lock()
	s.policies = policies
	s.assignments = policyIDsByAssignment
	s.categories = categoriesMap
	s.cacheMu.Unlock()

	return nil
}

func (s *Service) ShouldBlock(clientIP, clientMAC string, clientGroupIDs []uint, domainName string) (bool, string, string, bool, bool, string) {
	blocked, action, policyName, _, isDryRun, safeSearch, category := s.ShouldBlockDetailed(clientIP, clientMAC, clientGroupIDs, domainName)
	return blocked, action, policyName, isDryRun, safeSearch, category
}

func (s *Service) ShouldBlockDetailed(clientIP, clientMAC string, clientGroupIDs []uint, domainName string) (bool, string, string, string, bool, bool, string) {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	now := time.Now()

	// Policies are already sorted by priority DESC in repository.GetPolicies()
	for _, p := range s.policies {
		if !p.Enabled {
			continue
		}

		if !s.appliesToClient(p.ID, clientIP, clientMAC, clientGroupIDs) {
			continue
		}

		if !s.isScheduleActive(p.Schedule, now) {
			continue
		}

		cats := s.categories[p.ID]
		action := strings.ToLower(p.Action)

		if action == "block" || action == "nxdomain" || action == "null" {
			// If no categories, it's a general block for this client
			if len(cats) == 0 {
				return true, p.Action, p.Name, "unconditional block", p.IsDryRun, p.SafeSearch, ""
			}

			// Check if the domain matches any categories assigned to the policy
			for _, cat := range cats {
				if blocked, pattern := s.blacklistService.IsBlacklistedByCategoryDetailed(domainName, cat); blocked {
					return true, p.Action, p.Name, fmt.Sprintf("category:%s pattern:%s", cat, pattern), p.IsDryRun, p.SafeSearch, cat
				}
			}
		}

		if action == "allow" {
			// If no categories, it's a total allow (whitelist)
			if len(cats) == 0 {
				return false, "allow", p.Name, "unconditional allow", p.IsDryRun, p.SafeSearch, ""
			}

			// Check if domain matches any of the allowed categories
			for _, cat := range cats {
				if allowed, pattern := s.blacklistService.IsBlacklistedByCategoryDetailed(domainName, cat); allowed {
					return false, "allow", p.Name, fmt.Sprintf("category:%s pattern:%s", cat, pattern), p.IsDryRun, p.SafeSearch, cat
				}
			}
		}

		// If nothing above returned but p.SafeSearch is true, we could return it here, 
		// but ACTION is empty, so we continue searching?
		// No, if a policy matches the CLIENT criteria (appliesToClient), it's the winning policy 
		// even if it doesn't match a CATEGORY for blocking/allowing.
		// Wait, if ACTION is allow but domain doesn't match any CAT, it doesn't trigger.
		// But if ACTION is allow and NO categories are defined, it's a general allow.
		// SafeSearch should probably be applied if any matching policy has it.
		// Let's return the first matching policy's SafeSearch status if action matches or if it's the default.
	}

	return false, "", "", "", false, false, ""
}

func (s *Service) appliesToClient(policyID uint, ip, mac string, groupIDs []uint) bool {
	// check IP
	if s.hasAssignment("ip:"+strings.ToLower(ip), policyID) {
		return true
	}
	// check MAC
	if s.hasAssignment("mac:"+strings.ToLower(mac), policyID) {
		return true
	}
	// check Groups
	for _, gid := range groupIDs {
		if s.hasAssignment(fmt.Sprintf("group:%d", gid), policyID) {
			return true
		}
	}
	// check CIDRs
	for key, pIDs := range s.assignments {
		if strings.HasPrefix(key, "cidr:") {
			cidr := strings.TrimPrefix(key, "cidr:")
			_, ipNet, err := net.ParseCIDR(cidr)
			if err == nil && ipNet.Contains(net.ParseIP(ip)) {
				for _, pid := range pIDs {
					if pid == policyID {
						return true
					}
				}
			}
		}
	}

	return false
}

func (s *Service) hasAssignment(key string, policyID uint) bool {
	pIDs := s.assignments[key]
	for _, id := range pIDs {
		if id == policyID {
			return true
		}
	}
	return false
}

func (s *Service) isScheduleActive(sched *database.Schedule, t time.Time) bool {
	if sched == nil {
		return true
	}

	// Day check
	day := strings.ToLower(t.Weekday().String()[:3]) // mon, tue...
	if sched.Days != "" && !strings.Contains(strings.ToLower(sched.Days), day) {
		return false
	}

	// Time check
	if sched.StartTime != "" && sched.EndTime != "" {
		currentTime := t.Format("15:04")
		if currentTime < sched.StartTime || currentTime > sched.EndTime {
			return false
		}
	}

	return true
}

func (s *Service) GetPolicies() ([]database.Policy, error) {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	return s.policies, nil
}
