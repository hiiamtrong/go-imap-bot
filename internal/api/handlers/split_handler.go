package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/hiiamtrong/go-imap-bot/internal/api/dto"
	"github.com/hiiamtrong/go-imap-bot/internal/models"
	"github.com/hiiamtrong/go-imap-bot/internal/repository"
	"github.com/hiiamtrong/go-imap-bot/internal/smtp"
	"github.com/labstack/echo/v4"
)

type SplitHandler struct {
	splitRepo       *repository.TransactionSplitRepository
	transactionRepo *repository.TransactionRepository
	userRepo        *repository.UserRepository
	smtpService     *smtp.SMTPService
	splitHashRepo   *repository.SplitHashRepository
}

func NewSplitHandler(
	splitRepo *repository.TransactionSplitRepository,
	transactionRepo *repository.TransactionRepository,
	userRepo *repository.UserRepository,
	smtpService *smtp.SMTPService,
	splitHashRepo *repository.SplitHashRepository,
) *SplitHandler {
	return &SplitHandler{
		splitRepo:       splitRepo,
		transactionRepo: transactionRepo,
		userRepo:        userRepo,
		smtpService:     smtpService,
		splitHashRepo:   splitHashRepo,
	}
}

// CreateSplit godoc
// @Summary Create bill splits
// @Tags splits
// @Accept json
// @Produce json
// @Param request body dto.CreateSplitRequest true "Split Request"
// @Success 201 {object} dto.Response{data=[]dto.SplitResponse}
// @Router /api/splits [post]
func (h *SplitHandler) CreateSplit(c echo.Context) error {
	var req dto.CreateSplitRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid request body",
		})
	}

	if req.TransactionID == 0 {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Transaction ID is required",
		})
	}

	if len(req.Users) == 0 {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "At least one user is required",
		})
	}

	// Verify transaction exists
	transaction, err := h.transactionRepo.GetByID(req.TransactionID)
	if err != nil {
		return c.JSON(http.StatusNotFound, dto.Response{
			Error: "Transaction not found",
		})
	}

	// Create splits
	var splits []dto.SplitResponse
	for _, userReq := range req.Users {
		// Use user reason if provided, otherwise use transaction description
		reason := userReq.Reason
		if reason == "" {
			reason = transaction.Description
		}

		split := &models.TransactionSplit{
			TransactionID:   req.TransactionID,
			UserID:          userReq.UserID,
			Amount:          int64(userReq.Amount), // Convert float64 to int64
			Reason:          reason,
			Completed:       false,
			CreatedAt:       time.Now(),
			TotalBillAmount: transaction.Amount,
			BillCreatedAt:   transaction.Timestamp,
		}

		if err := h.splitRepo.Create(split); err != nil {
			return c.JSON(http.StatusInternalServerError, dto.Response{
				Error: "Failed to create split: " + err.Error(),
			})
		}

		// Get user info
		user, _ := h.userRepo.GetByID(userReq.UserID)
		userDTO := &dto.UserResponse{}
		if user != nil {
			userDTO = &dto.UserResponse{
				ID:        user.ID,
				Name:      user.Name,
				Email:     user.Email,
				Whitelist: user.IsWhitelisted,
				CreatedAt: user.CreatedAt,
			}
		}

		splits = append(splits, dto.SplitResponse{
			ID:            split.ID,
			TransactionID: split.TransactionID,
			UserID:        split.UserID,
			Amount:        split.Amount,
			Reason:        split.Reason,
			Completed:     split.Completed,
			User:          userDTO,
		})
	}

	return c.JSON(http.StatusCreated, dto.Response{Data: splits})
}

// GetSplitsForTransaction godoc
// @Summary Get splits for a transaction
// @Tags splits
// @Param id path int true "Transaction ID"
// @Success 200 {object} dto.Response{data=[]dto.SplitResponse}
// @Router /api/transactions/{id}/splits [get]
func (h *SplitHandler) GetSplitsForTransaction(c echo.Context) error {
	idStr := c.Param("id")
	transactionID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid transaction ID",
		})
	}

	splits, err := h.splitRepo.GetByTransactionID(transactionID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to get splits: " + err.Error(),
		})
	}

	response := make([]dto.SplitResponse, 0, len(splits))
	for _, split := range splits {
		splitDTO := dto.SplitResponse{
			ID:            split.ID,
			TransactionID: split.TransactionID,
			UserID:        split.UserID,
			Amount:        split.Amount,
			Reason:        split.Reason,
			Completed:     split.Completed,
		}

		// Get user info
		user, _ := h.userRepo.GetByID(split.UserID)
		if user != nil {
			splitDTO.User = &dto.UserResponse{
				ID:        user.ID,
				Name:      user.Name,
				Email:     user.Email,
				Whitelist: user.IsWhitelisted,
				CreatedAt: user.CreatedAt,
			}
		}

		response = append(response, splitDTO)
	}

	return c.JSON(http.StatusOK, dto.Response{Data: response})
}

