package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
)

type uploadWorkResponse struct {
	WorkID uint `json:"work_id"`
}

type gatewayError struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

func registerRoutes(r *chi.Mux, logger *logrus.Logger) {
	fileStorageURL := getEnv("FILE_STORAGE_URL", "http://file-storage:8081")
	analysisURL := getEnv("ANALYSIS_URL", "http://file-analysis:8082")

	r.Post("/works", uploadWorkHandler(fileStorageURL, analysisURL, logger))
	r.Get("/works/{id}", proxyHandler(analysisURL, "/works/{id}", logger))
	r.Get("/works/{id}/wordcloud", proxyHandler(analysisURL, "/works/{id}/wordcloud", logger))
}

func uploadWorkHandler(fileStorageURL, analysisURL string, logger *logrus.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(maxUploadSizeB); err != nil {
			writeGatewayError(w, http.StatusBadRequest, "invalid_multipart", "failed to parse multipart form")
			return
		}

		studentID := r.FormValue("student_id")
		assignmentID := r.FormValue("assignment_id")
		if studentID == "" || assignmentID == "" {
			writeGatewayError(w, http.StatusBadRequest, "validation_error", "student_id and assignment_id are required")
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			writeGatewayError(w, http.StatusBadRequest, "file_required", "form field 'file' is required")
			return
		}
		defer file.Close()

		if header.Size > maxUploadSizeB {
			writeGatewayError(w, http.StatusRequestEntityTooLarge, "file_too_large", "file exceeds max size")
			return
		}

		fileID, err := uploadToFileStorage(fileStorageURL, file, header.Filename)
		if err != nil {
			logger.WithError(err).Error("file-storage unavailable")
			writeGatewayError(w, http.StatusBadGateway, "file_storage_unavailable", "failed to store file")
			return
		}

		payload := map[string]string{
			"student_id":    studentID,
			"assignment_id": assignmentID,
			"file_id":       fileID,
		}
		body, _ := json.Marshal(payload)

		req, err := http.NewRequest(http.MethodPost, analysisURL+"/works", bytes.NewReader(body))
		if err != nil {
			writeGatewayError(w, http.StatusInternalServerError, "internal_error", "failed to build analysis request")
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			logger.WithError(err).Error("analysis-service unavailable")
			writeGatewayError(w, http.StatusBadGateway, "analysis_unavailable", "file stored but analysis service unavailable")
			return
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 500 {
			logger.WithFields(logrus.Fields{
				"status": resp.StatusCode,
				"body":   string(respBody),
			}).Error("analysis service 5xx")
			writeGatewayError(w, http.StatusBadGateway, "analysis_error", "analysis service failed")
			return
		}
		if resp.StatusCode >= 400 {
			writeGatewayError(w, resp.StatusCode, "analysis_client_error", string(respBody))
			return
		}

		w.Header().Set("Content-Type", contentTypeJSON)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(respBody)
	}
}

func proxyHandler(baseURL, pattern string, logger *logrus.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		target := baseURL + "/works/" + id
		if strings.HasSuffix(r.URL.Path, "/wordcloud") {
			target = baseURL + "/works/" + id + "/wordcloud"
		}

		req, err := http.NewRequest(r.Method, target, nil)
		if err != nil {
			writeGatewayError(w, http.StatusInternalServerError, "internal_error", "failed to build proxy request")
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			logger.WithError(err).Error("analysis-service unavailable")
			writeGatewayError(w, http.StatusBadGateway, "analysis_unavailable", "analysis service unavailable")
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", contentTypeJSON)
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}
}

func uploadToFileStorage(baseURL string, file io.Reader, filename string) (string, error) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer writer.Close()

		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, file); err != nil {
			pw.CloseWithError(err)
			return
		}
	}()

	req, err := http.NewRequest(http.MethodPost, baseURL+"/files", pr)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("file-storage returned %d: %s", resp.StatusCode, string(b))
	}

	var out struct {
		FileID string `json:"file_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.FileID, nil
}

func writeGatewayError(w http.ResponseWriter, code int, errCode, details string) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(gatewayError{
		Error:   errCode,
		Details: details,
	})
}


