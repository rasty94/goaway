package policy

import (
	"fmt"
	"goaway/backend/database"

	"gorm.io/gorm"
)

type Repository interface {
	GetPolicies() ([]database.Policy, error)
	CreatePolicy(policy *database.Policy) error
	UpdatePolicy(policy *database.Policy) error
	DeletePolicy(id uint) error

	GetAssignments() ([]database.PolicyAssignment, error)
	CreateAssignment(assignment *database.PolicyAssignment) error
	DeleteAssignment(id uint) error

	GetSchedules() ([]database.Schedule, error)
	CreateSchedule(schedule *database.Schedule) error

	GetCategories(policyID uint) ([]string, error)
	SetCategories(policyID uint, categories []string) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) GetPolicies() ([]database.Policy, error) {
	var policies []database.Policy
	if err := r.db.Preload("Schedule").Order("priority DESC").Find(&policies).Error; err != nil {
		return nil, fmt.Errorf("failed to query policies: %w", err)
	}
	return policies, nil
}

func (r *repository) CreatePolicy(policy *database.Policy) error {
	return r.db.Create(policy).Error
}

func (r *repository) UpdatePolicy(policy *database.Policy) error {
	return r.db.Save(policy).Error
}

func (r *repository) DeletePolicy(id uint) error {
	return r.db.Delete(&database.Policy{}, id).Error
}

func (r *repository) GetAssignments() ([]database.PolicyAssignment, error) {
	var assignments []database.PolicyAssignment
	if err := r.db.Preload("Policy").Find(&assignments).Error; err != nil {
		return nil, err
	}
	return assignments, nil
}

func (r *repository) CreateAssignment(assignment *database.PolicyAssignment) error {
	return r.db.Create(assignment).Error
}

func (r *repository) DeleteAssignment(id uint) error {
	return r.db.Delete(&database.PolicyAssignment{}, id).Error
}

func (r *repository) GetSchedules() ([]database.Schedule, error) {
	var schedules []database.Schedule
	if err := r.db.Find(&schedules).Error; err != nil {
		return nil, err
	}
	return schedules, nil
}

func (r *repository) CreateSchedule(schedule *database.Schedule) error {
	return r.db.Create(schedule).Error
}

func (r *repository) GetCategories(policyID uint) ([]string, error) {
	var categories []database.PolicyCategory
	if err := r.db.Where("policy_id = ?", policyID).Find(&categories).Error; err != nil {
		return nil, err
	}
	res := make([]string, len(categories))
	for i, c := range categories {
		res[i] = c.Category
	}
	return res, nil
}

func (r *repository) SetCategories(policyID uint, categories []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("policy_id = ?", policyID).Delete(&database.PolicyCategory{}).Error; err != nil {
			return err
		}
		for _, cat := range categories {
			if err := tx.Create(&database.PolicyCategory{PolicyID: policyID, Category: cat}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
