package group

import (
	"fmt"
	"goaway/backend/database"
	"goaway/backend/cluster"
	"goaway/backend/domain"
	"goaway/backend/logging"
	"slices"
	"strings"
	"sync"
)

var log = logging.GetLogger()

const (
	IdentifierTypeIP  = "ip"
	IdentifierTypeMAC = "mac"
	DefaultGroupName  = "default"
)

type Service struct {
	repository Repository

	cacheMu        sync.RWMutex
	groupsByID     map[uint]database.ClientGroup
	defaultGroup   uint
	assignments    map[string][]uint
	blockedByGroup map[uint]*domain.Matcher
	allowedByGroup map[uint]*domain.Matcher
	replicator     cluster.Replicator
	dualStack      DualStackCorrelator
}


type EffectivePolicy struct {
	GroupIDs            []uint   `json:"groupIDs"`
	GroupNames          []string `json:"groupNames"`
	UsesGlobalBlocklist bool     `json:"usesGlobalBlocklist"`
}

func NewService(repo Repository) *Service {
	s := &Service{
		repository:     repo,
		groupsByID:     make(map[uint]database.ClientGroup),
		assignments:    make(map[string][]uint),
		blockedByGroup: make(map[uint]*domain.Matcher),
		allowedByGroup: make(map[uint]*domain.Matcher),
	}

	if err := s.ensureDefaultGroup(); err != nil {
		log.Warning("Could not ensure default group exists: %v", err)
	}

	if err := s.RefreshCache(); err != nil {
		log.Warning("Could not populate group cache: %v", err)
	}

	return s
}

func (s *Service) SetReplicator(r cluster.Replicator) {
	s.replicator = r
}

func (s *Service) ensureDefaultGroup() error {
	groups, err := s.repository.GetGroups()
	if err != nil {
		return err
	}

	for _, g := range groups {
		if g.IsDefault {
			return nil
		}
	}

	return s.repository.CreateGroup(&database.ClientGroup{
		Name:              DefaultGroupName,
		Description:       "Default group used for unassigned clients",
		UseGlobalPolicies: true,
		IsDefault:         true,
	})
}

func normalizeIdentifier(identifier string) string {
	return strings.ToLower(strings.TrimSpace(identifier))
}

func identifierKey(identifierType, identifier string) string {
	return normalizeIdentifier(identifierType) + ":" + normalizeIdentifier(identifier)
}

func normalizeDomain(d string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(d)), ".")
}

func (s *Service) RefreshCache() error {
	groups, err := s.repository.GetGroups()
	if err != nil {
		return err
	}

	assignments, err := s.repository.GetAssignments()
	if err != nil {
		return err
	}

	blocked, err := s.repository.GetBlockedDomains()
	if err != nil {
		return err
	}

	allowed, err := s.repository.GetAllowedDomains()
	if err != nil {
		return err
	}

	groupsByID := make(map[uint]database.ClientGroup, len(groups))
	assignmentsMap := make(map[string][]uint)
	blockedByGroup := make(map[uint]*domain.Matcher)
	allowedByGroup := make(map[uint]*domain.Matcher)

	var defaultGroupID uint
	for _, g := range groups {
		groupsByID[g.ID] = g
		if g.IsDefault {
			defaultGroupID = g.ID
		}
		blockedByGroup[g.ID] = domain.NewMatcher()
		allowedByGroup[g.ID] = domain.NewMatcher()
	}

	for _, a := range assignments {
		key := identifierKey(a.IdentifierType, a.Identifier)
		assignmentsMap[key] = append(assignmentsMap[key], a.GroupID)
	}

	for _, b := range blocked {
		matcher, ok := blockedByGroup[b.GroupID]
		if !ok {
			continue
		}
		matcher.Add(normalizeDomain(b.Domain))
	}

	for _, a := range allowed {
		matcher, ok := allowedByGroup[a.GroupID]
		if !ok {
			continue
		}
		matcher.Add(normalizeDomain(a.Domain))
	}

	s.cacheMu.Lock()
	s.groupsByID = groupsByID
	s.assignments = assignmentsMap
	s.blockedByGroup = blockedByGroup
	s.allowedByGroup = allowedByGroup
	s.defaultGroup = defaultGroupID
	s.cacheMu.Unlock()

	return nil
}

