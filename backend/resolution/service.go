package resolution

import (
	"goaway/backend/cluster"
	"goaway/backend/database"
	"goaway/backend/logging"
)

type Service struct {
	repository Repository
	replicator cluster.Replicator
}

var log = logging.GetLogger()

func NewService(repo Repository) *Service {
	return &Service{repository: repo}
}

func (s *Service) SetReplicator(r cluster.Replicator) {
	s.replicator = r
}

func (s *Service) broadcast(t cluster.EventType, payload interface{}) {
	if s.replicator != nil {
		s.replicator.Broadcast(cluster.ReplicatedEvent{Type: t, Payload: payload})
	}
}

func (s *Service) CreateResolution(value, domain, recType string) error {
	if recType == "" {
		recType = "A"
	}
	log.Debug("Creating new resolution '%s' -> '%s' (%s)", domain, value, recType)
	if err := s.repository.CreateResolution(value, domain, recType); err != nil {
		return err
	}
	s.broadcast(cluster.EventResolutionAdd, map[string]string{"domain": domain, "value": value, "type": recType})
	return nil
}

func (s *Service) GetResolution(domain string) (database.Resolution, error) {
	log.Debug("Finding resolution for domain: %s", domain)
	return s.repository.FindResolution(domain)
}

func (s *Service) GetResolutions() ([]database.Resolution, error) {
	return s.repository.FindResolutions()
}

func (s *Service) DeleteResolution(value, domain string) (int, error) {
	n, err := s.repository.DeleteResolution(value, domain)
	if err == nil && n > 0 {
		s.broadcast(cluster.EventResolutionRemove, map[string]string{"domain": domain, "value": value})
	}
	return n, err
}
