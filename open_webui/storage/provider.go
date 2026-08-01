package storage

import (
	"io"
	"os"
	"path/filepath"
)

type Provider interface {
	UploadFile(file io.Reader, filename string) ([]byte, string, error)
	GetFile(filePath string) (string, error)
	DeleteFile(filePath string) error
	DeleteAllFiles() error
}

type LocalProvider struct {
	UploadDir string
}

func NewLocalProvider(uploadDir string) *LocalProvider {
	return &LocalProvider{UploadDir: uploadDir}
}

func (p *LocalProvider) UploadFile(file io.Reader, filename string) ([]byte, string, error) {
	if err := os.MkdirAll(p.UploadDir, 0o755); err != nil {
		return nil, "", err
	}
	contents, err := io.ReadAll(file)
	if err != nil {
		return nil, "", err
	}
	path := filepath.Join(p.UploadDir, filename)
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		return nil, "", err
	}
	return contents, path, nil
}

func (p *LocalProvider) GetFile(filePath string) (string, error) {
	return filePath, nil
}

func (p *LocalProvider) DeleteFile(filePath string) error {
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (p *LocalProvider) DeleteAllFiles() error {
	if err := os.RemoveAll(p.UploadDir); err != nil {
		return err
	}
	return os.MkdirAll(p.UploadDir, 0o755)
}
