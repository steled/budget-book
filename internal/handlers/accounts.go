package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/steled/budget-book/internal/database"
)

type accountDTO struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

func toAccountDTO(a database.Account) accountDTO {
	return accountDTO{ID: a.ID, Name: a.Name, Icon: a.Icon}
}

type accountRequest struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// ListAccounts returns every account.
func (h *Handlers) ListAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.store.ListAccounts()
	if err != nil {
		h.logger.Error("list accounts failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Konten konnten nicht geladen werden.")
		return
	}

	out := make([]accountDTO, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, toAccountDTO(a))
	}
	writeJSON(w, http.StatusOK, out)
}

// CreateAccount creates a new account.
func (h *Handlers) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req accountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage.")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name ist erforderlich.")
		return
	}

	account, err := h.store.CreateAccount(req.Name, req.Icon)
	if err != nil {
		h.logger.Error("create account failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Konto konnte nicht angelegt werden.")
		return
	}
	writeJSON(w, http.StatusCreated, toAccountDTO(account))
}

// UpdateAccount updates an existing account's name and icon.
func (h *Handlers) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige ID.")
		return
	}

	var req accountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage.")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name ist erforderlich.")
		return
	}

	account, err := h.store.UpdateAccount(id, req.Name, req.Icon)
	if err != nil {
		h.logger.Error("update account failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Konto konnte nicht aktualisiert werden.")
		return
	}
	writeJSON(w, http.StatusOK, toAccountDTO(account))
}

// DeleteAccount deletes an account, refusing if it still has transactions
// or recurring templates pointing at it.
func (h *Handlers) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige ID.")
		return
	}

	if err := h.store.DeleteAccount(id); err != nil {
		if errors.Is(err, database.ErrInUse) {
			writeError(w, http.StatusConflict, "Konto wird noch von Buchungen verwendet.")
			return
		}
		h.logger.Error("delete account failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Konto konnte nicht gelöscht werden.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
