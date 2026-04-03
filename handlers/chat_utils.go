package handlers

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"llm-gateway/models"
)

var (
	bufferPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
	gzipReaderPool = sync.Pool{
		New: func() interface{} {
			return new(gzip.Reader)
		},
	}
)

func gzipDecode(input []byte) ([]byte, error) {
	reader := gzipReaderPool.Get().(*gzip.Reader)
	defer gzipReaderPool.Put(reader)

	if err := reader.Reset(bytes.NewReader(input)); err != nil {
		return nil, err
	}
	defer reader.Close()

	buffer := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(buffer)
	buffer.Reset()

	_, err := io.Copy(buffer, reader)
	if err != nil {
		return nil, err
	}

	result := make([]byte, buffer.Len())
	copy(result, buffer.Bytes())
	return result, nil
}

type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type ModelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

func (h *ChatHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	if h.handleCORS(w, r) {
		return
	}

	var configs []models.ModelConfig
	if err := h.DB.Where("enabled = ?", true).Find(&configs).Error; err != nil {
		http.Error(w, "Failed to get models", http.StatusInternalServerError)
		return
	}

	now := time.Now().Unix()
	modelsList := make([]Model, len(configs))
	for i, c := range configs {
		modelsList[i] = Model{
			ID:      c.Name,
			Object:  "model",
			Created: now,
			OwnedBy: "system",
		}
	}

	response := ModelsResponse{
		Object: "list",
		Data:   modelsList,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
