package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/farhanhrywn/task-management-service/internal/domain"
)

type TeamRepository interface {
	List(ctx context.Context) ([]domain.Team, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Team, error)
	ListMembers(ctx context.Context, teamID uuid.UUID) ([]domain.TeamMemberView, error)
	// AddMember returns ErrDuplicate when the user is already a member.
	AddMember(ctx context.Context, m *domain.TeamMember) error
	RemoveMember(ctx context.Context, teamID, userID uuid.UUID) (removed bool, err error)
	// ShareTeam reports whether both users belong to at least one common team.
	// For userA == userB it is true when the user is in any team.
	ShareTeam(ctx context.Context, userA, userB uuid.UUID) (bool, error)
}

type teamRepository struct {
	db *gorm.DB
}

func NewTeamRepository(db *gorm.DB) TeamRepository {
	return &teamRepository{db: db}
}

func (r *teamRepository) List(ctx context.Context) ([]domain.Team, error) {
	teams := make([]domain.Team, 0)
	if err := dbFrom(ctx, r.db).Order("name").Find(&teams).Error; err != nil {
		return nil, translate(err)
	}
	return teams, nil
}

func (r *teamRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Team, error) {
	var t domain.Team
	if err := dbFrom(ctx, r.db).Where("id = ?", id).Take(&t).Error; err != nil {
		return nil, translate(err)
	}
	return &t, nil
}

// ListMembers joins users in a single query (no N+1).
func (r *teamRepository) ListMembers(ctx context.Context, teamID uuid.UUID) ([]domain.TeamMemberView, error) {
	members := make([]domain.TeamMemberView, 0)
	err := dbFrom(ctx, r.db).
		Table("team_members AS tm").
		Select("tm.user_id, u.name, u.email, tm.created_at AS joined_at").
		Joins("JOIN users u ON u.id = tm.user_id").
		Where("tm.team_id = ?", teamID).
		Order("u.name, u.email").
		Scan(&members).Error
	if err != nil {
		return nil, translate(err)
	}
	return members, nil
}

func (r *teamRepository) AddMember(ctx context.Context, m *domain.TeamMember) error {
	return translate(dbFrom(ctx, r.db).Create(m).Error)
}

func (r *teamRepository) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) (bool, error) {
	res := dbFrom(ctx, r.db).
		Where("team_id = ? AND user_id = ?", teamID, userID).
		Delete(&domain.TeamMember{})
	if res.Error != nil {
		return false, translate(res.Error)
	}
	return res.RowsAffected > 0, nil
}

func (r *teamRepository) ShareTeam(ctx context.Context, userA, userB uuid.UUID) (bool, error) {
	var shared bool
	err := dbFrom(ctx, r.db).Raw(`
		SELECT EXISTS (
			SELECT 1
			  FROM team_members a
			  JOIN team_members b ON a.team_id = b.team_id
			 WHERE a.user_id = ? AND b.user_id = ?
		)`, userA, userB).Scan(&shared).Error
	if err != nil {
		return false, translate(err)
	}
	return shared, nil
}
