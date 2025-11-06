package handlers

import (
	"net/http"

	"github.com/hiiamtrong/go-imap-bot/internal/api/dto"
	"github.com/labstack/echo/v4"
)

// SendReminders godoc
// @Summary Send payment reminders
// @Tags reminders
// @Accept json
// @Produce json
// @Param request body dto.SendReminderRequest true "Reminder Request"
// @Success 200 {object} dto.Response
// @Router /api/reminders [post]
func (h *SplitHandler) SendReminders(c echo.Context) error {
	var req dto.SendReminderRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid request body",
		})
	}

	if len(req.SplitIDs) == 0 {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "At least one split ID is required",
		})
	}

	// TODO: Implement reminder sending
	// This requires SendReminder and SendAngryReminder methods in SMTP service
	// For now, return success to indicate the API is working
	return c.JSON(http.StatusOK, dto.Response{
		Message: "Reminder functionality not yet implemented. Please implement SendReminder/SendAngryReminder in SMTP service.",
	})
}
