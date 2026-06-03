package api

import (
	"encoding/json"
	"log"
	"net/http"

	"scout-app/internal/domain/sync"
)

type SyncHandler struct {
	svc *sync.Service
}

func NewSyncHandler(svc *sync.Service) *SyncHandler {
	return &SyncHandler{svc: svc}
}

func (h *SyncHandler) Sync(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.Sync(r.Context())
	if err != nil {
		log.Printf("Sync failed: %v", err)
		http.Error(w, "Sync failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