// CompleteSplit godoc
// @Summary Mark all pending splits for a user as paid
// @Tags splits
// @Param id path int true "Split ID"
// @Success 200 {object} dto.Response
// @Router /api/splits/{id}/complete [post]
func (h *SplitHandler) CompleteSplit(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid split ID",
		})
	}

	// Get the split to find the user ID
	split, err := h.splitRepo.GetByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, dto.Response{
			Error: "Split not found",
		})
	}

	// Complete all pending splits for this user (same logic as handleConfirmDoneUsers)
	err = h.splitRepo.CompleteAllSplitsForUser(split.UserID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to complete splits: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, dto.Response{
		Message: "All pending splits for user completed successfully",
	})
}

// DeleteSplit godoc
// @Summary Delete a split
// @Tags splits
// @Param id path int true "Split ID"
// @Success 200 {object} dto.Response
// @Router /api/splits/{id} [delete]
func (h *SplitHandler) DeleteSplit(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid split ID",
		})
	}

	if err := h.splitRepo.Delete(context.Background(), id); err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to delete split: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, dto.Response{
		Message: "Split deleted successfully",
	})
}

// UpdateSplit godoc
// @Summary Update a split's amount and/or reason
// @Tags splits
// @Accept json
// @Produce json
// @Param id path int true "Split ID"
// @Param request body dto.UpdateSplitRequest true "Update Split Request"
// @Success 200 {object} dto.Response{data=dto.SplitResponse}
// @Router /api/splits/{id} [put]
func (h *SplitHandler) UpdateSplit(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid split ID",
		})
	}

	var req dto.UpdateSplitRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid request body",
		})
	}

	// Get existing split
	split, err := h.splitRepo.GetByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, dto.Response{
			Error: "Split not found",
		})
	}

	// Update fields if provided
	amount := split.Amount
	reason := split.Reason

	if req.Amount != nil {
		amount = int64(*req.Amount)
	}
	if req.Reason != nil {
		reason = *req.Reason
	}

	// Update the split
	if err := h.splitRepo.UpdateByID(id, amount, reason); err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to update split: " + err.Error(),
		})
	}

	// Get updated split
	updatedSplit, err := h.splitRepo.GetByID(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to get updated split: " + err.Error(),
		})
	}

	// Get user info
	user, _ := h.userRepo.GetByID(updatedSplit.UserID)
	var userDTO *dto.UserResponse
	if user != nil {
		userDTO = &dto.UserResponse{
			ID:        user.ID,
			Name:      user.Name,
			Email:     user.Email,
			Whitelist: user.IsWhitelisted,
			CreatedAt: user.CreatedAt,
		}
	}

	return c.JSON(http.StatusOK, dto.Response{
		Data: dto.SplitResponse{
			ID:            updatedSplit.ID,
			TransactionID: updatedSplit.TransactionID,
			UserID:        updatedSplit.UserID,
			Amount:        updatedSplit.Amount,
			Reason:        updatedSplit.Reason,
			Completed:     updatedSplit.Completed,
			User:          userDTO,
			CreatedAt:     updatedSplit.CreatedAt,
		},
	})
}

// CompleteSingleSplit godoc
// @Summary Mark a single split as paid
// @Tags splits
// @Param id path int true "Split ID"
// @Success 200 {object} dto.Response
// @Router /api/splits/{id}/complete-single [post]
func (h *SplitHandler) CompleteSingleSplit(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid split ID",
		})
	}

	// Complete single split
	if err := h.splitRepo.CompleteSplit(id); err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to complete split: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, dto.Response{
		Message: "Split completed successfully",
	})
}

// GetPendingSplitsSummary godoc
// @Summary Get pending splits grouped by user
// @Tags splits
// @Success 200 {object} dto.Response{data=[]dto.UserSplitSummary}
// @Router /api/splits/pending [get]
func (h *SplitHandler) GetPendingSplitsSummary(c echo.Context) error {
	// Get all pending splits
	splits, err := h.splitRepo.GetPendingSplits()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to get pending splits: " + err.Error(),
		})
	}

	// Group by user
	userMap := make(map[int64]*dto.UserSplitSummary)

	for _, split := range splits {
		if _, exists := userMap[split.UserID]; !exists {
			// Get user info
			user, err := h.userRepo.GetByID(split.UserID)
			if err != nil {
				continue
			}

			userMap[split.UserID] = &dto.UserSplitSummary{
				User: dto.UserResponse{
					ID:        user.ID,
					Name:      user.Name,
					Email:     user.Email,
					Whitelist: user.IsWhitelisted,
					CreatedAt: user.CreatedAt,
				},
				Splits:      []dto.SplitResponse{},
				TotalAmount: 0,
				BillCount:   0,
			}
		}

		summary := userMap[split.UserID]
		summary.Splits = append(summary.Splits, dto.SplitResponse{
			ID:            split.ID,
			TransactionID: split.TransactionID,
			UserID:        split.UserID,
			Amount:        split.Amount,
			Reason:        split.Reason,
			Completed:     split.Completed,
			CreatedAt:     split.CreatedAt,
			// SplitHash removed
		})
		summary.TotalAmount += split.Amount
		summary.BillCount++
	}

	// Convert map to slice
	response := make([]dto.UserSplitSummary, 0, len(userMap))
	for _, summary := range userMap {
		response = append(response, *summary)
	}

	return c.JSON(http.StatusOK, dto.Response{Data: response})
}

// SendReminders godoc
// @Summary Send payment reminders
// @Tags reminders
// @Accept json
// @Produce json
