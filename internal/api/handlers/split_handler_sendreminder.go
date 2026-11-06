package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

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

	// Each reminder costs a QR request, an S3 upload and an SMTP session, so
	// one at a time a long list outlasts the proxy's 60s timeout.
	var (
		mu           sync.Mutex
		wg           sync.WaitGroup
		successCount int
		failedUsers  = []string{}
		limit        = make(chan struct{}, 5)
	)
	fail := func(who string) {
		mu.Lock()
		failedUsers = append(failedUsers, who)
		mu.Unlock()
	}

	for userID, userSplits := range splitsByUser {
		wg.Add(1)
		limit <- struct{}{}
		go func() {
			defer func() { <-limit; wg.Done() }()

			user, err := h.userRepo.GetByID(userID)
			if err != nil {
				fail(fmt.Sprintf("User ID %d (fetch error)", userID))
				return
			}

			splitIDs := make([]int64, len(userSplits))
			for i, split := range userSplits {
				splitIDs[i] = split.ID
			}

			hash, err := h.splitHashRepo.GenerateHash(splitIDs)
			if err != nil {
				fail(user.Email)
				return
			}

			// Use config email as the "from user" - you may want to customize this
			fromUser := h.smtpService.GetFromEmail()
			if err := h.smtpService.SendBulkSplitReminders(user, userSplits, fromUser, hash, mode); err != nil {
				fail(user.Email)
				return
			}

			mu.Lock()
			successCount++
			mu.Unlock()
		}()
	}
	wg.Wait()

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
