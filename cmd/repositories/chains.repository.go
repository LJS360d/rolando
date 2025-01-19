package repositories

import (
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type Chain struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"unique;not null" json:"name"`
	ReplyRate   int       `gorm:"default:10" json:"reply_rate"`
	MaxSizeMb   int       `gorm:"default:25" json:"max_size_mb"`
	TTSLanguage string    `gorm:"default:'en'" json:"tts_language"`
	Pings       bool      `gorm:"default:true" json:"pings"`
	Trained     bool      `gorm:"default:false" json:"trained"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type ChainsRepository struct {
	DB *gorm.DB
}

func NewChainsRepository(dbPath string) (*ChainsRepository, error) {
	// Open SQLite database
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Migrate the schema (creates the tables if they don't exist)
	if err := db.AutoMigrate(&Chain{}); err != nil {
		return nil, err
	}
	return &ChainsRepository{DB: db}, nil
}

// CreateChain creates a new Markov Chain in the database
func (repo *ChainsRepository) CreateChain(id string, name string) (*Chain, error) {
	chain := &Chain{
		ID:          id,
		Name:        name,
		ReplyRate:   10,    // Default reply rate
		Pings:       true,  // Default pings
		Trained:     false, // Default trained
		MaxSizeMb:   25,    // Default max size in MB
		TTSLanguage: "en",
	}

	// Use GORM to insert the chain record
	if err := repo.DB.Create(chain).Error; err != nil {
		return nil, err
	}

	return chain, nil
}

func (repo *ChainsRepository) GetAll() ([]*Chain, error) {
	var chains []*Chain
	if err := repo.DB.Find(&chains).Error; err != nil {
		return nil, err
	}
	return chains, nil
}

// GetChainByID retrieves a chain by its ID
func (repo *ChainsRepository) GetChainByID(id string) (*Chain, error) {
	var chain Chain
	if err := repo.DB.First(&chain, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &chain, nil
}

// UpdateChain updates the properties of an existing chain
func (repo *ChainsRepository) UpdateChain(id string, fields map[string]any) (*Chain, error) {
	var chain Chain
	if err := repo.DB.First(&chain, "id = ?", id).Error; err != nil {
		return nil, err
	}

	// Update the chain's fields using GORM's updates
	if err := repo.DB.Model(&chain).Updates(fields).Error; err != nil {
		return nil, err
	}

	return &chain, nil
}

// GetChainsPage fetches chains with pagination and returns metadata
func (repo *ChainsRepository) GetChainsPage(limit, offset int) ([]Chain, int64, error) {
	var elements []Chain
	var total int64

	// Count total elements
	if err := repo.DB.Model(&Chain{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch paginated elements
	if err := repo.DB.
		Limit(limit).
		Offset(offset).
		Find(&elements).Error; err != nil {
		return nil, 0, err
	}

	return elements, total, nil
}

// DeleteChain deletes a chain by its ID
func (repo *ChainsRepository) DeleteChain(id string) error {
	if err := repo.DB.Delete(&Chain{}, "id = ?", id).Error; err != nil {
		return err
	}
	return nil
}

// CountChains counts the total number of chains in the database
func (repo *ChainsRepository) CountChains() (int64, error) {
	var count int64
	if err := repo.DB.Model(&Chain{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
