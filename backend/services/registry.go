package services

import (
	"embed"
	"fmt"
	"goaway/backend/api"
	"goaway/backend/api/key"
	"goaway/backend/audit"
	"goaway/backend/blacklist"
	"goaway/backend/dhcp"
	"goaway/backend/group"
	"goaway/backend/logging"
	"goaway/backend/notification"
	"goaway/backend/policy"
	"goaway/backend/prefetch"
	"goaway/backend/request"
	"goaway/backend/resolution"
	"goaway/backend/sync"
	"goaway/backend/cluster"
	"goaway/backend/user"
	"goaway/backend/whitelist"
	"net/http"
	synchronization "sync"

	"github.com/miekg/dns"
)

var log = logging.GetLogger()

// Manages all servers and services
type ServiceRegistry struct {
	APIServer *api.API
	errorChan chan ServiceError
	readyChan chan struct{}
	content   embed.FS

	UDPServer *dns.Server
	TCPServer *dns.Server
	DoTServer *dns.Server
	DoHServer *http.Server

	Context *AppContext

	version string
	date    string
	commit  string
	wg      synchronization.WaitGroup

	ResolutionService   *resolution.Service
	RequestService      *request.Service
	AuditService        *audit.Service
	PrefetchService     *prefetch.Service
	UserService         *user.Service
	KeyService          *key.Service
	NotificationService *notification.Service
	BlacklistService    *blacklist.Service
	DHCPService         *dhcp.Service
	GroupService        *group.Service
	PolicyService       *policy.Service
	WhitelistService    *whitelist.Service
	ClusterService      *cluster.Service
}

type ServiceError struct {
	Err     error
	Service string
}

func NewServiceRegistry(ctx *AppContext, version, commit, date string, content embed.FS) *ServiceRegistry {
	return &ServiceRegistry{
		Context:   ctx,
		version:   version,
		commit:    commit,
		date:      date,
		content:   content,
		readyChan: make(chan struct{}),
		errorChan: make(chan ServiceError, 10),
	}
}

func (r *ServiceRegistry) Initialize() error {
	r.setupDNSServers()

	if r.Context.Certificate.Certificate != nil {
		if err := r.setupSecureServers(); err != nil {
			return err
		}
	}

	r.setupAPIServer()

	return nil
}

func (r *ServiceRegistry) setupDNSServers() {
	config := r.Context.Config

	notifyReady := func() {
		log.Info("Started DNS server on: %s:%d", config.DNS.Address, config.DNS.Ports.TCPUDP)
		close(r.readyChan)
	}

	r.UDPServer = &dns.Server{
		Addr:      fmt.Sprintf("%s:%d", config.DNS.Address, config.DNS.Ports.TCPUDP),
		Net:       "udp",
		Handler:   r.Context.DNSServer,
		ReusePort: true,
		UDPSize:   config.DNS.UDPSize,
	}

	r.TCPServer = &dns.Server{
		Addr:              fmt.Sprintf("%s:%d", config.DNS.Address, config.DNS.Ports.TCPUDP),
		Net:               "tcp",
		Handler:           r.Context.DNSServer,
		ReusePort:         true,
		UDPSize:           config.DNS.UDPSize,
		NotifyStartedFunc: notifyReady,
	}
}

func (r *ServiceRegistry) setupSecureServers() error {
	dotServer, err := r.Context.DNSServer.InitDoT(r.Context.Certificate)
	if err != nil {
		return fmt.Errorf("failed to initialize DoT server: %w", err)
	}
	r.DoTServer = dotServer

	dohServer, err := r.Context.DNSServer.InitDoH(r.Context.Certificate)
	if err != nil {
		return fmt.Errorf("failed to initialize DoH server: %w", err)
	}
	r.DoHServer = dohServer

	return nil
}

