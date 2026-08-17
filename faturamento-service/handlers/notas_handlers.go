package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"korp-teste/faturamento-service/client"
	"korp-teste/faturamento-service/models"
	"korp-teste/faturamento-service/store"
)

type NotasHandlers struct {
	store         *store.NotasStore
	estoqueClient *client.EstoqueClient
}

func NewNotasHandlers(s *store.NotasStore, ec *client.EstoqueClient) *NotasHandlers {
	return &NotasHandlers{store: s, estoqueClient: ec}
}

type emitirRequest struct {
	Chave   string            `json:"chave"`
	Cliente string            `json:"cliente"`
	Itens   []models.ItemNota `json:"itens"`
}

// Emitir orquestra a emissao de uma nota fiscal: grava a nota, reserva o
// estoque, confirma a reserva. Se qualquer etapa falhar depois da reserva,
// cancela a reserva para devolver o estoque (compensacao). Se a chave ja
// existir, reaproveita a nota existente em vez de tentar criar de novo,
// permitindo que o cliente reenvie a mesma requisicao com seguranca.
func (h *NotasHandlers) Emitir(w http.ResponseWriter, r *http.Request) {
	var req emitirRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "corpo da requisicao invalido", http.StatusBadRequest)
		return
	}

	nota, err := h.store.GetByChave(req.Chave)
	if err == sql.ErrNoRows {
		nota = models.NotaFiscal{
			Chave:   req.Chave,
			Cliente: req.Cliente,
			Itens:   req.Itens,
		}
		if err := h.store.Create(&nota); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if nota.Status == "emitida" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(nota)
		return
	}

	if err := h.estoqueClient.Reservar(nota.Chave, nota.Itens); err != nil {
		h.store.AtualizarStatus(nota.Chave, "falha")
		http.Error(w, "nao foi possivel reservar o estoque, nota marcada como falha: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	if err := h.estoqueClient.Confirmar(nota.Chave); err != nil {
		h.estoqueClient.Cancelar(nota.Chave)
		h.store.AtualizarStatus(nota.Chave, "falha")
		http.Error(w, "falha ao confirmar a emissao, reserva de estoque cancelada: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	h.store.AtualizarStatus(nota.Chave, "emitida")
	nota.Status = "emitida"

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(nota)
}

// List retorna todas as notas fiscais cadastradas.
func (h *NotasHandlers) List(w http.ResponseWriter, r *http.Request) {
	notas, err := h.store.GetAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notas)
}
