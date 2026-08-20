package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/portfolio/pf-workspace/api/internal/domain"
)

func (s *Server) uploadConfig(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, s.svc.UploadConfig())
}

func (s *Server) uploadLocal(w http.ResponseWriter, r *http.Request, sub string) {
	r.Body = http.MaxBytesReader(w, r.Body, domain.MaxUploadBytes+1024*1024)
	if err := r.ParseMultipartForm(domain.MaxUploadBytes); err != nil {
		if tooLarge(err) {
			writeErr(w, domain.ErrTooLarge)
			return
		}
		writeErr(w, domain.ErrInvalid)
		return
	}
	wsID := strings.TrimSpace(r.FormValue("workspaceId"))
	purpose := strings.TrimSpace(r.FormValue("purpose"))
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, domain.ErrInvalid)
		return
	}
	defer file.Close()
	contentType := header.Header.Get("Content-Type")
	view, err := s.ts(r.Context()).SaveLocalFile(sub, wsID, purpose, header.Filename, contentType, header.Size, file)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

func (s *Server) linkRemote(w http.ResponseWriter, r *http.Request, sub string) {
	var body struct {
		WorkspaceID string `json:"workspaceId"`
		Purpose     string `json:"purpose"`
		FileID      string `json:"fileId"`
		Name        string `json:"name"`
		ContentType string `json:"contentType"`
		Size        int64  `json:"size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	view, err := s.ts(r.Context()).LinkRemoteFile(sub, strings.TrimSpace(body.WorkspaceID), strings.TrimSpace(body.Purpose), strings.TrimSpace(body.FileID), body.Name, body.ContentType, body.Size)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

func (s *Server) attachPage(w http.ResponseWriter, r *http.Request, sub, pageID string) {
	var body struct {
		FileID string `json:"fileId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "invalid json"}})
		return
	}
	view, err := s.ts(r.Context()).AttachPageFile(sub, pageID, strings.TrimSpace(body.FileID))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

func (s *Server) listPageAttachments(w http.ResponseWriter, r *http.Request, sub, pageID string) {
	files, err := s.ts(r.Context()).ListPageFiles(sub, pageID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}

func (s *Server) getFileMeta(w http.ResponseWriter, r *http.Request, sub, fileID string) {
	view, err := s.ts(r.Context()).GetFileForMember(sub, fileID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) serveFileContent(w http.ResponseWriter, r *http.Request) {
	fileID := r.PathValue("id")
	token := strings.TrimSpace(r.URL.Query().Get("t"))
	f, err := s.svc.OpenFileByToken(fileID, token)
	if err != nil {
		writeErr(w, err)
		return
	}
	if f.Provider != domain.FileProviderLocal || f.Path == "" {
		writeErr(w, domain.ErrNotFound)
		return
	}
	in, err := os.Open(f.Path)
	if err != nil {
		writeErr(w, domain.ErrNotFound)
		return
	}
	defer in.Close()
	if f.ContentType != "" {
		w.Header().Set("Content-Type", f.ContentType)
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, in)
}

func tooLarge(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr) || errors.Is(err, domain.ErrTooLarge)
}
