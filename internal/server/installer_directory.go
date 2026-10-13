package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/Busnes-app/ky-primitives/applysetup"
	"github.com/Busnes-app/ky-primitives/syncauth"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
	"io"
	"net/http"
	"strings"
)

func (s *Server) installerDirectory() http.Handler {
	key := func(r *http.Request) ([]byte, error) {
		b, _, err := s.db.InstallerConfiguration(r.Context())
		if err != nil {
			return nil, err
		}
		if b.SSO == nil {
			return nil, io.ErrUnexpectedEOF
		}
		secret, err := applysetup.Secret(b.SSO.DirectorySecretFile)
		return []byte(secret), err
	}
	return syncauth.Middleware(key, syncauth.Options{}, 64<<10, nil)(http.HandlerFunc(s.handleInstallerDirectory))
}

func (s *Server) handleInstallerDirectory(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, 400, "Directory body invalid")
		return
	}
	var user struct {
		ID     string `json:"id"`
		Active *bool  `json:"active"`
		Roles  []struct {
			Value string `json:"value"`
		} `json:"roles"`
		Meta struct {
			Version string `json:"version"`
		} `json:"meta"`
	}
	if json.Unmarshal(body, &user) != nil || user.ID == "" || len(user.ID) > 256 {
		writeError(w, 400, "Directory identity invalid")
		return
	}
	signed, _ := syncauth.EventFromContext(r)
	version := strings.TrimSuffix(strings.TrimPrefix(user.Meta.Version, `W/"`), `"`)
	if user.Meta.Version != `W/"`+version+`"` {
		writeError(w, 400, "Directory revision invalid")
		return
	}
	if (signed.Type == "user.created" || signed.Type == "user.updated") && user.Active == nil {
		writeError(w, 400, "Directory active state required")
		return
	}
	sum := sha256.Sum256(append([]byte(signed.Type+"\x00"), body...))
	s.installerMu.Lock()
	defer s.installerMu.Unlock()
	b, managed, err := s.db.InstallerConfiguration(r.Context())
	if err != nil || !managed || b.SSO == nil {
		writeError(w, 503, "Directory unavailable")
		return
	}
	event := db.InstallerDirectory{Issuer: b.SSO.IssuerURL, Subject: user.ID, Event: signed.Type, Version: version, Digest: hex.EncodeToString(sum[:]), Active: user.Active != nil && *user.Active}
	for _, role := range user.Roles {
		event.Admin = event.Admin || role.Value == "kyrecovery.admin"
	}
	changed, err := s.db.InspectInstallerDirectory(r.Context(), event)
	if err != nil {
		writeError(w, 422, "Directory subject or revision refused")
		return
	}
	if changed {
		if err = s.ledger.ApplyInstallerDirectory(r.Context(), event); err != nil {
			writeError(w, 503, "Directory audit mutation failed")
			return
		}
	}
	w.WriteHeader(204)
}
