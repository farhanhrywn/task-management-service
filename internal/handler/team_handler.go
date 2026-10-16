package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/internal/domain"
	"github.com/farhanhrywn/task-management-service/internal/dto"
	"github.com/farhanhrywn/task-management-service/pkg/response"
)

type TeamService interface {
	List(ctx context.Context) ([]domain.Team, error)
	ListMembers(ctx context.Context, teamID uuid.UUID) ([]domain.TeamMemberView, error)
	AddMember(ctx context.Context, teamID, userID uuid.UUID) (*domain.TeamMember, error)
	RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error
}

type TeamHandler struct {
	teams TeamService
}

func NewTeamHandler(teams TeamService) *TeamHandler {
	return &TeamHandler{teams: teams}
}

// List handles GET /teams.
func (h *TeamHandler) List(c *gin.Context) {
	teams, err := h.teams.List(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	data := make([]dto.TeamResponse, 0, len(teams))
	for i := range teams {
		data = append(data, dto.NewTeamResponse(&teams[i]))
	}
	response.Data(c, http.StatusOK, data)
}

// ListMembers handles GET /teams/:id/members.
func (h *TeamHandler) ListMembers(c *gin.Context) {
	teamID, err := parseID(c, "id")
	if err != nil {
		_ = c.Error(err)
		return
	}
	members, err := h.teams.ListMembers(c.Request.Context(), teamID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	data := make([]dto.TeamMemberResponse, 0, len(members))
	for i := range members {
		data = append(data, dto.NewTeamMemberResponse(&members[i]))
	}
	response.Data(c, http.StatusOK, data)
}

// AddMember handles POST /teams/:id/members (super admin only).
func (h *TeamHandler) AddMember(c *gin.Context) {
	teamID, err := parseID(c, "id")
	if err != nil {
		_ = c.Error(err)
		return
	}
	var req dto.AddMemberRequest
	if err := bindJSON(c, &req); err != nil {
		_ = c.Error(err)
		return
	}

	member, err := h.teams.AddMember(c.Request.Context(), teamID, uuid.MustParse(req.UserID))
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Data(c, http.StatusCreated, dto.NewMembershipResponse(member))
}

// RemoveMember handles DELETE /teams/:id/members/:userId (super admin only).
func (h *TeamHandler) RemoveMember(c *gin.Context) {
	teamID, err := parseID(c, "id")
	if err != nil {
		_ = c.Error(err)
		return
	}
	userID, err := parseID(c, "userId")
	if err != nil {
		_ = c.Error(err)
		return
	}
	if err := h.teams.RemoveMember(c.Request.Context(), teamID, userID); err != nil {
		_ = c.Error(err)
		return
	}
	response.Message(c, http.StatusOK, "member removed")
}
