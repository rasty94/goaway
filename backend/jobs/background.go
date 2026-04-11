package jobs

import (
	arp "goaway/backend/dns"
	"goaway/backend/logging"
	"goaway/backend/services"
	"time"
)

var log = logging.GetLogger()

type BackgroundJobs struct {
	registry *services.ServiceRegistry
}

func NewBackgroundJobs(registry *services.ServiceRegistry) *BackgroundJobs {
	return &BackgroundJobs{
		registry: registry,
	}
}

func (b *BackgroundJobs) Start(readyChan <-chan struct{}) {
	b.startHostnameCachePopulation()
	b.cleanVendorResponseCache(readyChan)
	b.startARPProcessing(readyChan)
	b.startScheduledUpdates(readyChan)
	b.startCacheCleanup(readyChan)
	b.startPrefetcher(readyChan)
	b.startLogRetentionCleanup(readyChan)
	b.startUpstreamHealthProber(readyChan)
}

func (b *BackgroundJobs) startHostnameCachePopulation() {
	if err := b.registry.Context.DNSServer.PopulateClientCaches(); err != nil {
		log.Warning("Unable to populate hostname cache: %s", err)
	}
}

func (b *BackgroundJobs) startARPProcessing(readyChan <-chan struct{}) {
	go func() {
		<-readyChan
		log.Debug("Starting ARP table processing...")
		arp.ProcessARPTable()
	}()
}

func (b *BackgroundJobs) cleanVendorResponseCache(readyChan <-chan struct{}) {
	go func() {
		<-readyChan
		log.Debug("Starting vendor response table processing...")
		arp.CleanVendorResponseCache()
	}()
}

func (b *BackgroundJobs) startScheduledUpdates(readyChan <-chan struct{}) {
	go func() {
		<-readyChan
		if b.registry.Context.Config.Misc.ScheduledBlacklistUpdates {
			log.Debug("Starting scheduler for automatic list updates...")
			b.registry.BlacklistService.ScheduleAutomaticListUpdates()
		}
	}()
}

func (b *BackgroundJobs) startCacheCleanup(readyChan <-chan struct{}) {
	go func() {
		<-readyChan
		log.Debug("Starting cache cleanup routine...")
		b.registry.Context.DNSServer.ClearOldEntries()
	}()
}

func (b *BackgroundJobs) startPrefetcher(readyChan <-chan struct{}) {
	go func() {
		<-readyChan
		log.Debug("Starting prefetcher...")
		b.registry.PrefetchService.Run()
	}()
}
func (b *BackgroundJobs) startLogRetentionCleanup(readyChan <-chan struct{}) {
	go func() {
		<-readyChan
		log.Debug("Starting log retention cleanup routine...")
		
		// Run every hour
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			retentionDays := b.registry.Context.Config.Misc.StatisticsRetention
			if retentionDays > 0 {
				log.Info("Cleaning up logs older than %d days...", retentionDays)
				if err := b.registry.RequestService.DeleteOldLogs(retentionDays); err != nil {
					log.Error("Failed to clean up old logs: %v", err)
				}
			}
		}
	}()
}

func (b *BackgroundJobs) startUpstreamHealthProber(readyChan <-chan struct{}) {
	go func() {
		<-readyChan
		log.Debug("Starting upstream health prober...")
		
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			upstreams := b.registry.Context.Config.DNS.Upstream.Servers
			for _, u := range upstreams {
				if u.Enabled {
					go b.registry.Context.DNSServer.ProbeUpstream(u.Address)
				}
			}
		}
	}()
}
