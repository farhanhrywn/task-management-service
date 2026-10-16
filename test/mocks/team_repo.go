package mocks

import (
	"context"
	"sort"
	"sync"

	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/repository"
)

// InMemoryTeamRepo stores teams and memberships. Membership keys mirror the
// (team_id, user_id) primary key.
type InMemoryTeamRepo struct {
	mu      sync.Mutex
	teams   map[uuid.UUID]domain.Team
	members map[[2]uuid.UUID]domain.TeamMember
}

var _ repository.TeamRepository = (*InMemoryTeamRepo)(nil)

func NewInMemoryTeamRepo() *InMemoryTeamRepo {
	return &InMemoryTeamRepo{
		teams:   make(map[uuid.UUID]domain.Team),
		members: make(map[[2]uuid.UUID]domain.TeamMember),
	}
}

// PutTeam inserts a team directly.
func (r *InMemoryTeamRepo) PutTeam(t domain.Team) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.teams[t.ID] = t
}

func (r *InMemoryTeamRepo) List(context.Context) ([]domain.Team, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	teams := make([]domain.Team, 0, len(r.teams))
	for _, t := range r.teams {
		teams = append(teams, t)
	}
	sort.Slice(teams, func(i, j int) bool { return teams[i].Name < teams[j].Name })
	return teams, nil
}

func (r *InMemoryTeamRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.Team, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.teams[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &t, nil
}

func (r *InMemoryTeamRepo) ListMembers(_ context.Context, teamID uuid.UUID) ([]domain.TeamMemberView, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	views := make([]domain.TeamMemberView, 0)
	for k, m := range r.members {
		if k[0] == teamID {
			views = append(views, domain.TeamMemberView{UserID: m.UserID, JoinedAt: m.CreatedAt})
		}
	}
	return views, nil
}

func (r *InMemoryTeamRepo) AddMember(_ context.Context, m *domain.TeamMember) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := [2]uuid.UUID{m.TeamID, m.UserID}
	if _, ok := r.members[k]; ok {
		return repository.ErrDuplicate
	}
	r.members[k] = *m
	return nil
}

func (r *InMemoryTeamRepo) RemoveMember(_ context.Context, teamID, userID uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := [2]uuid.UUID{teamID, userID}
	if _, ok := r.members[k]; !ok {
		return false, nil
	}
	delete(r.members, k)
	return true, nil
}

func (r *InMemoryTeamRepo) ShareTeam(_ context.Context, userA, userB uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k := range r.members {
		if k[1] != userA {
			continue
		}
		if _, ok := r.members[[2]uuid.UUID{k[0], userB}]; ok {
			return true, nil
		}
	}
	return false, nil
}
