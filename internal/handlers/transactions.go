package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/steled/budget-book/internal/database"
)

type transactionDTO struct {
	ID            int64  `json:"id"`
	AmountCents   int64  `json:"amountCents"`
	Date          string `json:"date"`
	Description   string `json:"description"`
	CategoryID    int64  `json:"categoryId"`
	CategoryName  string `json:"categoryName"`
	CategoryColor string `json:"categoryColor"`
	CategoryType  string `json:"categoryType"`
	AccountID     int64  `json:"accountId"`
	AccountName   string `json:"accountName"`
	Source        string `json:"source"`
}

func toTransactionDTO(t database.TransactionDetail) transactionDTO {
	return transactionDTO{
		ID:            t.ID,
		AmountCents:   t.AmountCents,
		Date:          t.Date.Format("2006-01-02"),
		Description:   t.Description,
		CategoryID:    t.CategoryID,
		CategoryName:  t.CategoryName,
		CategoryColor: t.CategoryColor,
		CategoryType:  string(t.CategoryType),
		AccountID:     t.AccountID,
		AccountName:   t.AccountName,
		Source:        string(t.Source),
	}
}

type transactionRequest struct {
	Amount      string `json:"amount"`
	Date        string `json:"date"`
	Description string `json:"description"`
	CategoryID  int64  `json:"categoryId"`
	AccountID   int64  `json:"accountId"`
}

func (req transactionRequest) parse() (amountCents int64, date time.Time, err error) {
	amountCents, err = parseAmountCents(req.Amount)
	if err != nil {
		return 0, time.Time{}, err
	}
	date, err = time.Parse("2006-01-02", req.Date)
	if err != nil {
		return 0, time.Time{}, err
	}
	if req.CategoryID == 0 || req.AccountID == 0 {
		return 0, time.Time{}, errMissingReference
	}
	return amountCents, date, nil
}

var errMissingReference = jsonError("Kategorie und Konto sind erforderlich.")

type jsonError string

func (e jsonError) Error() string { return string(e) }

// ListTransactions returns transactions, newest first. An optional ?limit=
// query parameter caps the result.
func (h *Handlers) ListTransactions(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "Ungültiges limit.")
			return
		}
		limit = parsed
	}

	transactions, err := h.store.ListTransactions(limit)
	if err != nil {
		h.logger.Error("list transactions failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Buchungen konnten nicht geladen werden.")
		return
	}

	out := make([]transactionDTO, 0, len(transactions))
	for _, t := range transactions {
		out = append(out, toTransactionDTO(t))
	}
	writeJSON(w, http.StatusOK, out)
}

// CreateTransaction books a new manual transaction.
func (h *Handlers) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	var req transactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage.")
		return
	}
	amountCents, date, err := req.parse()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := h.store.CreateTransaction(amountCents, date, req.Description, req.CategoryID, req.AccountID, database.SourceManual)
	if err != nil {
		h.logger.Error("create transaction failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Buchung konnte nicht angelegt werden.")
		return
	}

	writeTransactionDetail(h, w, created.ID, http.StatusCreated)
}

// UpdateTransaction edits an existing transaction. Amount, date, category
// and account may all be changed; the source flag is preserved.
func (h *Handlers) UpdateTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige ID.")
		return
	}

	var req transactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage.")
		return
	}
	amountCents, date, err := req.parse()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if _, err := h.store.UpdateTransaction(id, amountCents, date, req.Description, req.CategoryID, req.AccountID); err != nil {
		h.logger.Error("update transaction failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Buchung konnte nicht aktualisiert werden.")
		return
	}

	writeTransactionDetail(h, w, id, http.StatusOK)
}

// writeTransactionDetail reloads the transaction with its joined
// category/account details and writes it as the response body.
func writeTransactionDetail(h *Handlers, w http.ResponseWriter, id int64, status int) {
	detail, err := h.store.GetTransactionDetail(id)
	if err != nil {
		h.logger.Error("reload transaction failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Buchung konnte nicht geladen werden.")
		return
	}
	writeJSON(w, status, toTransactionDTO(detail))
}

// DeleteTransaction deletes a transaction.
func (h *Handlers) DeleteTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige ID.")
		return
	}

	if err := h.store.DeleteTransaction(id); err != nil {
		h.logger.Error("delete transaction failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Buchung konnte nicht gelöscht werden.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
