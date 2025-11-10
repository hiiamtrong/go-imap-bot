package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hiiamtrong/go-imap-bot/internal/api/dto"
	"github.com/hiiamtrong/go-imap-bot/internal/models"
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

	// Fetch all splits by IDs
	splits, err := h.splitRepo.GetByIDs(req.SplitIDs)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: fmt.Sprintf("Failed to fetch splits: %v", err),
		})
	}

	if len(splits) == 0 {
		return c.JSON(http.StatusNotFound, dto.Response{
			Error: "No splits found with the provided IDs",
		})
	}

	// Group splits by user ID
	splitsByUser := make(map[int64][]*models.TransactionSplit)
	for _, split := range splits {
		splitsByUser[split.UserID] = append(splitsByUser[split.UserID], split)
	}

	// Determine mode based on angry flag
	mode := "normal"
	if req.Angry {
		mode = "angry"
	}

	// Send reminders to each user
	successCount := 0
	failedUsers := []string{}

	for userID, userSplits := range splitsByUser {
		// Fetch user details
		user, err := h.userRepo.GetByID(userID)
		if err != nil {
			failedUsers = append(failedUsers, fmt.Sprintf("User ID %d (fetch error)", userID))
			continue
		}

		// Extract split IDs for this user
		splitIDs := make([]int64, len(userSplits))
		for i, split := range userSplits {
			splitIDs[i] = split.ID
		}

		// Generate hash for these splits
		hash, err := h.splitHashRepo.GenerateHash(splitIDs)
		if err != nil {
			failedUsers = append(failedUsers, user.Email)
			continue
		}

		// Send email reminder
		// Use config email as the "from user" - you may want to customize this
		fromUser := h.smtpService.GetFromEmail()
		err = h.smtpService.SendBulkSplitReminders(user, userSplits, fromUser, hash, mode)
		if err != nil {
			failedUsers = append(failedUsers, user.Email)
			continue
		}

		successCount++
	}

	// Prepare response message
	message := fmt.Sprintf("Successfully sent %d reminder(s)", successCount)
	if len(failedUsers) > 0 {
		message += fmt.Sprintf(". Failed to send to: %s", strings.Join(failedUsers, ", "))
	}

	return c.JSON(http.StatusOK, dto.Response{
		Message: message,
		Data: map[string]interface{}{
			"success_count": successCount,
			"failed_count":  len(failedUsers),
			"failed_users":  failedUsers,
		},
	})
}
