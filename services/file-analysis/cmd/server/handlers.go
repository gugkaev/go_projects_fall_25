package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func createWorkHandler(s *AnalysisService, logger *logrus.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in CreateWorkInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_json", Details: "cannot parse request body"})
			return
		}
		if in.StudentID == "" || in.AssignmentID == "" || in.FileID == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "validation_error", Details: "student_id, assignment_id and file_id are required"})
			return
		}

		work, err := s.CreateWork(r.Context(), in)
		if err != nil {
			logger.WithError(err).Error("failed to create work")
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "analysis_error", Details: "failed to analyze work"})
			return
		}

		writeJSON(w, http.StatusCreated, work)
	}
}

func getWorkHandler(s *AnalysisService, logger *logrus.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_id", Details: "id must be numeric"})
			return
		}

		work, err := s.GetWork(r.Context(), uint(id))
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				writeJSON(w, http.StatusNotFound, errorResponse{Error: "not_found", Details: "work not found"})
				return
			}
			logger.WithError(err).Error("failed to get work")
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "db_error", Details: "failed to fetch work"})
			return
		}

		writeJSON(w, http.StatusOK, work)
	}
}

func wordCloudHandler(s *AnalysisService, logger *logrus.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_id", Details: "id must be numeric"})
			return
		}

		url, err := s.GenerateWordCloud(r.Context(), uint(id))
		if err != nil {
			logger.WithError(err).Error("failed to generate wordcloud")
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "wordcloud_error", Details: "failed to generate wordcloud"})
			return
		}

		writeJSON(w, http.StatusOK, WordCloudResponse{URL: url})
	}
}


