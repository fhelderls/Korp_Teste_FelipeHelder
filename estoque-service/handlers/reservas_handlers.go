package handlers

import (
	"encoding/json"
	"korp-teste/estoque-service/models"
	"korp-teste/estoque-service/store"
	"net/http"
)

type ReservasHandlers struct {
	store *store.ReservasStore
}

func NewReservasHandlers(s *store.ReservasStore) *ReservasHandlers {
	return &ReservasHandlers{store: s}

}

type reservarRequest struct {
	Chave string               `json:"chave"`
	Itens []models.ItemReserva `json:"itens"`
}

// Reservar receve Chave de idempotencia e os itens no corpo da requisicao.
func (h *ReservasHandlers) Reservar(w http.ResponseWriter, r *http.Request) {

	var req reservarRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "corpo da requisicao invalido", http.StatusBadRequest)
		return
	}
	if req.Chave == "" {
		http.Error(w, "Chave e obrigatoria", http.StatusBadRequest)
		return
	}

	if err := h.store.Reservar(req.Chave, req.Itens); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusCreated)

}

// Confirmar le a chave a partir da URL (ex: /reservas/{chave}/confirmar).
func (h *ReservasHandlers) Confirmar(w http.ResponseWriter, r *http.Request) {
	chave := r.PathValue("chave")

	if err := h.store.Confirmar(chave); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return

	}
	w.WriteHeader(http.StatusOK)
}

// Cancelar le a chave a partir da URL (ex: /reservas/{chaves}/cancelar).
func (h *ReservasHandlers) Cancelar(w http.ResponseWriter, r *http.Request) {
	chave := r.PathValue("chave")

	if err := h.store.Cancelar(chave); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
}

