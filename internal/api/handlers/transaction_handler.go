package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hiiamtrong/go-imap-bot/internal/api/dto"
	"github.com/hiiamtrong/go-imap-bot/internal/models"
	"github.com/hiiamtrong/go-imap-bot/internal/repository"
	"github.com/labstack/echo/v4"
)

type TransactionHandler struct {
	transactionRepo      *repository.TransactionRepository
	tagRepo              *repository.TagRepository
	transactionSplitRepo *repository.TransactionSplitRepository
	userRepo             *repository.UserRepository
}

func NewTransactionHandler(
	transactionRepo *repository.TransactionRepository,
	tagRepo *repository.TagRepository,
	transactionSplitRepo *repository.TransactionSplitRepository,
	userRepo *repository.UserRepository,
) *TransactionHandler {
	return &TransactionHandler{
		transactionRepo:      transactionRepo,
		tagRepo:              tagRepo,
		transactionSplitRepo: transactionSplitRepo,
		userRepo:             userRepo,
	}
}

// GetTransactions godoc
// @Summary Get list of transactions
// @Tags transactions
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Param type query string false "Transaction type (add/subtract)"
// @Param start_date query string false "Start date (RFC3339 format)"
// @Param end_date query string false "End date (RFC3339 format)"
// @Param min_amount query int false "Minimum amount"
// @Param max_amount query int false "Maximum amount"
// @Param tag_ids query string false "Comma-separated tag IDs"
// @Param search query string false "Search in description"
// @Success 200 {object} dto.Response{data=[]dto.TransactionResponse}
// @Router /api/transactions [get]
func (h *TransactionHandler) GetTransactions(c echo.Context) error {
	limitStr := c.QueryParam("limit")
	offsetStr := c.QueryParam("offset")

	limit := int64(20)
	offset := int64(0)

	if limitStr != "" {
		if l, err := strconv.ParseInt(limitStr, 10, 64); err == nil {
			limit = l
		}
	}

	if offsetStr != "" {
		if o, err := strconv.ParseInt(offsetStr, 10, 64); err == nil {
			offset = o
		}
	}

	// Parse filters
	filters := repository.TransactionFilters{}

	if typeParam := c.QueryParam("type"); typeParam != "" {
		filters.Type = &typeParam
	}
	if searchParam := c.QueryParam("search"); searchParam != "" {
		filters.Description = &searchParam
	}

	// Parse date range
	if startDateStr := c.QueryParam("start_date"); startDateStr != "" {
		if startDate, err := time.Parse(time.RFC3339, startDateStr); err == nil {
			filters.StartDate = &startDate
		}
	}
	if endDateStr := c.QueryParam("end_date"); endDateStr != "" {
		if endDate, err := time.Parse(time.RFC3339, endDateStr); err == nil {
			filters.EndDate = &endDate
		}
	}

	// Parse amount range
	if minAmountStr := c.QueryParam("min_amount"); minAmountStr != "" {
		if minAmount, err := strconv.ParseInt(minAmountStr, 10, 64); err == nil {
			filters.MinAmount = &minAmount
		}
	}
	if maxAmountStr := c.QueryParam("max_amount"); maxAmountStr != "" {
		if maxAmount, err := strconv.ParseInt(maxAmountStr, 10, 64); err == nil {
			filters.MaxAmount = &maxAmount
		}
	}

	// Parse tag IDs
	if tagIDsStr := c.QueryParam("tag_ids"); tagIDsStr != "" {
		tagIDStrs := strings.Split(tagIDsStr, ",")
		for _, tidStr := range tagIDStrs {
			if tid, err := strconv.ParseInt(strings.TrimSpace(tidStr), 10, 64); err == nil {
				filters.TagIDs = append(filters.TagIDs, tid)
			}
		}
	}

	fmt.Printf("%+v\n", filters)

	transactions, err := h.transactionRepo.GetRecentTransactionsWithFilters(context.Background(), limit, offset, filters)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to get transactions: " + err.Error(),
		})
	}

	response := make([]dto.TransactionResponse, 0, len(transactions))
	for _, t := range transactions {
		transactionDTO := h.toTransactionDTO(t)

		// Get tags
		tags, _ := h.transactionRepo.GetTagsForTransaction(t.ID)
		for _, tag := range tags {
			transactionDTO.Tags = append(transactionDTO.Tags, dto.TagResponse{
				ID:   tag.ID,
				Name: tag.Name,
			})
		}

		// Get splits
		splits, _ := h.transactionSplitRepo.GetByTransactionID(t.ID)
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
			if user, err := h.userRepo.GetByID(split.UserID); err == nil {
				splitDTO.User = &dto.UserResponse{
					ID:        user.ID,
					Name:      user.Name,
					Email:     user.Email,
					Whitelist: user.IsWhitelisted,
					CreatedAt: user.CreatedAt,
				}
			}

			transactionDTO.Splits = append(transactionDTO.Splits, splitDTO)
		}

		response = append(response, transactionDTO)
	}

	return c.JSON(http.StatusOK, dto.Response{Data: response})
}

