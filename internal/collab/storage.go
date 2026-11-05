package collab

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// StorageProvider abstracts binary file storage for attachments (Local Disk, S3, Cloudflare R2, GCS).
type StorageProvider interface {
	Save(ctx context.Context, filename string, r io.Reader) (url string, err error)
	Delete(ctx context.Context, key string) error
}

// LocalDiskStorage implements StorageProvider targeting a local or mounted volume directory.
type LocalDiskStorage struct {
	baseDir string
	baseURL string
}

func NewLocalDiskStorage(baseDir, baseURL string) (*LocalDiskStorage, error) {
	if baseDir == "" {
		baseDir = "./uploads"
	}
	if baseURL == "" {
		baseURL = "/uploads"
	}
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to initialize local disk storage dir: %w", err)
	}
	return &LocalDiskStorage{
		baseDir: baseDir,
		baseURL: strings.TrimSuffix(baseURL, "/"),
	}, nil
}

func (s *LocalDiskStorage) Save(ctx context.Context, filename string, r io.Reader) (string, error) {
	ext := filepath.Ext(filename)
	uniqueName := fmt.Sprintf("%s%s", uuid.NewString(), ext)
	dst := filepath.Join(s.baseDir, uniqueName)

	f, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("failed to write file content: %w", err)
	}

	return fmt.Sprintf("%s/%s", s.baseURL, uniqueName), nil
}

func (s *LocalDiskStorage) Delete(ctx context.Context, key string) error {
	dst := filepath.Join(s.baseDir, filepath.Base(key))
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// MemoryStorage implements in-memory StorageProvider for tests and isolated ephemeral environments.
type MemoryStorage struct {
	mu    sync.RWMutex
	files map[string][]byte
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{files: make(map[string][]byte)}
}

func (m *MemoryStorage) Save(ctx context.Context, filename string, r io.Reader) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return "", err
	}

	ext := filepath.Ext(filename)
	uniqueName := fmt.Sprintf("%s%s", uuid.NewString(), ext)
	m.files[uniqueName] = buf.Bytes()

	return fmt.Sprintf("/uploads/%s", uniqueName), nil
}

func (m *MemoryStorage) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, filepath.Base(key))
	return nil
}