func (s *Service) ListBlockedDomains() []database.GroupBlockedDomain {
	rows, _ := s.repository.GetBlockedDomains()
	return rows
}

func (s *Service) ListAllowedDomains() []database.GroupAllowedDomain {
	rows, _ := s.repository.GetAllowedDomains()
	return rows
}

func (s *Service) GetGroups() ([]database.ClientGroup, error) {
	return s.repository.GetGroups()
}

func (s *Service) CreateGroup(name, description string, useGlobalPolicies bool) (*database.ClientGroup, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("group name is required")
	}

	group := &database.ClientGroup{
		Name:              name,
		Description:       strings.TrimSpace(description),
		UseGlobalPolicies: useGlobalPolicies,
		IsDefault:         false,
	}

	if err := s.repository.CreateGroup(group); err != nil {
		return nil, err
	}

	if err := s.RefreshCache(); err != nil {
		return nil, err
	}

	if s.replicator != nil {
		s.replicator.Broadcast(cluster.ReplicatedEvent{
			Type:    cluster.EventGroupCreate,
			Payload: group,
		})
	}

	return group, nil
}

func (s *Service) groupNameByID(id uint) string {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	if g, ok := s.groupsByID[id]; ok {
		return g.Name
	}
	return ""
}

func (s *Service) groupNamesByIDs(ids []uint) []string {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if g, ok := s.groupsByID[id]; ok && g.Name != "" {
			names = append(names, g.Name)
		}
	}
	return names
}

// GetGroupIDByName resolves a stable group name to its local database ID.
func (s *Service) GetGroupIDByName(name string) (uint, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, false
	}
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	for id, g := range s.groupsByID {
		if g.Name == name {
			return id, true
		}
	}
	return 0, false
}

func (s *Service) DeleteGroup(id uint) error {
	name := s.groupNameByID(id)
	if err := s.repository.DeleteGroup(id); err != nil {
		return err
	}
	if err := s.RefreshCache(); err != nil {
		log.Error("Failed to refresh group cache after deletion: %v", err)
	}

	if s.replicator != nil {
		s.replicator.Broadcast(cluster.ReplicatedEvent{
			Type:    cluster.EventGroupDelete,
			Payload: map[string]interface{}{"id": id, "name": name},
		})
	}

	return nil
}

func (s *Service) broadcast(t cluster.EventType, payload interface{}) {
	if s.replicator != nil {
		s.replicator.Broadcast(cluster.ReplicatedEvent{Type: t, Payload: payload})
	}
}

func (s *Service) SetGroupUseGlobalPolicies(id uint, enabled bool) error {
	if err := s.repository.SetGroupUseGlobalPolicies(id, enabled); err != nil {
		return err
	}
	s.broadcast(cluster.EventGroupUpdate, map[string]interface{}{"id": id, "name": s.groupNameByID(id), "useGlobalPolicies": enabled})
	return s.RefreshCache()
}

func (s *Service) ReplaceAssignments(identifier, identifierType string, groupIDs []uint) error {
	identifier = normalizeIdentifier(identifier)
	identifierType = normalizeIdentifier(identifierType)

	if identifier == "" {
		return fmt.Errorf("identifier is required")
	}
	if identifierType != IdentifierTypeIP && identifierType != IdentifierTypeMAC {
		return fmt.Errorf("identifierType must be 'ip' or 'mac'")
	}

	cleanGroupIDs := make([]uint, 0, len(groupIDs))
	seen := map[uint]struct{}{}
	for _, id := range groupIDs {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		cleanGroupIDs = append(cleanGroupIDs, id)
	}

	if err := s.repository.ReplaceAssignments(identifier, identifierType, cleanGroupIDs); err != nil {
		return err
	}
	s.broadcast(cluster.EventClientUpdate, map[string]interface{}{
		"identifier":     identifier,
		"identifierType": identifierType,
		"groupIds":       cleanGroupIDs,
		"groupNames":     s.groupNamesByIDs(cleanGroupIDs),
	})
	return s.RefreshCache()
}

