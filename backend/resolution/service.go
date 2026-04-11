package resolution

import (
	"goaway/backend/database"
	"goaway/backend/logging"
)

type Service struct {
	repository Repository
}

var log = logging.GetLogger()

func NewService(repo Repository) *Service {
	return &Service{repository: repo}
}

func (s *Service) CreateResolution(value, domain, recType string) error {
	if recType == "" {
		recType = "A"
	}
	log.Debug("Creating new resolution '%s' -> '%s' (%s)", domain, value, recType)
	return s.repository.CreateResolution(value, domain, recType)
}

func (s *Service) GetResolution(domain string) (database.Resolution, error) {
	log.Debug("Finding resolution for domain: %s", domain)
	return s.repository.FindResolution(domain)
}

func (s *Service) GetResolutions() ([]database.Resolution, error) {
	return s.repository.FindResolutions()
}

func (s *Service) DeleteResolution(value, domain string) (int, error) {
	return s.repository.DeleteResolution(value, domain)
}
