package whitelist

import (
	"goaway/backend/domain"
	"goaway/backend/cluster"
	"goaway/backend/logging"
)

type Service struct {
	repository Repository
	Matcher    *domain.Matcher
	replicator cluster.Replicator
}

var log = logging.GetLogger()

func NewService(repo Repository) *Service {
	service := &Service{
		repository: repo,
		Matcher:    domain.NewMatcher(),
	}

	_, err := service.GetDomains() // Preload cache
	if err != nil {
		log.Warning("Could not preload domains cache, %v", err)
	}

	return service
}

func (s *Service) SetReplicator(r cluster.Replicator) {
	s.replicator = r
}

func (s *Service) AddDomain(d string) error {
	err := s.repository.AddDomain(d)
	if err != nil {
		return err
	}

	s.Matcher.Add(d)

	if s.replicator != nil {
		s.replicator.Broadcast(cluster.ReplicatedEvent{
			Type:    cluster.EventWhitelistAdd,
			Payload: map[string]string{"domain": d},
		})
	}

	return nil
}

func (s *Service) GetDomains() (map[string]bool, error) {
	domains, err := s.repository.GetDomains()
	if err != nil {
		return nil, err
	}

	s.Matcher = domain.NewMatcher()
	for d := range domains {
		s.Matcher.Add(d)
	}
	return domains, nil
}

func (s *Service) RemoveDomain(d string) error {
	err := s.repository.RemoveDomain(d)
	if err != nil {
		return err
	}

	s.Matcher.Remove(d)

	if s.replicator != nil {
		s.replicator.Broadcast(cluster.ReplicatedEvent{
			Type:    cluster.EventWhitelistRemove,
			Payload: map[string]string{"domain": d},
		})
	}

	return nil
}

func (s *Service) IsWhitelisted(d string) bool {
	whitelisted, _ := s.IsWhitelistedDetailed(d)
	return whitelisted
}

func (s *Service) IsWhitelistedDetailed(d string) (bool, string) {
	return s.Matcher.MatchDetailed(d)
}