func (s *Service) AddBlockedDomain(groupID uint, domainName string) error {
	domainName = normalizeDomain(domainName)
	if domainName == "" {
		return fmt.Errorf("domain is required")
	}
	if err := s.repository.AddBlockedDomain(groupID, domainName); err != nil {
		return err
	}
	s.broadcast(cluster.EventGroupBlockedAdd, map[string]interface{}{"groupId": groupID, "groupName": s.groupNameByID(groupID), "domain": domainName})
	return s.RefreshCache()
}

// AddBlockedDomains inserts every domain in one pass and refreshes the cache exactly once at the
// end, instead of once per domain. It keeps inserting after an error so every domain gets a
// chance, and still refreshes the cache on partial failure so it matches what actually made it
// into the database. inserted reports how many domains actually landed, since err only ever
// carries the first failure.
func (s *Service) AddBlockedDomains(groupID uint, domains []string) (inserted int, err error) {
	groupName := s.groupNameByID(groupID)
	for _, domainName := range domains {
		domainName = normalizeDomain(domainName)
		if domainName == "" {
			continue
		}
		if insErr := s.repository.AddBlockedDomain(groupID, domainName); insErr != nil {
			if err == nil {
				err = insErr
			}
			continue
		}
		inserted++
		s.broadcast(cluster.EventGroupBlockedAdd, map[string]interface{}{"groupId": groupID, "groupName": groupName, "domain": domainName})
	}
	if refreshErr := s.RefreshCache(); refreshErr != nil && err == nil {
		err = refreshErr
	}
	return inserted, err
}

func (s *Service) RemoveBlockedDomain(groupID uint, domainName string) error {
	domainName = normalizeDomain(domainName)
	if err := s.repository.RemoveBlockedDomain(groupID, domainName); err != nil {
		return err
	}
	s.broadcast(cluster.EventGroupBlockedRemove, map[string]interface{}{"groupId": groupID, "groupName": s.groupNameByID(groupID), "domain": domainName})
	return s.RefreshCache()
}

func (s *Service) AddAllowedDomain(groupID uint, domainName string) error {
	domainName = normalizeDomain(domainName)
	if domainName == "" {
		return fmt.Errorf("domain is required")
	}
	if err := s.repository.AddAllowedDomain(groupID, domainName); err != nil {
		return err
	}
	s.broadcast(cluster.EventGroupAllowedAdd, map[string]interface{}{"groupId": groupID, "groupName": s.groupNameByID(groupID), "domain": domainName})
	return s.RefreshCache()
}

// AddAllowedDomains is the allowlist counterpart of AddBlockedDomains.
func (s *Service) AddAllowedDomains(groupID uint, domains []string) (inserted int, err error) {
	groupName := s.groupNameByID(groupID)
	for _, domainName := range domains {
		domainName = normalizeDomain(domainName)
		if domainName == "" {
			continue
		}
		if insErr := s.repository.AddAllowedDomain(groupID, domainName); insErr != nil {
			if err == nil {
				err = insErr
			}
			continue
		}
		inserted++
		s.broadcast(cluster.EventGroupAllowedAdd, map[string]interface{}{"groupId": groupID, "groupName": groupName, "domain": domainName})
	}
	if refreshErr := s.RefreshCache(); refreshErr != nil && err == nil {
		err = refreshErr
	}
	return inserted, err
}

func (s *Service) RemoveAllowedDomain(groupID uint, domainName string) error {
	domainName = normalizeDomain(domainName)
	if err := s.repository.RemoveAllowedDomain(groupID, domainName); err != nil {
		return err
	}
	s.broadcast(cluster.EventGroupAllowedRemove, map[string]interface{}{"groupId": groupID, "groupName": s.groupNameByID(groupID), "domain": domainName})
	return s.RefreshCache()
}

// RecordClientIdentity registers an IP/MAC/hostname tuple for dual-stack IPv4/IPv6 correlation.
func (s *Service) RecordClientIdentity(ip, mac, hostname string) {
	s.dualStack.Record(ip, mac, hostname)
}