// GetTransaction godoc
// @Summary Get transaction by ID
// @Tags transactions
// @Param id path int true "Transaction ID"
// @Success 200 {object} dto.Response{data=dto.TransactionResponse}
// @Router /api/transactions/{id} [get]
func (h *TransactionHandler) GetTransaction(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid transaction ID",
		})
	}

	transaction, err := h.transactionRepo.GetByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, dto.Response{
				Error: "Transaction not found",
			})
		}
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to get transaction: " + err.Error(),
		})
	}

	transactionDTO := h.toTransactionDTO(transaction)

	// Get tags
	tags, _ := h.transactionRepo.GetTagsForTransaction(transaction.ID)
	for _, tag := range tags {
		transactionDTO.Tags = append(transactionDTO.Tags, dto.TagResponse{
			ID:   tag.ID,
			Name: tag.Name,
		})
	}

	// Get splits
	splits, _ := h.transactionSplitRepo.GetByTransactionID(transaction.ID)
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
		if user, err := h.userRepo.GetByID(split.UserID); err == nil {
			splitDTO.User = &dto.UserResponse{
				ID:        user.ID,
				Name:      user.Name,
				Email:     user.Email,
				Whitelist: user.IsWhitelisted,
				CreatedAt: user.CreatedAt,
			}
		}

		transactionDTO.Splits = append(transactionDTO.Splits, splitDTO)
	}

	return c.JSON(http.StatusOK, dto.Response{Data: transactionDTO})
}

// CreateVirtualBill godoc
// @Summary Create a virtual bill
// @Tags transactions
// @Accept json
// @Produce json
// @Param request body dto.CreateVirtualBillRequest true "Virtual Bill Request"
// @Success 201 {object} dto.Response{data=dto.TransactionResponse}
// @Router /api/transactions/virtual [post]
func (h *TransactionHandler) CreateVirtualBill(c echo.Context) error {
	var req dto.CreateVirtualBillRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid request body",
		})
	}

	if req.Amount == 0 {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Amount cannot be zero",
		})
	}

	if req.Description == "" {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Description is required",
		})
	}

	// Determine transaction type based on amount sign
	// Positive amount = expense (subtract from balance)
	// Negative amount = income (add to balance)
	transactionType := string(models.TransactionTypeSubtract)
	if req.Amount < 0 {
		transactionType = string(models.TransactionTypeAdd)
	}

	transaction := &models.Transaction{
		MailID:         0, // Virtual bill has no associated mail
		Amount:         req.Amount,
		CurrentBalance: 0,
		Type:           transactionType,
		Description:    req.Description,
		Timestamp:      time.Now(),
		CreatedAt:      time.Now(),
		From:           "virtual",
		To:             "virtual",
	}

	if err := h.transactionRepo.Create(transaction); err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to create virtual bill: " + err.Error(),
		})
	}

	transactionDTO := h.toTransactionDTO(transaction)

	return c.JSON(http.StatusCreated, dto.Response{Data: transactionDTO})
}

// DeleteTransaction godoc
// @Summary Delete a transaction
// @Tags transactions
// @Param id path int true "Transaction ID"
// @Success 200 {object} dto.Response
// @Router /api/transactions/{id} [delete]
func (h *TransactionHandler) DeleteTransaction(c echo.Context) error {
	idStr := c.Param("id")
	_, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid transaction ID",
		})
	}

	// Note: You may want to add a Delete method to the repository
	// For now, return not implemented
	return c.JSON(http.StatusNotImplemented, dto.Response{
		Message: "Delete transaction not implemented",
	})
}

