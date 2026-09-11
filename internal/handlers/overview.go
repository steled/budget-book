package handlers

import "net/http"

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

	writeJSON(w, http.StatusOK, overviewResponse{
		TotalBalanceCents: total,
		Accounts:          accounts,
		Transactions:      txDTOs,
	})
}
