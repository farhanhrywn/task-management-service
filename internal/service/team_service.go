package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/repository"
	"github.com/farhanhrywn/task-management-service/pkg/apperror"
	"github.com/farhanhrywn/task-management-service/pkg/clock"
)

// TeamService manages team membership. Teams themselves only come from seed.
type TeamService struct {
	teams repository.TeamRepository
	users repository.UserRepository
	clock clock.Clock
}

func NewTeamService(teams repository.TeamRepository, users repository.UserRepository, clk clock.Clock) *TeamService {
	return &TeamService{teams: teams, users: users, clock: clk}
}

func (s *TeamService) List(ctx context.Context) ([]domain.Team, error) {
	teams, err := s.teams.List(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return teams, nil
}

func (s *TeamService) ListMembers(ctx context.Context, teamID uuid.UUID) ([]domain.TeamMemberView, error) {
	if err := s.ensureTeam(ctx, teamID); err != nil {
		return nil, err
	}
	members, err := s.teams.ListMembers(ctx, teamID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return members, nil
}

// AddMember: team must exist (404), user must exist (404), and not already be
// a member (409, enforced by the primary key).
func (s *TeamService) AddMember(ctx context.Context, teamID, userID uuid.UUID) (*domain.TeamMember, error) {
	if err := s.ensureTeam(ctx, teamID); err != nil {
		return nil, err
	}
	if _, err := s.users.FindByID(ctx, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperror.UserNotFound()
		}
		return nil, apperror.Internal(err)
	}

	member := &domain.TeamMember{TeamID: teamID, UserID: userID, CreatedAt: s.clock.Now()}
	if err := s.teams.AddMember(ctx, member); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, apperror.MemberAlreadyExists()
		}
		return nil, apperror.Internal(err)
	}
	return member, nil
}

// RemoveMember removes the membership only; tasks assigned to the user are
// intentionally left unchanged.
func (s *TeamService) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	if err := s.ensureTeam(ctx, teamID); err != nil {
		return err
	}
	removed, err := s.teams.RemoveMember(ctx, teamID, userID)
	if err != nil {
		return apperror.Internal(err)
	}
	if !removed {
		return apperror.MemberNotFound()
	}
	return nil
}

func (s *TeamService) ensureTeam(ctx context.Context, teamID uuid.UUID) error {
	if _, err := s.teams.FindByID(ctx, teamID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return apperror.TeamNotFound()
		}
		return apperror.Internal(err)
	}
	return nil
}
