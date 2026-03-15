package resolution

import (
	"errors"
	"fmt"
	"goaway/backend/database"
	"strings"

	"gorm.io/gorm"
)

type Repository interface {
	CreateResolution(value, domain, recType string) error
	FindResolution(domain string) (database.Resolution, error)
	FindResolutions() ([]database.Resolution, error)
	DeleteResolution(value, domain string) (int, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateResolution(value, domain, recType string) error {
	res := database.Resolution{
		Domain: domain,
		Value:  value,
		Type:   recType,
	}

	if err := r.db.Create(&res).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return errors.New("domain already exists, must be unique")
		}
		return fmt.Errorf("could not create new resolution: %w", err)
	}
	return nil
}

func (r *repository) FindResolution(domain string) (database.Resolution, error) {
	var res database.Resolution

	r.db.Where("domain = ?", domain).Find(&res)
	if res.Value != "" {
		return res, nil
	}

	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		wildcardDomain := "*." + strings.Join(parts[i:], ".")
		if err := r.db.Where("domain = ?", wildcardDomain).Find(&res).Error; err == nil && res.Value != "" {
			return res, nil
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return database.Resolution{}, err
		}
	}

	return database.Resolution{}, nil
}

func (r *repository) FindResolutions() ([]database.Resolution, error) {
	var resolutions []database.Resolution
	if err := r.db.Find(&resolutions).Error; err != nil {
		return nil, err
	}
	return resolutions, nil
}

func (r *repository) DeleteResolution(value, domain string) (int, error) {
	result := r.db.Where("domain = ? AND value = ?", domain, value).Delete(&database.Resolution{})
	if result.Error != nil {
		return 0, result.Error
	}
	return int(result.RowsAffected), nil
}
