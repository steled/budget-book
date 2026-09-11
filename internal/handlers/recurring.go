package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/steled/budget-book/internal/database"
)

type recurringTemplateDTO struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	AmountCents int64  `json:"amountCents"`
	CategoryID  int64  `json:"categoryId"`
	AccountID   int64  `json:"accountId"`
	Interval    string `json:"interval"`
	NextDueDate string `json:"nextDueDate"`
	Active      bool   `json:"active"`
}

func toRecurringTemplateDTO(rt database.RecurringTemplate) recurringTemplateDTO {
	return recurringTemplateDTO{
		ID:          rt.ID,
		Name:        rt.Name,
		AmountCents: rt.AmountCents,
		CategoryID:  rt.CategoryID,
		AccountID:   rt.AccountID,
		Interval:    string(rt.Interval),
		NextDueDate: rt.NextDueDate.Format("2006-01-02"),
		Active:      rt.Active,
	}
}

type recurringTemplateRequest struct {
	Name        string `json:"name"`
	Amount      string `json:"amount"`
	CategoryID  int64  `json:"categoryId"`
	AccountID   int64  `json:"accountId"`
	Interval    string `json:"interval"`
	NextDueDate string `json:"nextDueDate"`
	Active      bool   `json:"active"`
}

func (req recurringTemplateRequest) parse() (amountCents int64, interval database.RecurringInterval, nextDue time.Time, err error) {
	if req.Name == "" {
		return 0, "", time.Time{}, jsonError("Name ist erforderlich.")
	}
	amountCents, err = parseAmountCents(req.Amount)
	if err != nil {
		return 0, "", time.Time{}, err
	}
	if req.CategoryID == 0 || req.AccountID == 0 {
		return 0, "", time.Time{}, errMissingReference
	}
	interval = database.RecurringInterval(req.Interval)
	switch interval {
	case database.IntervalWeekly, database.IntervalMonthly, database.IntervalYearly:
	default:
		return 0, "", time.Time{}, jsonError("Intervall muss weekly, monthly oder yearly sein.")
	}
	nextDue, err = time.Parse("2006-01-02", req.NextDueDate)
	if err != nil {
		return 0, "", time.Time{}, jsonError("Ungültiges Fälligkeitsdatum.")
	}
	return amountCents, interval, nextDue, nil
}

// ListRecurringTemplates returns every recurring template.
func (h *Handlers) ListRecurringTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := h.store.ListRecurringTemplates()
	if err != nil {
		h.logger.Error("list recurring templates failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Vorlagen konnten nicht geladen werden.")
		return
	}

	out := make([]recurringTemplateDTO, 0, len(templates))
	for _, rt := range templates {
		out = append(out, toRecurringTemplateDTO(rt))
	}
	writeJSON(w, http.StatusOK, out)
}

// CreateRecurringTemplate creates a new recurring template.
func (h *Handlers) CreateRecurringTemplate(w http.ResponseWriter, r *http.Request) {
	var req recurringTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage.")
		return
	}
	amountCents, interval, nextDue, err := req.parse()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := h.store.CreateRecurringTemplate(req.Name, amountCents, req.CategoryID, req.AccountID, interval, nextDue, req.Active)
	if err != nil {
		h.logger.Error("create recurring template failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Vorlage konnte nicht angelegt werden.")
		return
	}
	writeJSON(w, http.StatusCreated, toRecurringTemplateDTO(created))
}

// UpdateRecurringTemplate updates an existing recurring template.
func (h *Handlers) UpdateRecurringTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige ID.")
		return
	}

	var req recurringTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage.")
		return
	}
	amountCents, interval, nextDue, err := req.parse()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.store.UpdateRecurringTemplate(id, req.Name, amountCents, req.CategoryID, req.AccountID, interval, nextDue, req.Active)
	if err != nil {
		h.logger.Error("update recurring template failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Vorlage konnte nicht aktualisiert werden.")
		return
	}
	writeJSON(w, http.StatusOK, toRecurringTemplateDTO(updated))
}

// DeleteRecurringTemplate deletes a recurring template. Transactions it
// already generated are left untouched.
func (h *Handlers) DeleteRecurringTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige ID.")
		return
	}

	if err := h.store.DeleteRecurringTemplate(id); err != nil {
		h.logger.Error("delete recurring template failed", "error", err)
		writeError(w, http.StatusInternalServerError, "Vorlage konnte nicht gelöscht werden.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
