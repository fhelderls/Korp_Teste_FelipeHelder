package handlers

import (
	"encoding/json"
	"korp-teste/estoque-service/models"
	"korp-teste/estoque-service/store"
	"net/http"
)

type ProdutosHandlers struct {
	store *store.ProdutosStore
}

func NewProdutosHandlers(s *store.ProdutosStore) *ProdutosHandlers {
	return &ProdutosHandlers{store: s}
}

// Create recebe um produto em JSON no corpo da requisicao e cria no banco.
func (h *ProdutosHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var produto models.Produto
	if err := json.NewDecoder(r.Body).Decode(&produto); err != nil {
		http.Error(w, "corpo da requisicao invalido", http.StatusBadRequest)
		return
	}

	if err := h.store.Create(&produto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return

	}
	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(produto)

}

// list retorna todos os produtos cadastrados.

func (h *ProdutosHandlers) List(w http.ResponseWriter, r *http.Request) {
	produtos, err := h.store.GetAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-type", "application/json")
	json.NewEncoder(w).Encode(produtos)

}

// Delete remove um produto pelo codigo, lido da URL.
func (h *ProdutosHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	codigo := r.PathValue("codigo")
	if err := h.store.Delete(codigo); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
