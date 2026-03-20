package dhcp

import (
	"fmt"
	"goaway/backend/database"
	"strings"

	"gorm.io/gorm"
)

type Repository interface {
	ListStaticLeases() ([]database.StaticDHCPLease, error)
	CreateStaticLease(lease *database.StaticDHCPLease) error
	UpdateStaticLease(id uint, lease *database.StaticDHCPLease) error
	DeleteStaticLease(id uint) error

	ListActiveLeases() ([]database.ActiveDHCPLease, error)
	CreateOrUpdateActiveLease(lease *database.ActiveDHCPLease) error
	DeleteExpiredLeases() error

	// DHCPv6
	ListStaticv6Leases() ([]database.StaticDHCPv6Lease, error)
	CreateStaticv6Lease(lease *database.StaticDHCPv6Lease) error
	UpdateStaticv6Lease(id uint, lease *database.StaticDHCPv6Lease) error
	DeleteStaticv6Lease(id uint) error

	ListActivev6Leases() ([]database.ActiveDHCPv6Lease, error)
	CreateOrUpdateActivev6Lease(lease *database.ActiveDHCPv6Lease) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) ListStaticLeases() ([]database.StaticDHCPLease, error) {
	var leases []database.StaticDHCPLease
	if err := r.db.Order("created_at DESC").Find(&leases).Error; err != nil {
		return nil, fmt.Errorf("failed to query static DHCP leases: %w", err)
	}
	return leases, nil
}

func (r *repository) CreateStaticLease(lease *database.StaticDHCPLease) error {
	lease.MAC = strings.ToLower(strings.TrimSpace(lease.MAC))
	lease.IP = strings.TrimSpace(lease.IP)
	lease.Hostname = strings.TrimSpace(lease.Hostname)
	if err := r.db.Create(lease).Error; err != nil {
		return fmt.Errorf("failed to create static DHCP lease: %w", err)
	}
	return nil
}

func (r *repository) UpdateStaticLease(id uint, lease *database.StaticDHCPLease) error {
	updates := map[string]interface{}{
		"mac":      strings.ToLower(strings.TrimSpace(lease.MAC)),
		"ip":       strings.TrimSpace(lease.IP),
		"hostname": strings.TrimSpace(lease.Hostname),
		"enabled":  lease.Enabled,
	}

	result := r.db.Model(&database.StaticDHCPLease{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update static DHCP lease: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("static DHCP lease not found")
	}
	return nil
}

func (r *repository) DeleteStaticLease(id uint) error {
	result := r.db.Where("id = ?", id).Delete(&database.StaticDHCPLease{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete static DHCP lease: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("static DHCP lease not found")
	}
	return nil
}

func (r *repository) ListActiveLeases() ([]database.ActiveDHCPLease, error) {
	var leases []database.ActiveDHCPLease
	if err := r.db.Order("expires_at ASC").Find(&leases).Error; err != nil {
		return nil, fmt.Errorf("failed to query active DHCP leases: %w", err)
	}
	return leases, nil
}

func (r *repository) CreateOrUpdateActiveLease(lease *database.ActiveDHCPLease) error {
	lease.MAC = strings.ToLower(strings.TrimSpace(lease.MAC))
	lease.IP = strings.TrimSpace(lease.IP)
	lease.Hostname = strings.TrimSpace(lease.Hostname)

	// Upsert based on MAC
	result := r.db.Where("mac = ?", lease.MAC).Assign(database.ActiveDHCPLease{
		IP:        lease.IP,
		Hostname:  lease.Hostname,
		ExpiresAt: lease.ExpiresAt,
	}).FirstOrCreate(lease)

	if result.Error != nil {
		return fmt.Errorf("failed to upsert active DHCP lease: %w", result.Error)
	}
	return nil
}

func (r *repository) DeleteExpiredLeases() error {
	err1 := r.db.Where("expires_at < CURRENT_TIMESTAMP").Delete(&database.ActiveDHCPLease{}).Error
	err2 := r.db.Where("expires_at < CURRENT_TIMESTAMP").Delete(&database.ActiveDHCPv6Lease{}).Error
	if err1 != nil {
		return err1
	}
	return err2
}

func (r *repository) ListStaticv6Leases() ([]database.StaticDHCPv6Lease, error) {
	var leases []database.StaticDHCPv6Lease
	if err := r.db.Order("created_at DESC").Find(&leases).Error; err != nil {
		return nil, fmt.Errorf("failed to query static DHCPv6 leases: %w", err)
	}
	return leases, nil
}

func (r *repository) CreateStaticv6Lease(lease *database.StaticDHCPv6Lease) error {
	lease.DUID = strings.TrimSpace(lease.DUID)
	lease.IP = strings.TrimSpace(lease.IP)
	lease.Hostname = strings.TrimSpace(lease.Hostname)
	if err := r.db.Create(lease).Error; err != nil {
		return fmt.Errorf("failed to create static DHCPv6 lease: %w", err)
	}
	return nil
}

func (r *repository) UpdateStaticv6Lease(id uint, lease *database.StaticDHCPv6Lease) error {
	updates := map[string]interface{}{
		"duid":     strings.TrimSpace(lease.DUID),
		"ip":       strings.TrimSpace(lease.IP),
		"hostname": strings.TrimSpace(lease.Hostname),
		"enabled":  lease.Enabled,
	}

	result := r.db.Model(&database.StaticDHCPv6Lease{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update static DHCPv6 lease: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("static DHCPv6 lease not found")
	}
	return nil
}

func (r *repository) DeleteStaticv6Lease(id uint) error {
	result := r.db.Where("id = ?", id).Delete(&database.StaticDHCPv6Lease{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete static DHCPv6 lease: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("static DHCPv6 lease not found")
	}
	return nil
}

func (r *repository) ListActivev6Leases() ([]database.ActiveDHCPv6Lease, error) {
	var leases []database.ActiveDHCPv6Lease
	if err := r.db.Order("expires_at ASC").Find(&leases).Error; err != nil {
		return nil, fmt.Errorf("failed to query active DHCPv6 leases: %w", err)
	}
	return leases, nil
}

func (r *repository) CreateOrUpdateActivev6Lease(lease *database.ActiveDHCPv6Lease) error {
	lease.DUID = strings.TrimSpace(lease.DUID)
	lease.IP = strings.TrimSpace(lease.IP)
	lease.Hostname = strings.TrimSpace(lease.Hostname)

	// Upsert based on DUID/IAID
	result := r.db.Where("duid = ? AND iaid = ?", lease.DUID, lease.IAID).Assign(database.ActiveDHCPv6Lease{
		IP:        lease.IP,
		Hostname:  lease.Hostname,
		ExpiresAt: lease.ExpiresAt,
	}).FirstOrCreate(lease)

	if result.Error != nil {
		return fmt.Errorf("failed to upsert active DHCPv6 lease: %w", result.Error)
	}
	return nil
}

