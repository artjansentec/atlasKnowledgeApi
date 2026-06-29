package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

type FileStorage interface {
	Save(ctx context.Context, originalName string, reader io.Reader) (storageKey string, err error)
	Open(ctx context.Context, storageKey string) (io.ReadCloser, error)
	Delete(ctx context.Context, storageKey string) error
}

type LocalFileStorage struct {
	basePath string
}

func NewLocalFileStorage(basePath string) (*LocalFileStorage, error) {
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	return &LocalFileStorage{basePath: basePath}, nil
}

func (s *LocalFileStorage) Save(_ context.Context, originalName string, reader io.Reader) (string, error) {
	key := uuid.NewString() + filepath.Ext(originalName)
	fullPath := filepath.Join(s.basePath, key)

	file, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		_ = os.Remove(fullPath)
		return "", fmt.Errorf("write file: %w", err)
	}

	return key, nil
}

func (s *LocalFileStorage) Open(_ context.Context, storageKey string) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.basePath, filepath.Base(storageKey))
	return os.Open(fullPath)
}

func (s *LocalFileStorage) Delete(_ context.Context, storageKey string) error {
	fullPath := filepath.Join(s.basePath, filepath.Base(storageKey))
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}
