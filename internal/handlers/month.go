package handlers

import (
	"net/http"
	"strconv"
	"time"
)

type monthResponse struct {
	Year         int   `json:"year"`
	Month        int   `json:"month"`
	IncomeCents  int64 `json:"incomeCents"`
	ExpenseCents int64 `json:"expenseCents"`
	BalanceCents int64 `json:"balanceCents"`
}

// GetMonth returns the income/expense/balance summary for one calendar
// month, defaulting to the current month. Accepts ?year=YYYY&month=1-12.
func (h *Handlers) GetMonth(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	year, month := now.Year(), int(now.Month())

	if raw := r.URL.Query().Get("year"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Ungültiges Jahr.")
			return
		}
		year = parsed
	}
	if raw := r.URL.Query().Get("month"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 12 {
			writeError(w, http.StatusBadRequest, "Ungültiger Monat.")
			return
		}
		month = parsed
	}

	income, expense, err := h.store.MonthSummary(year, time.Month(month))
	if err != nil {
		h.logger.Error("month summary failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Monatsübersicht konnte nicht geladen werden.")
		return
	}

	writeJSON(w, http.StatusOK, monthResponse{
		Year:         year,
		Month:        month,
		IncomeCents:  income,
		ExpenseCents: expense,
		BalanceCents: income - expense,
	})
}