func (r *ServiceRegistry) setupAPIServer() {
	r.APIServer = &api.API{
		DNS:            r.Context.DNSServer,
		Authentication: r.Context.Config.API.Authentication,
		Config:         r.Context.Config,
		DNSPort:        r.Context.Config.DNS.Ports.TCPUDP,
		Version:        r.version,
		Commit:         r.commit,
		Date:           r.date,
		DNSServer:      r.Context.DNSServer,
		DBConn:         r.Context.DBConn,

		ResolutionService:   r.ResolutionService,
		RequestService:      r.RequestService,
		AuditService:        r.AuditService,
		PrefetchService:     r.PrefetchService,
		NotificationService: r.NotificationService,
		UserService:         r.UserService,
		KeyService:          r.KeyService,
		DHCPService:         r.DHCPService,
		BlacklistService:    r.BlacklistService,
		GroupService:        r.GroupService,
		PolicyService:       r.PolicyService,
		WhitelistService:    r.WhitelistService,
	}

	// Initialize replica sync manager for HA support
	r.APIServer.ReplicaSyncManager = sync.NewReplicaSyncManager(r.Context.Config, r.APIServer)

	r.ClusterService = cluster.NewService(r.Context.Config, "local-node") // Using temporary fixed ID
	r.APIServer.ClusterManager = r.ClusterService
	
	r.BlacklistService.SetReplicator(r.ClusterService)
	r.WhitelistService.SetReplicator(r.ClusterService)
	r.GroupService.SetReplicator(r.ClusterService)
}

func (r *ServiceRegistry) StartAll() {
	r.startDNSServers()

	if r.Context.Certificate.Certificate != nil {
		r.startSecureServers()
	}

	r.startDHCPService()
	r.startAPIServer()
}

func (r *ServiceRegistry) startDNSServers() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		if err := r.UDPServer.ListenAndServe(); err != nil {
			r.errorChan <- ServiceError{Service: "UDP", Err: err}
		}
	}()

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		if err := r.TCPServer.ListenAndServe(); err != nil {
			r.errorChan <- ServiceError{Service: "TCP", Err: err}
		}
	}()
}

func (r *ServiceRegistry) startSecureServers() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		if err := r.DoTServer.ListenAndServe(); err != nil {
			r.errorChan <- ServiceError{Service: "DoT", Err: err}
		}
	}()

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		if serverIP, err := api.GetServerIP(); err == nil {
			log.Info("DoH (dns-over-https) server running at https://%s:%d/dns-query",
				serverIP, r.Context.Config.DNS.Ports.DoH)
		} else {
			log.Info("DoH (dns-over-https) server running on port :%d", r.Context.Config.DNS.Ports.DoH)
		}

		if err := r.DoHServer.ListenAndServeTLS(
			r.Context.Config.DNS.TLS.Cert,
			r.Context.Config.DNS.TLS.Key,
		); err != nil {
			r.errorChan <- ServiceError{Service: "DoH", Err: err}
		}
	}()
}

func (r *ServiceRegistry) startDHCPService() {
	if r.DHCPService == nil {
		return
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		if err := r.DHCPService.Start(); err != nil {
			r.errorChan <- ServiceError{Service: "DHCP", Err: err}
		}
	}()
}

func (r *ServiceRegistry) startAPIServer() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		<-r.readyChan

		errorChan := make(chan struct{}, 1)
		go func() {
			<-errorChan
			r.errorChan <- ServiceError{Service: "API", Err: fmt.Errorf("API server stopped")}
		}()

		// Start replica sync manager for HA support
		if r.APIServer.ReplicaSyncManager != nil {
			r.APIServer.ReplicaSyncManager.Start()
		}

		// Start the new active clustering service
		if r.ClusterService != nil {
			r.ClusterService.Start()
		}

		r.APIServer.Start(r.content, errorChan)
	}()
}

func (r *ServiceRegistry) WaitGroup() *synchronization.WaitGroup {
	return &r.wg
}

func (r *ServiceRegistry) ReadyChannel() <-chan struct{} {
	return r.readyChan
}

func (r *ServiceRegistry) ErrorChannel() <-chan ServiceError {
	return r.errorChan
}

func (r *ServiceRegistry) GetPrefetcher() *prefetch.Service {
	return r.APIServer.PrefetchService
}
