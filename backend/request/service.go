package request

import (
	"context"
	"goaway/backend/api/models"
	model "goaway/backend/dns/server/models"
	"goaway/backend/logging"
	"goaway/backend/settings"
	"net"
	"time"
)

type Service struct {
	repository Repository
	config     *settings.Config
}

var log = logging.GetLogger()

func NewService(repo Repository, cfg *settings.Config) *Service {
	return &Service{repository: repo, config: cfg}
}

func (s *Service) SaveRequestLog(entries []model.RequestLogEntry) error {
	if s.config.Misc.AnonymizeIP {
		for i := range entries {
			if entries[i].ClientInfo != nil {
				entries[i].ClientInfo.IP = MaskIP(entries[i].ClientInfo.IP)
			}
		}
	}

	if err := s.repository.SaveRequestLog(entries); err != nil {
		return err
	}

	log.Debug("Saved %d new request log(s)", len(entries))
	return nil
}

func MaskIP(ip string) string {
	addr := net.ParseIP(ip)
	if addr == nil {
		return ip
	}
	if v4 := addr.To4(); v4 != nil {
		// Mask last octet
		v4[3] = 0
		return v4.String()
	}
	// Mask last 64 bits of IPv6
	for i := 8; i < 16; i++ {
		addr[i] = 0
	}
	return addr.String()
}

func (s *Service) GetClientNameFromIP(ip string) string {
	return s.repository.GetClientName(ip)
}

func (s *Service) GetDistinctRequestIP() int {
	return s.repository.GetDistinctRequestIP()
}

func (s *Service) GetRequestSummaryByInterval(interval int) ([]model.RequestLogIntervalSummary, error) {
	return s.repository.GetRequestSummaryByInterval(interval)
}

func (s *Service) GetResponseSizeSummaryByInterval(intervalMinutes int) ([]model.ResponseSizeSummary, error) {
	return s.repository.GetResponseSizeSummaryByInterval(intervalMinutes)
}

func (s *Service) GetUniqueQueryTypes() ([]models.QueryTypeCount, error) {
	return s.repository.GetUniqueQueryTypes()
}

func (s *Service) FetchQueries(q models.QueryParams) ([]model.RequestLogEntry, error) {
	return s.repository.FetchQueries(q)
}

func (s *Service) FetchClient(ip string) (*model.Client, error) {
	return s.repository.FetchClient(ip)
}

func (s *Service) FetchAllClients() (map[string]model.Client, error) {
	return s.repository.FetchAllClients()
}

func (s *Service) GetClientDetailsWithDomains(clientIP string) (ClientRequestDetails, string, map[string]int, error) {
	return s.repository.GetClientDetailsWithDomains(clientIP)
}

func (s *Service) GetClientHistory(clientIP string) ([]models.DomainHistory, error) {
	return s.repository.GetClientHistory(clientIP)
}

func (s *Service) GetTopBlockedDomains(blockedRequests int) ([]map[string]interface{}, error) {
	return s.repository.GetTopBlockedDomains(blockedRequests)
}

func (s *Service) GetTopPermittedDomains(permittedRequests int) ([]map[string]interface{}, error) {
	return s.repository.GetTopPermittedDomains(permittedRequests)
}

func (s *Service) GetTopQueriedDomains() ([]map[string]interface{}, error) {
	return s.repository.GetTopQueriedDomains()
}

func (s *Service) GetTopClients() ([]map[string]interface{}, error) {
	return s.repository.GetTopClients()
}

func (s *Service) CountQueries(search string) (int, error) {
	return s.repository.CountQueries(search)
}

func (s *Service) UpdateClientName(ip string, name string) error {
	if err := s.repository.UpdateClientName(ip, name); err != nil {
		return err
	}

	log.Info("Name changed to %s for client %s", name, ip)
	return nil
}

func (s *Service) UpdateClientBypass(ip string, bypass bool) error {
	if err := s.repository.UpdateClientBypass(ip, bypass); err != nil {
		return err
	}

	log.Info("Bypass toggled to %t for %s", bypass, ip)
	return nil
}

type vacuumFunc func(ctx context.Context)

func (s *Service) DeleteRequestLogsTimebased(vacuum vacuumFunc, requestThreshold, maxRetries int, retryDelay time.Duration) {
	if err := s.repository.DeleteRequestLogsTimebased(vacuum, requestThreshold, maxRetries, retryDelay); err != nil {
		log.Warning("Error while deleting old request logs: %v", err)
	}
}

func (s *Service) DeleteOldLogs(days int) error {
	return s.repository.DeleteOldLogs(days)
}
