package service

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/portfolio/pf-workspace/api/internal/domain"
	"github.com/portfolio/pf-workspace/api/internal/id"
)

type UploadConfig struct {
	Provider    string `json:"provider"`
	MaxBytes    int64  `json:"maxBytes"`
	MediaAPIURL string `json:"mediaApiUrl,omitempty"`
}

type FileView struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	Purpose     string `json:"purpose"`
	Provider    string `json:"provider"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	Name        string `json:"name"`
	URL         string `json:"url"`
}

func (s *Service) UploadConfig() UploadConfig {
	cfg := UploadConfig{Provider: domain.FileProviderLocal, MaxBytes: domain.MaxUploadBytes}
	if s.mediaURL != "" {
		cfg.Provider = domain.FileProviderP03
		cfg.MediaAPIURL = s.mediaURL
	}
	return cfg
}

func (s *Service) FileURL(f domain.StoredFile) string {
	return s.publicURL + "/v1/files/" + f.ID + "/content?t=" + f.ViewToken
}

func (s *Service) toFileView(f domain.StoredFile) FileView {
	return FileView{
		ID:          f.ID,
		WorkspaceID: f.WorkspaceID,
		Purpose:     f.Purpose,
		Provider:    f.Provider,
		ContentType: f.ContentType,
		Size:        f.Size,
		Name:        f.Name,
		URL:         s.FileURL(f),
	}
}

func (s *Service) SaveLocalFile(sub, wsID, purpose, name, contentType string, size int64, r io.Reader) (FileView, error) {
	if err := s.requireWrite(wsID, sub); err != nil {
		return FileView{}, err
	}
	if !domain.ValidFilePurpose(purpose) {
		return FileView{}, domain.ErrInvalid
	}
	if size < 0 || size > domain.MaxUploadBytes {
		return FileView{}, domain.ErrTooLarge
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == ".." {
		name = "upload"
	}
	if err := os.MkdirAll(s.uploadDir, 0o700); err != nil {
		return FileView{}, err
	}
	fileID := id.New()
	path := filepath.Join(s.uploadDir, fileID)
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return FileView{}, err
	}
	defer out.Close()
	written, err := io.Copy(out, io.LimitReader(r, domain.MaxUploadBytes+1))
	if err != nil {
		_ = os.Remove(path)
		return FileView{}, err
	}
	if written > domain.MaxUploadBytes {
		_ = os.Remove(path)
		return FileView{}, domain.ErrTooLarge
	}
	f := domain.StoredFile{
		ID:          fileID,
		WorkspaceID: wsID,
		UploaderSub: sub,
		Purpose:     purpose,
		Provider:    domain.FileProviderLocal,
		ContentType: contentType,
		Size:        written,
		Name:        name,
		ViewToken:   id.New(),
		Path:        path,
		CreatedAt:   s.now().UTC(),
	}
	if err := s.store.SaveFile(f); err != nil {
		_ = os.Remove(path)
		return FileView{}, err
	}
	return s.toFileView(f), nil
}

func (s *Service) LinkRemoteFile(sub, wsID, purpose, fileID, name, contentType string, size int64) (FileView, error) {
	if s.mediaURL == "" {
		return FileView{}, domain.ErrInvalid
	}
	if err := s.requireWrite(wsID, sub); err != nil {
		return FileView{}, err
	}
	if !domain.ValidFilePurpose(purpose) {
		return FileView{}, domain.ErrInvalid
	}
	fileID = strings.TrimSpace(fileID)
	if fileID == "" || id.Parse(fileID) != nil {
		return FileView{}, domain.ErrInvalid
	}
	if size < 0 || size > domain.MaxUploadBytes {
		return FileView{}, domain.ErrTooLarge
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == ".." {
		name = "upload"
	}
	f := domain.StoredFile{
		ID:          fileID,
		WorkspaceID: wsID,
		UploaderSub: sub,
		Purpose:     purpose,
		Provider:    domain.FileProviderP03,
		ContentType: contentType,
		Size:        size,
		Name:        name,
		ViewToken:   id.New(),
		CreatedAt:   s.now().UTC(),
	}
	if err := s.store.SaveFile(f); err != nil {
		return FileView{}, err
	}
	return s.toFileView(f), nil
}

func (s *Service) AttachPageFile(sub, pageID, fileID string) (FileView, error) {
	page, err := s.GetPage(sub, pageID)
	if err != nil {
		return FileView{}, err
	}
	if err := s.requireWrite(page.WorkspaceID, sub); err != nil {
		return FileView{}, err
	}
	f, err := s.store.GetFile(fileID)
	if err != nil {
		return FileView{}, err
	}
	if f.WorkspaceID != page.WorkspaceID || f.Purpose != domain.PurposeWiki {
		return FileView{}, domain.ErrForbidden
	}
	if err := s.store.AttachPageFile(pageID, fileID); err != nil {
		return FileView{}, err
	}
	return s.toFileView(f), nil
}

func (s *Service) ListPageFiles(sub, pageID string) ([]FileView, error) {
	if _, err := s.GetPage(sub, pageID); err != nil {
		return nil, err
	}
	files, err := s.store.ListPageFiles(pageID)
	if err != nil {
		return nil, err
	}
	out := make([]FileView, 0, len(files))
	for _, f := range files {
		out = append(out, s.toFileView(f))
	}
	return out, nil
}

func (s *Service) GetFileForMember(sub, fileID string) (FileView, error) {
	f, err := s.store.GetFile(fileID)
	if err != nil {
		return FileView{}, err
	}
	if err := s.requireRead(f.WorkspaceID, sub); err != nil {
		return FileView{}, err
	}
	return s.toFileView(f), nil
}

func (s *Service) OpenFileByToken(fileID, token string) (domain.StoredFile, error) {
	f, err := s.Unscoped().store.GetFile(fileID)
	if err != nil {
		return domain.StoredFile{}, err
	}
	if token == "" || token != f.ViewToken {
		return domain.StoredFile{}, domain.ErrUnauthorized
	}
	return f, nil
}

func (s *Service) requireChatFile(wsID, fileID string) error {
	f, err := s.store.GetFile(fileID)
	if err != nil {
		return domain.ErrInvalid
	}
	if f.WorkspaceID != wsID || f.Purpose != domain.PurposeChat {
		return domain.ErrForbidden
	}
	return nil
}