// AddTagToTransaction godoc
// @Summary Add a tag to a transaction
// @Tags transactions
// @Param id path int true "Transaction ID"
// @Param tagId path int true "Tag ID"
// @Success 200 {object} dto.Response
// @Router /api/transactions/{id}/tags/{tagId} [post]
func (h *TransactionHandler) AddTagToTransaction(c echo.Context) error {
	transactionIDStr := c.Param("id")
	tagIDStr := c.Param("tagId")

	transactionID, err := strconv.ParseInt(transactionIDStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid transaction ID",
		})
	}

	tagID, err := strconv.ParseInt(tagIDStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid tag ID",
		})
	}

	if err := h.transactionRepo.AddTagToTransaction(transactionID, tagID); err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to add tag to transaction: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, dto.Response{
		Message: "Tag added successfully",
	})
}

// RemoveTagFromTransaction godoc
// @Summary Remove a tag from a transaction
// @Tags transactions
// @Param id path int true "Transaction ID"
// @Param tagId path int true "Tag ID"
// @Success 200 {object} dto.Response
// @Router /api/transactions/{id}/tags/{tagId} [delete]
func (h *TransactionHandler) RemoveTagFromTransaction(c echo.Context) error {
	transactionIDStr := c.Param("id")
	tagIDStr := c.Param("tagId")

	transactionID, err := strconv.ParseInt(transactionIDStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid transaction ID",
		})
	}

	tagID, err := strconv.ParseInt(tagIDStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid tag ID",
		})
	}

	if err := h.transactionRepo.RemoveTagFromTransaction(transactionID, tagID); err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to remove tag from transaction: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, dto.Response{
		Message: "Tag removed successfully",
	})
}

// CompleteTransaction godoc
// @Summary Mark a transaction as completed
// @Tags transactions
// @Param id path int true "Transaction ID"
// @Success 200 {object} dto.Response
// @Router /api/transactions/{id}/complete [post]
func (h *TransactionHandler) CompleteTransaction(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid transaction ID",
		})
	}

	// Check if transaction exists
	transaction, err := h.transactionRepo.GetByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, dto.Response{
				Error: "Transaction not found",
			})
		}
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to get transaction: " + err.Error(),
		})
	}

	// Check if already completed
	if transaction.Completed {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Transaction is already completed",
		})
	}

	// Mark transaction as completed
	if err := h.transactionRepo.Complete(context.Background(), id); err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to complete transaction: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, dto.Response{
		Message: "Transaction marked as completed",
	})
}

func (h *TransactionHandler) UpdateDescription(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid transaction ID",
		})
	}

	var req dto.UpdateDescriptionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Invalid request body",
		})
	}

	if req.Description == "" {
		return c.JSON(http.StatusBadRequest, dto.Response{
			Error: "Description cannot be empty",
		})
	}

	// Check if transaction exists
	transaction, err := h.transactionRepo.GetByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, dto.Response{
				Error: "Transaction not found",
			})
		}
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to get transaction: " + err.Error(),
		})
	}

	// Update the description
	if err := h.transactionRepo.UpdateDescription(context.Background(), id, req.Description); err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to update description: " + err.Error(),
		})
	}

	// Update local transaction object for response
	transaction.Description = req.Description
	transactionDTO := h.toTransactionDTO(transaction)

	// Get tags
	tags, _ := h.transactionRepo.GetTagsForTransaction(transaction.ID)
	for _, tag := range tags {
		transactionDTO.Tags = append(transactionDTO.Tags, dto.TagResponse{
			ID:   tag.ID,
			Name: tag.Name,
		})
	}

	// Get splits
	splits, _ := h.transactionSplitRepo.GetByTransactionID(transaction.ID)
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
		if user, err := h.userRepo.GetByID(split.UserID); err == nil {
			splitDTO.User = &dto.UserResponse{
				ID:        user.ID,
				Name:      user.Name,
				Email:     user.Email,
				Whitelist: user.IsWhitelisted,
				CreatedAt: user.CreatedAt,
			}
		}

		transactionDTO.Splits = append(transactionDTO.Splits, splitDTO)
	}

	return c.JSON(http.StatusOK, dto.Response{Data: transactionDTO})
}

// Helper function to convert model to DTO
func (h *TransactionHandler) toTransactionDTO(t *models.Transaction) dto.TransactionResponse {
	return dto.TransactionResponse{
		ID:          t.ID,
		MailID:      t.MailID,
		Amount:      t.Amount,
		Balance:     t.CurrentBalance,
		Currency:    t.Currency,
		Type:        t.Type,
		Description: t.Description,
		Completed:   t.Completed,
		Timestamp:   t.Timestamp,
		Tags:        []dto.TagResponse{},
		Splits:      []dto.SplitResponse{},
	}
}
