package handlers

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/hiiamtrong/go-imap-bot/internal/api/dto"
	"github.com/hiiamtrong/go-imap-bot/internal/billsplit"
	"github.com/hiiamtrong/go-imap-bot/internal/llm"
	"github.com/hiiamtrong/go-imap-bot/internal/models"
	"github.com/hiiamtrong/go-imap-bot/internal/repository"
	"github.com/labstack/echo/v4"
)

const (
	maxParseImages     = 3
	maxParseImageBytes = 10 << 20
)

type ParseHandler struct {
	llm             *llm.Client
	userRepo        *repository.UserRepository
	aliasRepo       *repository.AliasRepository
	transactionRepo *repository.TransactionRepository
	selfUserID      int64
}

func NewParseHandler(
	llmClient *llm.Client,
	userRepo *repository.UserRepository,
	aliasRepo *repository.AliasRepository,
	transactionRepo *repository.TransactionRepository,
	selfUserID int64,
) *ParseHandler {
	return &ParseHandler{llm: llmClient, userRepo: userRepo, aliasRepo: aliasRepo, transactionRepo: transactionRepo, selfUserID: selfUserID}
}

// ParseBill godoc
// @Summary Propose a bill split from text, images or a Google Sheet link
// @Tags splits
// @Accept json
// @Produce json
// @Param request body dto.ParseBillRequest true "Parse Request"
// @Success 200 {object} dto.Response{data=dto.ParseBillResponse}
// @Router /api/splits/parse [post]
func (h *ParseHandler) ParseBill(c echo.Context) error {
	fail := func(status int, msg string) error {
		return c.JSON(status, dto.Response{Error: msg})
	}

	if h.llm == nil {
		return fail(http.StatusServiceUnavailable, "AI splitting is not configured")
	}

	var req dto.ParseBillRequest
	if err := c.Bind(&req); err != nil {
		return fail(http.StatusBadRequest, "Invalid request body")
	}
	if strings.TrimSpace(req.Text) == "" && len(req.Images) == 0 {
		return fail(http.StatusBadRequest, "Provide a message, a sheet link or an image")
	}

	images, err := decodeImages(req.Images)
	if err != nil {
		return fail(http.StatusBadRequest, err.Error())
	}

	var target *models.Transaction
	if req.TransactionID != 0 {
		if target, err = h.transactionRepo.GetByID(req.TransactionID); err != nil {
			return fail(http.StatusNotFound, "Transaction not found")
		}
	}

	in := llm.Input{Text: req.Text, Images: images}
	if sheetURL, ok := billsplit.SheetExportURL(req.Text); ok {
		if in.Sheet, err = billsplit.FetchSheetCSV(c.Request().Context(), sheetURL); err != nil {
			return fail(http.StatusBadRequest, err.Error())
		}
	}

	users, err := h.userRepo.GetAll()
	if err != nil {
		return fail(http.StatusInternalServerError, "Failed to load users")
	}

	aliases, err := h.aliasRepo.GetAll()
	if err != nil {
		return fail(http.StatusInternalServerError, "Failed to load aliases")
	}

	in.Users, in.Aliases = billsplit.KnownUsers(users), aliases
	bill, err := h.llm.ParseBill(c.Request().Context(), in)
	if err != nil {
		log.Printf("parse bill: %v", err)
		return fail(http.StatusBadGateway, "Could not analyze the content, please try again")
	}

	plan, description, problems := billsplit.ForBill(bill, users, h.selfUserID, target, aliases)
	if len(problems) > 0 {
		log.Printf("parse bill rejected: %s | model matched: %s", strings.Join(problems, "; "), matchedIDs(bill.People))
		return fail(http.StatusUnprocessableEntity, strings.Join(problems, "; "))
	}

	shares := make([]dto.ParsedShare, len(plan.Shares))
	for i, s := range plan.Shares {
		shares[i] = dto.ParsedShare{UserID: s.UserID, Name: s.Name, Amount: s.Amount, Alias: s.Alias}
	}
	return c.JSON(http.StatusOK, dto.Response{Data: dto.ParseBillResponse{Description: description, Total: plan.Total, Shares: shares, Prorated: plan.Prorated}})
}

// matchedIDs shows which known user the model picked for each person, so a
// rejected request can be told apart from a model that matched nothing (0).
func matchedIDs(people []llm.Person) string {
	parts := make([]string, len(people))
	for i, p := range people {
		parts[i] = fmt.Sprintf("%q=%d", p.Name, p.UserID)
	}
	return strings.Join(parts, " ")
}

func decodeImages(encoded []string) ([][]byte, error) {
	if len(encoded) > maxParseImages {
		return nil, fmt.Errorf("At most %d images are allowed", maxParseImages)
	}

	images := make([][]byte, 0, len(encoded))
	for _, s := range encoded {
		if _, payload, found := strings.Cut(s, ","); found && strings.HasPrefix(s, "data:") {
			s = payload
		}
		img, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("Image is not valid base64")
		}
		if len(img) > maxParseImageBytes {
			return nil, fmt.Errorf("Image is larger than %d MB", maxParseImageBytes>>20)
		}
		images = append(images, img)
	}
	return images, nil
}
