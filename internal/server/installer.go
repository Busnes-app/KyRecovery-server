package server

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/Busnes-app/ky-primitives/applysetup"
	"github.com/Busnes-app/kyrecovery-server/internal/auth"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
)

func InstallerSocket(dataDir string) (string, error) {
	dir, err := filepath.Abs(dataDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "installer", "setup.sock"), nil
}

func (s *Server) ApplyInstaller(ctx context.Context, raw []byte, check bool) (applysetup.Report, error) {
	var b applysetup.Common
	if err := applysetup.DecodeRaw(raw, &b); err != nil {
		return applysetup.Report{}, err
	}
	r := applysetup.NewReport(b)
	if err := db.ValidateInstaller(b); err != nil {
		return r, err
	}
	if _, err := applysetup.Secret(b.SSO.ClientSecretFile); err != nil {
		return r, err
	}
	secret, err := applysetup.Secret(b.SSO.DirectorySecretFile)
	if err != nil {
		return r, err
	}
	if len(secret) < 16 {
		return r, errors.New("directory credential too short")
	}
	s.installerMu.Lock()
	defer s.installerMu.Unlock()
	if s.ledger.Healthy() != nil {
		return r, errors.New("setup requires a healthy audit ledger")
	}
	if _, err = s.ledger.Verify(ctx); err != nil {
		return r, errors.New("setup requires a verified audit ledger")
	}
	_, managed, err := s.db.InstallerConfiguration(ctx)
	if err != nil {
		return r, err
	}
	if !managed && s.authMgr.GetConfig() != (auth.OIDCConfig{}) {
		return r, errors.New("installer refuses active native SSO configuration")
	}
	r, err = s.db.InspectInstaller(ctx, b)
	if check || err == nil {
		return r, err
	}
	if !errors.Is(err, db.ErrInstallerMissing) {
		return r, err
	}
	if managed {
		return r, errors.New("retained installer identity missing; explicit repair required")
	}
	if err = s.ledger.ApplyInstaller(ctx, b); err != nil {
		return r, err
	}
	r, err = s.db.InspectInstaller(ctx, b)
	if err != nil {
		return r, err
	}
	r.Created = r.Present
	r.Present = []applysetup.Object{}
	return r, nil
}
