package group

import (
	"fmt"
	"goaway/backend/database"
	"strings"

	"gorm.io/gorm"
)

type Repository interface {
	GetGroups() ([]database.ClientGroup, error)
	CreateGroup(group *database.ClientGroup) error
	DeleteGroup(id uint) error
	SetGroupUseGlobalPolicies(id uint, enabled bool) error

	ReplaceAssignments(identifier, identifierType string, groupIDs []uint) error
	GetAssignments() ([]database.ClientGroupAssignment, error)
	GetAssignmentsByIdentifier(identifier, identifierType string) ([]database.ClientGroupAssignment, error)

	AddBlockedDomain(groupID uint, domain string) error
	RemoveBlockedDomain(groupID uint, domain string) error
	GetBlockedDomains() ([]database.GroupBlockedDomain, error)

	AddAllowedDomain(groupID uint, domain string) error
	RemoveAllowedDomain(groupID uint, domain string) error
	GetAllowedDomains() ([]database.GroupAllowedDomain, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) GetGroups() ([]database.ClientGroup, error) {
	var groups []database.ClientGroup
	if err := r.db.Order("is_default DESC, name ASC").Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("failed to query groups: %w", err)
	}
	return groups, nil
}

func (r *repository) CreateGroup(group *database.ClientGroup) error {
	if err := r.db.Create(group).Error; err != nil {
		return fmt.Errorf("failed to create group: %w", err)
	}
	return nil
}

func (r *repository) DeleteGroup(id uint) error {
	result := r.db.Where("id = ? AND is_default = ?", id, false).Delete(&database.ClientGroup{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete group: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("group not found or cannot delete default group")
	}
	return nil
}

func (r *repository) SetGroupUseGlobalPolicies(id uint, enabled bool) error {
	result := r.db.Model(&database.ClientGroup{}).Where("id = ?", id).Update("use_global_policies", enabled)
	if result.Error != nil {
		return fmt.Errorf("failed to update group policy: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("group not found")
	}
	return nil
}

func (r *repository) ReplaceAssignments(identifier, identifierType string, groupIDs []uint) error {
	identifier = strings.TrimSpace(strings.ToLower(identifier))
	identifierType = strings.TrimSpace(strings.ToLower(identifierType))

	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("identifier = ? AND identifier_type = ?", identifier, identifierType).Delete(&database.ClientGroupAssignment{}).Error; err != nil {
			return fmt.Errorf("failed to clear assignments: %w", err)
		}

		for _, groupID := range groupIDs {
			assignment := database.ClientGroupAssignment{
				Identifier:     identifier,
				IdentifierType: identifierType,
				GroupID:        groupID,
			}
			if err := tx.Create(&assignment).Error; err != nil {
				return fmt.Errorf("failed to assign group %d: %w", groupID, err)
			}
		}

		return nil
	})
}

func (r *repository) GetAssignments() ([]database.ClientGroupAssignment, error) {
	var rows []database.ClientGroupAssignment
	if err := r.db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query assignments: %w", err)
	}
	return rows, nil
}

func (r *repository) GetAssignmentsByIdentifier(identifier, identifierType string) ([]database.ClientGroupAssignment, error) {
	var rows []database.ClientGroupAssignment
	if err := r.db.Where("identifier = ? AND identifier_type = ?", strings.ToLower(identifier), strings.ToLower(identifierType)).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query assignments: %w", err)
	}
	return rows, nil
}

func (r *repository) AddBlockedDomain(groupID uint, domain string) error {
	row := database.GroupBlockedDomain{GroupID: groupID, Domain: domain}
	if err := r.db.Create(&row).Error; err != nil {
		return fmt.Errorf("failed to add blocked domain: %w", err)
	}
	return nil
}

func (r *repository) RemoveBlockedDomain(groupID uint, domain string) error {
	result := r.db.Where("group_id = ? AND domain = ?", groupID, domain).Delete(&database.GroupBlockedDomain{})
	if result.Error != nil {
		return fmt.Errorf("failed to remove blocked domain: %w", result.Error)
	}
	return nil
}

func (r *repository) GetBlockedDomains() ([]database.GroupBlockedDomain, error) {
	var rows []database.GroupBlockedDomain
	if err := r.db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query blocked domains: %w", err)
	}
	return rows, nil
}

func (r *repository) AddAllowedDomain(groupID uint, domain string) error {
	row := database.GroupAllowedDomain{GroupID: groupID, Domain: domain}
	if err := r.db.Create(&row).Error; err != nil {
		return fmt.Errorf("failed to add allowed domain: %w", err)
	}
	return nil
}

func (r *repository) RemoveAllowedDomain(groupID uint, domain string) error {
	result := r.db.Where("group_id = ? AND domain = ?", groupID, domain).Delete(&database.GroupAllowedDomain{})
	if result.Error != nil {
		return fmt.Errorf("failed to remove allowed domain: %w", result.Error)
	}
	return nil
}

func (r *repository) GetAllowedDomains() ([]database.GroupAllowedDomain, error) {
	var rows []database.GroupAllowedDomain
	if err := r.db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query allowed domains: %w", err)
	}
	return rows, nil
}
