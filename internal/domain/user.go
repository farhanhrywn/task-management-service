// Package domain holds the GORM entities and enums shared by every layer.
package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleUser       Role = "user"
	RoleSuperAdmin Role = "super_admin"
)

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name         string    `gorm:"type:varchar(100);not null"`
	Email        string    `gorm:"type:varchar(255);not null;uniqueIndex:uq_users_email"`
	PasswordHash string    `gorm:"type:varchar(255);not null"`
	Role         Role      `gorm:"type:varchar(20);not null;default:user;check:chk_users_role,role IN ('user','super_admin')"`
	CreatedAt    time.Time `gorm:"type:timestamptz(0);not null"`
	UpdatedAt    time.Time `gorm:"type:timestamptz(0);not null"`
}