func (s *Service) getGroupIDsForClient(ip, mac string) []uint {
	correlatedIPs, resolvedMAC := s.dualStack.CorrelatedIdentifiers(ip, mac)

	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	ids := make([]uint, 0, 4)
	seen := map[uint]struct{}{}

	keys := make([]string, 0, 2+len(correlatedIPs))
	keys = append(keys, identifierKey(IdentifierTypeIP, ip), identifierKey(IdentifierTypeMAC, resolvedMAC))
	for _, peerIP := range correlatedIPs {
		keys = append(keys, identifierKey(IdentifierTypeIP, peerIP))
	}

	for _, key := range keys {
		for _, groupID := range s.assignments[key] {
			if _, ok := seen[groupID]; ok {
				continue
			}
			seen[groupID] = struct{}{}
			ids = append(ids, groupID)
		}
	}

	if len(ids) == 0 && s.defaultGroup != 0 {
		ids = append(ids, s.defaultGroup)
	}

	return ids
}

func (s *Service) GetEffectivePolicy(ip, mac string) EffectivePolicy {
	groupIDs := s.getGroupIDsForClient(ip, mac)

	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	policy := EffectivePolicy{GroupIDs: groupIDs}
	policy.GroupNames = make([]string, 0, len(groupIDs))
	for _, id := range groupIDs {
		group, ok := s.groupsByID[id]
		if !ok {
			continue
		}
		policy.GroupNames = append(policy.GroupNames, group.Name)
		if group.UseGlobalPolicies {
			policy.UsesGlobalBlocklist = true
		}
	}

	return policy
}

func (s *Service) GetAssignmentsByIdentifier(identifier, identifierType string) ([]uint, error) {
	rows, err := s.repository.GetAssignmentsByIdentifier(identifier, identifierType)
	if err != nil {
		return nil, err
	}
	groupIDs := make([]uint, 0, len(rows))
	for _, r := range rows {
		groupIDs = append(groupIDs, r.GroupID)
	}
	slices.Sort(groupIDs)
	return groupIDs, nil
}

func (s *Service) ShouldBlock(ip, mac, domainName, fullDomain string, globalBlocked, globalWhitelisted bool) bool {
	blocked, _, _ := s.ShouldBlockDetailed(ip, mac, domainName, fullDomain, globalBlocked, globalWhitelisted)
	return blocked
}

func (s *Service) ShouldBlockDetailed(ip, mac, domainName, fullDomain string, globalBlocked, globalWhitelisted bool) (bool, string, string) {
	groupIDs := s.getGroupIDsForClient(ip, mac)
	domainName = normalizeDomain(domainName)
	fullDomain = normalizeDomain(fullDomain)

	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	groupBlocked := false
	groupBlockedReason := ""
	groupAllowed := false
	groupAllowedReason := ""
	globalEnabledByGroup := false

	for _, groupID := range groupIDs {
		group, ok := s.groupsByID[groupID]
		if !ok {
			continue
		}

		if group.UseGlobalPolicies {
			globalEnabledByGroup = true
		}

		if matcher, ok := s.blockedByGroup[groupID]; ok {
			if matched, pattern := matcher.MatchDetailed(domainName); matched {
				groupBlocked = true
				groupBlockedReason = fmt.Sprintf("Group: %s, List: Blocked domains, Pattern: %s", group.Name, pattern)
			}
		}

		if matcher, ok := s.allowedByGroup[groupID]; ok {
			if matched, pattern := matcher.MatchDetailed(domainName); matched {
				groupAllowed = true
				groupAllowedReason = fmt.Sprintf("Group: %s, List: Allowed domains, Pattern: %s", group.Name, pattern)
			} else if matched, pattern := matcher.MatchDetailed(fullDomain); matched {
				groupAllowed = true
				groupAllowedReason = fmt.Sprintf("Group: %s, List: Allowed domains (full), Pattern: %s", group.Name, pattern)
			}
		}
	}

	if globalWhitelisted {
		return false, "Global Whitelist", fullDomain
	}
	if groupAllowed {
		return false, "Group Allowed", groupAllowedReason
	}

	if groupBlocked {
		return true, "Group Blocked", groupBlockedReason
	}

	if globalEnabledByGroup && globalBlocked {
		return true, "Global Blacklist", domainName
	}

	return false, "", ""
}
