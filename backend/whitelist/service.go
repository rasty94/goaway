package whitelist

import (
	"goaway/backend/domain"
	"goaway/backend/logging"
)

type Service struct {
	repository Repository
	Matcher    *domain.Matcher
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

func (s *Service) AddDomain(d string) error {
	err := s.repository.AddDomain(d)
	if err != nil {
		return err
	}

	s.Matcher.Add(d)
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
	return nil
}

func (s *Service) IsWhitelisted(d string) bool {
	return s.Matcher.Match(d)
}
