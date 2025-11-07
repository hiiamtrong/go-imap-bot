package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/hiiamtrong/go-imap-bot/internal/api/dto"
	"github.com/hiiamtrong/go-imap-bot/internal/models"
	"github.com/hiiamtrong/go-imap-bot/internal/repository"
	"github.com/labstack/echo/v4"
)

type UserHandler struct {
	userRepo *repository.UserRepository
}

func NewUserHandler(userRepo *repository.UserRepository) *UserHandler {
	return &UserHandler{
		userRepo: userRepo,
	}
}

// GetUsers godoc
// @Summary Get all users
// @Tags users
// @Param search query string false "Search in name or email"
// @Param whitelist query bool false "Filter by whitelist status"
// @Success 200 {object} dto.Response{data=[]dto.UserResponse}
// @Router /api/users [get]
func (h *UserHandler) GetUsers(c echo.Context) error {
	// Parse filters
	filters := repository.UserFilters{
		Search: c.QueryParam("search"),
	}

	// Parse whitelist filter
	if whitelistStr := c.QueryParam("whitelist"); whitelistStr != "" {
		if whitelistStr == "true" {
			whitelist := true
			filters.Whitelist = &whitelist
		} else if whitelistStr == "false" {
			whitelist := false
			filters.Whitelist = &whitelist
		}
	}

	users, err := h.userRepo.GetAllWithFilters(filters)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to get users: " + err.Error(),
		})
	}

	var response []dto.UserResponse
	for _, u := range users {
		response = append(response, dto.UserResponse{
			ID:        u.ID,
			Name:      u.Name,
			Email:     u.Email,
			Whitelist: u.IsWhitelisted,
			CreatedAt: u.CreatedAt,
		})
	}

	return c.JSON(http.StatusOK, dto.Response{Data: response})
}

// GetUser godoc
// @Summary Get user by ID
// @Tags users
// @Param id path int true "User ID"
// @Success 200 {object} dto.Response{data=dto.UserResponse}
// @Router /api/users/{id} [get]
func (h *UserHandler) GetUser(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid user ID",
		})
	}

	user, err := h.userRepo.GetByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, dto.Response{
				Error: "User not found",
			})
		}
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to get user: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, dto.Response{
		Data: dto.UserResponse{
			ID:        user.ID,
			Name:      user.Name,
			Email:     user.Email,
			Whitelist: user.IsWhitelisted,
			CreatedAt: user.CreatedAt,
		},
	})
}

// CreateUser godoc
// @Summary Create a new user
// @Tags users
// @Accept json
// @Produce json
// @Param request body dto.CreateUserRequest true "User Request"
// @Success 201 {object} dto.Response{data=dto.UserResponse}
// @Router /api/users [post]
func (h *UserHandler) CreateUser(c echo.Context) error {
	var req dto.CreateUserRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid request body",
		})
	}

	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Name is required",
		})
	}

	if req.Email == "" {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Email is required",
		})
	}

	user := &models.User{
		Name:          req.Name,
		Email:         req.Email,
		IsWhitelisted: req.Whitelist,
		CreatedAt:     time.Now(),
	}

	if err := h.userRepo.Create(user); err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to create user: " + err.Error(),
		})
	}

	// If whitelist was set, update it after creation
	if req.Whitelist {
		user.IsWhitelisted = true
		h.userRepo.Update(user)
	}

	return c.JSON(http.StatusCreated, dto.Response{
		Data: dto.UserResponse{
			ID:        user.ID,
			Name:      user.Name,
			Email:     user.Email,
			Whitelist: user.IsWhitelisted,
			CreatedAt: user.CreatedAt,
		},
	})
}

// UpdateUser godoc
// @Summary Update a user
// @Tags users
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Param request body dto.UpdateUserRequest true "User Update Request"
// @Success 200 {object} dto.Response{data=dto.UserResponse}
// @Router /api/users/{id} [put]
func (h *UserHandler) UpdateUser(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid user ID",
		})
	}

	var req dto.UpdateUserRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid request body",
		})
	}

	// Get existing user
	user, err := h.userRepo.GetByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, dto.Response{
				Error: "User not found",
			})
		}
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to get user: " + err.Error(),
		})
	}

	// Update fields if provided
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Whitelist != nil {
		user.IsWhitelisted = *req.Whitelist
	}

	// Update user in database
	if err := h.userRepo.Update(user); err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to update user: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, dto.Response{
		Data: dto.UserResponse{
			ID:        user.ID,
			Name:      user.Name,
			Email:     user.Email,
			Whitelist: user.IsWhitelisted,
			CreatedAt: user.CreatedAt,
		},
	})
}

// DeleteUser godoc
// @Summary Delete a user
// @Tags users
// @Param id path int true "User ID"
// @Success 200 {object} dto.Response
// @Router /api/users/{id} [delete]
func (h *UserHandler) DeleteUser(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid user ID",
		})
	}

	if err := h.userRepo.Delete(id); err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to delete user: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, dto.Response{
		Message: "User deleted successfully",
	})
}
