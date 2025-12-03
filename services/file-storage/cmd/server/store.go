package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

var ErrNotFound = errors.New("file not found")

type FileStore struct {
	root   string
	logger *logrus.Logger
}

func NewFileStore(root string, logger *logrus.Logger) *FileStore {
	return &FileStore{
		root:   root,
		logger: logger,
	}
}

func (s *FileStore) Save(_ context.Context, r io.Reader, originalName string) (string, error) {
	id, err := randomID()
	if err != nil {
		return "", err
	}

	path := filepath.Join(s.root, id)
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	// persist original filename in sidecar
	metaPath := filepath.Join(s.root, id+".name")
	if err := os.WriteFile(metaPath, []byte(originalName), 0o644); err != nil {
		s.logger.WithError(err).Warn("failed to write filename metadata")
	}

	return id, nil
}

func (s *FileStore) Open(_ context.Context, id string) (io.ReadCloser, string, error) {
	path := filepath.Join(s.root, id)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", ErrNotFound
		}
		return nil, "", fmt.Errorf("open file: %w", err)
	}

	metaPath := filepath.Join(s.root, id+".name")
	filenameBytes, err := os.ReadFile(metaPath)
	filename := "file"
	if err == nil && len(filenameBytes) > 0 {
		filename = string(filenameBytes)
	}

	return f, filename, nil
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return hex.EncodeToString(b), nil
}


