package cluster

import (
	"os/exec"
	"runtime"
	"sync"
	"time"
)

type VipManager struct {
	cluster    *Service
	vip        string
	iface      string
	
	mu         sync.Mutex
	isHolding  bool
	stopCh     chan struct{}
}

func NewVipManager(cluster *Service, vip, iface string) *VipManager {
	return &VipManager{
		cluster: cluster,
		vip:     vip,
		iface:   iface,
		stopCh:  make(chan struct{}),
	}
}

func (v *VipManager) Start() {
	log.Info("[HA/VIP] Starting VIP Manager for %s on %s", v.vip, v.iface)
	go v.monitorLoop()
}

func (v *VipManager) Stop() {
	close(v.stopCh)
	v.Release()
}

func (v *VipManager) monitorLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-v.stopCh:
			return
		case <-ticker.C:
			// Check if we should hold the VIP
			shouldHold := v.cluster.selfRole == RolePrimary
			
			v.mu.Lock()
			if shouldHold && !v.isHolding {
				v.Takeover()
			} else if !shouldHold && v.isHolding {
				v.Release()
			}
			v.mu.Unlock()
		}
	}
}

func (v *VipManager) Takeover() {
	log.Info("[HA/VIP] Proclaming Leader: Taking over VIP %s", v.vip)
	
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		// macOS: ifconfig <iface> alias <ip> netmask 255.255.255.255
		cmd = exec.Command("ifconfig", v.iface, "alias", v.vip, "netmask", "255.255.255.255")
	} else {
		// Linux: ip addr add <ip>/32 dev <iface>
		cmd = exec.Command("ip", "addr", "add", v.vip+"/32", "dev", v.iface)
	}

	if err := cmd.Run(); err != nil {
		log.Error("[HA/VIP] Failed to takeover VIP: %v", err)
	} else {
		v.isHolding = true
		v.sendGratuitousARP()
	}
}

func (v *VipManager) Release() {
	log.Info("[HA/VIP] Releasing VIP %s", v.vip)

	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		// macOS: ifconfig <iface> -alias <ip>
		cmd = exec.Command("ifconfig", v.iface, "-alias", v.vip)
	} else {
		// Linux: ip addr del <ip>/32 dev <iface>
		cmd = exec.Command("ip", "addr", "del", v.vip+"/32", "dev", v.iface)
	}

	if err := cmd.Run(); err != nil {
		log.Debug("[HA/VIP] Note: VIP release might have failed (perhaps already gone): %v", err)
	}
	v.isHolding = false
}

func (v *VipManager) sendGratuitousARP() {
	// Send gratuitous ARP to update switch tables
	// On Linux we can use 'arping' if available
	if runtime.GOOS == "linux" {
		_ = exec.Command("arping", "-A", "-I", v.iface, "-c", "2", v.vip).Run()
	}
}
