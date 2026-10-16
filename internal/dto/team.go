package dto

import (
	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/pkg/response"
)

type AddMemberRequest struct {
	UserID string `json:"user_id" binding:"required,uuid"`
}

type TeamResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type TeamMemberResponse struct {
	UserID   string `json:"user_id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	JoinedAt string `json:"joined_at"`
}

type MembershipResponse struct {
	TeamID   string `json:"team_id"`
	UserID   string `json:"user_id"`
	JoinedAt string `json:"joined_at"`
}

func NewTeamResponse(t *domain.Team) TeamResponse {
	return TeamResponse{
		ID:        t.ID.String(),
		Name:      t.Name,
		CreatedAt: response.FormatTime(t.CreatedAt),
		UpdatedAt: response.FormatTime(t.UpdatedAt),
	}
}

func NewTeamMemberResponse(m *domain.TeamMemberView) TeamMemberResponse {
	return TeamMemberResponse{
		UserID:   m.UserID.String(),
		Name:     m.Name,
		Email:    m.Email,
		JoinedAt: response.FormatTime(m.JoinedAt),
	}
}

func NewMembershipResponse(m *domain.TeamMember) MembershipResponse {
	return MembershipResponse{
		TeamID:   m.TeamID.String(),
		UserID:   m.UserID.String(),
		JoinedAt: response.FormatTime(m.CreatedAt),
	}
}
