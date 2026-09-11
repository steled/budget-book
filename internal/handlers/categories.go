package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/steled/budget-book/internal/database"
)

type categoryDTO struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Type  string `json:"type"`
}

func toCategoryDTO(c database.Category) categoryDTO {
	return categoryDTO{ID: c.ID, Name: c.Name, Color: c.Color, Type: string(c.Type)}
}

type categoryRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
	Type  string `json:"type"`
}

func (req categoryRequest) validate() (database.CategoryType, error) {
	if req.Name == "" {
		return "", jsonError("Name ist erforderlich.")
	}
	if req.Color == "" {
		return "", jsonError("Farbe ist erforderlich.")
	}
	typ := database.CategoryType(req.Type)
	if typ != database.CategoryIncome && typ != database.CategoryExpense {
		return "", jsonError("Typ muss income oder expense sein.")
	}
	return typ, nil
}

// ListCategories returns every category.
func (h *Handlers) ListCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.store.ListCategories()
	if err != nil {
		h.logger.Error("list categories failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Kategorien konnten nicht geladen werden.")
		return
	}

	out := make([]categoryDTO, 0, len(categories))
	for _, c := range categories {
		out = append(out, toCategoryDTO(c))
	}
	writeJSON(w, http.StatusOK, out)
}

// CreateCategory creates a new user-defined category on top of the seeded
// defaults.
func (h *Handlers) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req categoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage.")
		return
	}
	typ, err := req.validate()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	category, err := h.store.CreateCategory(req.Name, req.Color, typ)
	if err != nil {
		h.logger.Error("create category failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Kategorie konnte nicht angelegt werden.")
		return
	}
	writeJSON(w, http.StatusCreated, toCategoryDTO(category))
}

// UpdateCategory updates an existing category's name, color and type.
func (h *Handlers) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige ID.")
		return
	}

	var req categoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage.")
		return
	}
	typ, err := req.validate()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	category, err := h.store.UpdateCategory(id, req.Name, req.Color, typ)
	if err != nil {
		h.logger.Error("update category failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Kategorie konnte nicht aktualisiert werden.")
		return
	}
	writeJSON(w, http.StatusOK, toCategoryDTO(category))
}

// DeleteCategory deletes a category, refusing if it still has transactions
// or recurring templates pointing at it.
func (h *Handlers) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige ID.")
		return
	}

	if err := h.store.DeleteCategory(id); err != nil {
		if errors.Is(err, database.ErrInUse) {
			writeError(w, http.StatusConflict, "Kategorie wird noch von Buchungen verwendet.")
			return
		}
		h.logger.Error("delete category failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Kategorie konnte nicht gelöscht werden.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
