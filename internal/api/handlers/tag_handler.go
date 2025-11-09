package handlers

import (
	"net/http"

	"github.com/hiiamtrong/go-imap-bot/internal/api/dto"
	"github.com/hiiamtrong/go-imap-bot/internal/repository"
	"github.com/labstack/echo/v4"
)

type TagHandler struct {
	tagRepo *repository.TagRepository
}

func NewTagHandler(tagRepo *repository.TagRepository) *TagHandler {
	return &TagHandler{
		tagRepo: tagRepo,
	}
}

// GetTags godoc
// @Summary Get all tags
// @Tags tags
// @Success 200 {object} dto.Response{data=[]dto.TagResponse}
// @Router /api/tags [get]
func (h *TagHandler) GetTags(c echo.Context) error {
	tags, err := h.tagRepo.GetAllTags()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to get tags: " + err.Error(),
		})
	}

	response := make([]dto.TagResponse, 0, len(tags))
	for _, tag := range tags {
		response = append(response, dto.TagResponse{
			ID:   tag.ID,
			Name: tag.Name,
		})
	}

	return c.JSON(http.StatusOK, dto.Response{Data: response})
}

// CreateTag godoc
// @Summary Create a new tag
// @Tags tags
// @Accept json
// @Produce json
// @Param request body dto.CreateTagRequest true "Tag Request"
// @Success 201 {object} dto.Response{data=dto.TagResponse}
// @Router /api/tags [post]
func (h *TagHandler) CreateTag(c echo.Context) error {
	var req dto.CreateTagRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid request body",
		})
	}

	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Tag name is required",
		})
	}

	id, err := h.tagRepo.Create(req.Name)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to create tag: " + err.Error(),
		})
	}

	return c.JSON(http.StatusCreated, dto.Response{
		Data: dto.TagResponse{
			ID:   id,
			Name: req.Name,
		},
	})
}
