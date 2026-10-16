package domain

import (
	"time"

	"github.com/google/uuid"
)

type Team struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name      string    `gorm:"type:varchar(100);not null;uniqueIndex:uq_teams_name"`
	CreatedAt time.Time `gorm:"type:timestamptz(0);not null"`
	UpdatedAt time.Time `gorm:"type:timestamptz(0);not null"`
}

// TeamMember is the many-to-many join between teams and users.
// CreatedAt is the time the user joined the team.
type TeamMember struct {
	TeamID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey;index:idx_team_members_user_id"`
	CreatedAt time.Time `gorm:"type:timestamptz(0);not null"`

	Team *Team `gorm:"foreignKey:TeamID;constraint:OnDelete:CASCADE"`
	User *User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}

// TeamMemberView is a read model joining team_members with users.
type TeamMemberView struct {
	UserID   uuid.UUID
	Name     string
	Email    string
	JoinedAt time.Time
}
