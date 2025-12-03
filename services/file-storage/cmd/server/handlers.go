package main

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
)

type uploadResponse struct {
	FileID string `json:"file_id"`
}

type errorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

func uploadHandler(store *FileStore, logger *logrus.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(maxUploadSizeB); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_multipart", "failed to parse multipart form")
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "file_required", "form field 'file' is required")
			return
		}
		defer file.Close()

		if header.Size > maxUploadSizeB {
			writeError(w, http.StatusRequestEntityTooLarge, "file_too_large", "file exceeds max size")
			return
		}

		id, err := store.Save(r.Context(), file, header.Filename)
		if err != nil {
			logger.WithError(err).Error("failed to save file")
			writeError(w, http.StatusInternalServerError, "storage_error", "failed to save file")
			return
		}

		w.Header().Set("Content-Type", contentTypeJSON)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(uploadResponse{FileID: id})
	}
}

func downloadHandler(store *FileStore, logger *logrus.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "id_required", "file id is required")
			return
		}

		reader, filename, err := store.Open(r.Context(), id)
		if err != nil {
			if err == ErrNotFound {
				writeError(w, http.StatusNotFound, "not_found", "file not found")
				return
			}
			logger.WithError(err).Error("failed to open file")
			writeError(w, http.StatusInternalServerError, "storage_error", "failed to open file")
			return
		}
		defer reader.Close()

		w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
		w.WriteHeader(http.StatusOK)
		if _, err := io.Copy(w, reader); err != nil {
			logger.WithError(err).Error("failed to stream file")
		}
	}
}

func writeError(w http.ResponseWriter, code int, errCode, details string) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error:   errCode,
		Details: details,
	})
}


