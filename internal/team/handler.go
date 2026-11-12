package team

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"ticketing-api/internal/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	teams := rg.Group("/teams")
	teams.Use(authMiddleware)
	{
		teams.POST("", h.CreateTeam)
		teams.GET("", h.ListTeams)
		teams.GET("/:id", h.GetTeamByID)

		teams.POST("/:id/members", h.AddMember)
		teams.DELETE("/:id/members/:user_id", h.RemoveMember)
		teams.GET("/:id/members", h.ListMembers)
	}
}

func (h *Handler) CreateTeam(c *gin.Context) {
	var req CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	team, err := h.service.CreateTeam(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrDuplicateSlug) {
			response.Error(c, http.StatusConflict, response.CodeConflict, "Team slug is already in use")
			return
		}
		response.InternalError(c, err, "Failed to create team")
		return
	}

	response.Created(c, team)
}

func (h *Handler) ListTeams(c *gin.Context) {
	teams, err := h.service.ListTeams(c.Request.Context())
	if err != nil {
		response.InternalError(c, err, "Failed to list teams")
		return
	}
	response.OK(c, teams)
}

func (h *Handler) GetTeamByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid team ID")
		return
	}

	team, err := h.service.GetTeamByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrTeamNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Team not found")
			return
		}
		response.InternalError(c, err, "Failed to get team")
		return
	}
	response.OK(c, team)
}

func (h *Handler) AddMember(c *gin.Context) {
	teamID, err := strconv.Atoi(c.Param("id"))
	if err != nil || teamID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid team ID")
		return
	}

	var req AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	member, err := h.service.AddMember(c.Request.Context(), teamID, req)
	if err != nil {
		if errors.Is(err, ErrMemberDuplicate) {
			response.Error(c, http.StatusConflict, response.CodeConflict, "User is already a member of this team")
			return
		}
		response.InternalError(c, err, "Failed to add member to team")
		return
	}

	response.Created(c, member)
}

func (h *Handler) RemoveMember(c *gin.Context) {
	teamID, err := strconv.Atoi(c.Param("id"))
	if err != nil || teamID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid team ID")
		return
	}

	userID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid user ID")
		return
	}

	if err := h.service.RemoveMember(c.Request.Context(), teamID, userID); err != nil {
		if errors.Is(err, ErrMemberNotFound) {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "Member not found in team")
			return
		}
		response.InternalError(c, err, "Failed to remove member")
		return
	}

	response.OK(c, gin.H{"removed": true, "team_id": teamID, "user_id": userID})
}

func (h *Handler) ListMembers(c *gin.Context) {
	teamID, err := strconv.Atoi(c.Param("id"))
	if err != nil || teamID <= 0 {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidArgument, "Invalid team ID")
		return
	}

	members, err := h.service.ListMembers(c.Request.Context(), teamID)
	if err != nil {
		response.InternalError(c, err, "Failed to list members")
		return
	}

	response.OK(c, members)
}

