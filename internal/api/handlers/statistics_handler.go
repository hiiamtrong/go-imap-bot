package handlers

import (
	"database/sql"
	"net/http"

	"github.com/hiiamtrong/go-imap-bot/internal/api/dto"
	"github.com/hiiamtrong/go-imap-bot/internal/database"
	"github.com/labstack/echo/v4"
)

type StatisticsHandler struct {
	db *database.Database
}

func NewStatisticsHandler(db *database.Database) *StatisticsHandler {
	return &StatisticsHandler{
		db: db,
	}
}

// GetStatistics godoc
// @Summary Get spending statistics
// @Tags statistics
// @Success 200 {object} dto.Response{data=dto.StatisticsResponse}
// @Router /api/statistics [get]
func (h *StatisticsHandler) GetStatistics(c echo.Context) error {
	stats := dto.StatisticsResponse{}

	// Get total spent and received
	query := `
		SELECT
			COALESCE(SUM(CASE WHEN type = 'subtract' THEN amount ELSE 0 END), 0) as total_spent,
			COALESCE(SUM(CASE WHEN type = 'add' THEN amount ELSE 0 END), 0) as total_received,
			COUNT(*) as transaction_count
		FROM transactions
	`
	err := h.db.Conn.QueryRow(query).Scan(&stats.TotalSpent, &stats.TotalReceived, &stats.TransactionCount)
	if err != nil && err != sql.ErrNoRows {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to get statistics: " + err.Error(),
		})
	}

	// Get latest balance
	balanceQuery := `SELECT current_balance FROM transactions ORDER BY timestamp DESC LIMIT 1`
	err = h.db.Conn.QueryRow(balanceQuery).Scan(&stats.Balance)
	if err != nil && err != sql.ErrNoRows {
		stats.Balance = 0
	}

	// Get split statistics
	splitQuery := `
		SELECT
			COUNT(CASE WHEN completed = 1 THEN 1 END) as completed,
			COUNT(CASE WHEN completed = 0 THEN 1 END) as pending
		FROM transaction_splits
	`
	err = h.db.Conn.QueryRow(splitQuery).Scan(&stats.CompletedSplits, &stats.PendingSplits)
	if err != nil && err != sql.ErrNoRows {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to get split statistics: " + err.Error(),
		})
	}

	// Get monthly spending
	monthlyQuery := `
		SELECT
			strftime('%Y-%m', timestamp) as month,
			SUM(amount) as amount,
			COUNT(*) as count
		FROM transactions
		WHERE type = 'subtract'
		GROUP BY strftime('%Y-%m', timestamp)
		ORDER BY month DESC
		LIMIT 12
	`
	rows, err := h.db.Conn.Query(monthlyQuery)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to get monthly spending: " + err.Error(),
		})
	}
	defer rows.Close()

	for rows.Next() {
		var spending dto.MonthlySpending
		if err := rows.Scan(&spending.Month, &spending.Amount, &spending.Count); err == nil {
			stats.SpendingByMonth = append(stats.SpendingByMonth, spending)
		}
	}

	// Get spending by tag
	tagQuery := `
		SELECT
			t.name as tag,
			SUM(tr.amount) as amount,
			COUNT(*) as count
		FROM transaction_tags tt
		JOIN tags t ON tt.tag_id = t.id
		JOIN transactions tr ON tt.transaction_id = tr.id
		WHERE tr.type = 'subtract'
		GROUP BY t.name
		ORDER BY amount DESC
		LIMIT 10
	`
	tagRows, err := h.db.Conn.Query(tagQuery)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to get tag spending: " + err.Error(),
		})
	}
	defer tagRows.Close()

	for tagRows.Next() {
		var spending dto.TagSpending
		if err := tagRows.Scan(&spending.Tag, &spending.Amount, &spending.Count); err == nil {
			stats.SpendingByTag = append(stats.SpendingByTag, spending)
		}
	}

	return c.JSON(http.StatusOK, dto.Response{Data: stats})
}

// GetMonthlySpending godoc
// @Summary Get monthly spending breakdown
// @Tags statistics
// @Param year query int false "Year"
// @Success 200 {object} dto.Response{data=[]dto.MonthlySpending}
// @Router /api/statistics/monthly [get]
func (h *StatisticsHandler) GetMonthlySpending(c echo.Context) error {
	year := c.QueryParam("year")

	query := `
		SELECT
			strftime('%Y-%m', timestamp) as month,
			SUM(amount) as amount,
			COUNT(*) as count
		FROM transactions
		WHERE type = 'subtract'
	`

	if year != "" {
		query += ` AND strftime('%Y', timestamp) = ?`
	}

	query += ` GROUP BY strftime('%Y-%m', timestamp) ORDER BY month DESC`

	var rows *sql.Rows
	var err error

	if year != "" {
		rows, err = h.db.Conn.Query(query, year)
	} else {
		rows, err = h.db.Conn.Query(query)
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to get monthly spending: " + err.Error(),
		})
	}
	defer rows.Close()

	var spending []dto.MonthlySpending
	for rows.Next() {
		var s dto.MonthlySpending
		if err := rows.Scan(&s.Month, &s.Amount, &s.Count); err == nil {
			spending = append(spending, s)
		}
	}

	return c.JSON(http.StatusOK, dto.Response{Data: spending})
}

// GetTagSpending godoc
// @Summary Get spending by tag
// @Tags statistics
// @Success 200 {object} dto.Response{data=[]dto.TagSpending}
// @Router /api/statistics/tags [get]
func (h *StatisticsHandler) GetTagSpending(c echo.Context) error {
	query := `
		SELECT
			t.name as tag,
			SUM(tr.amount) as amount,
			COUNT(*) as count
		FROM transaction_tags tt
		JOIN tags t ON tt.tag_id = t.id
		JOIN transactions tr ON tt.transaction_id = tr.id
		WHERE tr.type = 'subtract'
		GROUP BY t.name
		ORDER BY amount DESC
	`

	rows, err := h.db.Conn.Query(query)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.Response{
			Error: "Failed to get tag spending: " + err.Error(),
		})
	}
	defer rows.Close()

	var spending []dto.TagSpending
	for rows.Next() {
		var s dto.TagSpending
		if err := rows.Scan(&s.Tag, &s.Amount, &s.Count); err == nil {
			spending = append(spending, s)
		}
	}

	return c.JSON(http.StatusOK, dto.Response{Data: spending})
}
