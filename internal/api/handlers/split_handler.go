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
}

func NewSplitHandler(
	splitRepo *repository.TransactionSplitRepository,
	transactionRepo *repository.TransactionRepository,
	userRepo *repository.UserRepository,
	smtpService *smtp.SMTPService,
) *SplitHandler {
	return &SplitHandler{
		splitRepo:       splitRepo,
		transactionRepo: transactionRepo,
		userRepo:        userRepo,
		smtpService:     smtpService,
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
		split := &models.TransactionSplit{
			TransactionID:   req.TransactionID,
			UserID:          userReq.UserID,
			Amount:          userReq.Amount,
			Reason:          userReq.Reason,
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
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid transaction ID",
		})
	}

	splits, err := h.splitRepo.GetByTransactionID(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to get splits: " + err.Error(),
		})
	}

	var response []dto.SplitResponse
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
// @Summary Mark a split as paid
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

	// Update split status
	err = h.splitRepo.UpdateSplitStatus([]int64{id}, nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to complete split: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, dto.Response{
		Message: "Split completed successfully",
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

// // SendReminders godoc
// // @Summary Send payment reminders
// @Tags reminders
// @Accept json
// @Produce json
