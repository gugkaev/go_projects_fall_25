package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type AnalysisService struct {
	db            *gorm.DB
	logger        *logrus.Logger
	fileStoreURL  string
	quickChartURL string
}

func NewAnalysisService(db *gorm.DB, logger *logrus.Logger) *AnalysisService {
	fsURL := os.Getenv("FILE_STORAGE_URL")
	if fsURL == "" {
		fsURL = "http://file-storage:8081"
	}
	qcURL := os.Getenv("QUICKCHART_URL")
	if qcURL == "" {
		qcURL = "https://quickchart.io/wordcloud"
	}
	return &AnalysisService{
		db:            db,
		logger:        logger,
		fileStoreURL:  strings.TrimRight(fsURL, "/"),
		quickChartURL: qcURL,
	}
}

type CreateWorkInput struct {
	StudentID    string `json:"student_id"`
	AssignmentID string `json:"assignment_id"`
	FileID       string `json:"file_id"`
}

type WordCloudResponse struct {
	URL string `json:"url"`
}

func (s *AnalysisService) CreateWork(ctx context.Context, in CreateWorkInput) (*Work, error) {
	content, err := s.fetchFile(ctx, in.FileID)
	if err != nil {
		return nil, fmt.Errorf("fetch file: %w", err)
	}

	hash := hashContent(content)
		
	var existing Work
	var plagiarism bool
	if err := s.db.WithContext(ctx).
		Where("assignment_id = ? AND student_id <> ? AND file_id IN (?)", in.AssignmentID, in.StudentID, s.subqueryFileIDsByContent(ctx, hash)).
		First(&existing).Error; err == nil {
		plagiarism = true
	}

	work := &Work{
		StudentID:    in.StudentID,
		AssignmentID: in.AssignmentID,
		FileID:       in.FileID,
		Plagiarism:   plagiarism,
		Status:       StatusCompleted,
	}

	if err := s.db.WithContext(ctx).Create(work).Error; err != nil {
		return nil, err
	}

	return work, nil
}

func (s *AnalysisService) subqueryFileIDsByContent(ctx context.Context, hash string) []string {
	_ = ctx
	_ = hash
	return []string{}
}

func (s *AnalysisService) GetWork(ctx context.Context, id uint) (*Work, error) {
	var work Work
	if err := s.db.WithContext(ctx).First(&work, id).Error; err != nil {
		return nil, err
	}
	return &work, nil
}

func (s *AnalysisService) GenerateWordCloud(ctx context.Context, id uint) (string, error) {
	work, err := s.GetWork(ctx, id)
	if err != nil {
		return "", err
	}

	content, err := s.fetchFile(ctx, work.FileID)
	if err != nil {
		return "", fmt.Errorf("fetch file: %w", err)
	}

	params := url.Values{}
	params.Set("text", string(content))
	wordCloudURL := fmt.Sprintf("%s?%s", s.quickChartURL, params.Encode())

	return wordCloudURL, nil
}

func (s *AnalysisService) fetchFile(ctx context.Context, fileID string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/files/%s", s.fileStoreURL, fileID), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("file storage returned %d: %s", resp.StatusCode, string(b))
	}
	return io.ReadAll(resp.Body)
}

func hashContent(b []byte) string {
	h := 0
	for _, c := range b {
		h = (h*31 + int(c)) & 0x7fffffff
	}
	return fmt.Sprintf("%d", h)
}

type errorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}


