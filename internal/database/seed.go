package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/farhanhrywn/task-management-service/internal/config"
	"github.com/farhanhrywn/task-management-service/internal/domain"
)

var seedTeams = []string{"Team Alpha", "Team Beta"}

// Seed inserts the default teams and the super admin account. It is safe to
// run on every startup: existing rows are left untouched (ON CONFLICT DO NOTHING).
func Seed(ctx context.Context, db *gorm.DB, admin config.SuperAdminConfig) error {
	db = db.WithContext(ctx)

	for _, name := range seedTeams {
		team := domain.Team{ID: uuid.New(), Name: name}
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "name"}},
			DoNothing: true,
		}).Create(&team).Error; err != nil {
			return fmt.Errorf("seed team %q: %w", name, err)
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(admin.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash super admin password: %w", err)
	}
	user := domain.User{
		ID:           uuid.New(),
		Name:         admin.Name,
		Email:        strings.ToLower(strings.TrimSpace(admin.Email)),
		PasswordHash: string(hash),
		Role:         domain.RoleSuperAdmin,
	}
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "email"}},
		DoNothing: true,
	}).Create(&user).Error; err != nil {
		return fmt.Errorf("seed super admin: %w", err)
	}
	return nil
}
