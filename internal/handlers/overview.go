package handlers

import (
	"net/http"
	"time"

	"github.com/steled/budget-book/internal/database"
)

// upcomingDTO is a recurring booking due later this month that has not been
// booked yet.
type upcomingDTO struct {
	TemplateID    int64  `json:"templateId"`
	Description   string `json:"description"`
	AmountCents   int64  `json:"amountCents"`
	Date          string `json:"date"`
	CategoryName  string `json:"categoryName"`
	CategoryColor string `json:"categoryColor"`
	CategoryType  string `json:"categoryType"`
	AccountName   string `json:"accountName"`
}

// overviewAccountDTO is an account tile on the Übersicht page.
type overviewAccountDTO struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Icon         string `json:"icon"`
	BalanceCents int64  `json:"balanceCents"`
}

type overviewResponse struct {
	TotalBalanceCents int64                `json:"totalBalanceCents"`
	Accounts          []overviewAccountDTO `json:"accounts"`
	Transactions      []transactionDTO     `json:"transactions"`
	Upcoming          []upcomingDTO        `json:"upcoming"`
}

// recentTransactionsLimit bounds the combined transaction list on the
// Übersicht page; older bookings remain in the database and are reachable
// via the month view.
const recentTransactionsLimit = 200

// GetOverview returns the data for the Übersicht (start) page: total
// balance, per-account balances, and the combined, chronologically ordered
// transaction list.
func (h *Handlers) GetOverview(w http.ResponseWriter, r *http.Request) {
	total, err := h.store.TotalBalanceCents()
	if err != nil {
		h.logger.Error("total balance failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Saldo konnte nicht geladen werden.")
		return
	}

	balances, err := h.store.ListAccountBalances()
	if err != nil {
		h.logger.Error("list account balances failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Konten konnten nicht geladen werden.")
		return
	}
	accounts := make([]overviewAccountDTO, 0, len(balances))
	for _, b := range balances {
		accounts = append(accounts, overviewAccountDTO{
			ID:           b.ID,
			Name:         b.Name,
			Icon:         b.Icon,
			BalanceCents: b.BalanceCents,
		})
	}

	transactions, err := h.store.ListTransactions(recentTransactionsLimit)
	if err != nil {
		h.logger.Error("list transactions failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Buchungen konnten nicht geladen werden.")
		return
	}
	txDTOs := make([]transactionDTO, 0, len(transactions))
	for _, t := range transactions {
		txDTOs = append(txDTOs, toTransactionDTO(t))
	}

	upcoming, err := h.upcomingThisMonth()
	if err != nil {
		h.logger.Error("list upcoming recurring failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Anstehende Buchungen konnten nicht geladen werden.")
		return
	}

	writeJSON(w, http.StatusOK, overviewResponse{
		Upcoming:          upcoming,
		TotalBalanceCents: total,
		Accounts:          accounts,
		Transactions:      txDTOs,
	})
}

func (h *Handlers) upcomingThisMonth() ([]upcomingDTO, error) {
	now := time.Now()
	endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	occurrences, err := h.store.ListUpcomingRecurring(endOfMonth)
	if err != nil {
		return nil, err
	}
	categories, err := h.store.ListCategories()
	if err != nil {
		return nil, err
	}
	accounts, err := h.store.ListAccounts()
	if err != nil {
		return nil, err
	}
	categoryByID := make(map[int64]database.Category, len(categories))
	for _, c := range categories {
		categoryByID[c.ID] = c
	}
	accountNames := make(map[int64]string, len(accounts))
	for _, a := range accounts {
		accountNames[a.ID] = a.Name
	}

	out := make([]upcomingDTO, 0, len(occurrences))
	for _, o := range occurrences {
		c := categoryByID[o.Template.CategoryID]
		out = append(out, upcomingDTO{
			TemplateID:    o.Template.ID,
			Description:   o.Template.Name,
			AmountCents:   o.AmountCents,
			Date:          o.Date.Format("2006-01-02"),
			CategoryName:  c.Name,
			CategoryColor: c.Color,
			CategoryType:  string(c.Type),
			AccountName:   accountNames[o.Template.AccountID],
		})
	}
	return out, nil
}
