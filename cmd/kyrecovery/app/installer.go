package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/Busnes-app/ky-primitives/applysetup"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
	"github.com/Busnes-app/kyrecovery-server/internal/server"
)

func runInstaller(args []string) error {
	opts, err := applysetup.Parse(args)
	if err != nil {
		return err
	}
	if opts.Capabilities {
		return applysetup.WriteCapabilities(os.Stdout, "kyrecovery", "kyidentity-oidc", "exact-admin-subjects", "admin-only", "directory-webhook", "check")
	}
	if opts.Restart {
		return errors.New("KyRecovery setup activates live")
	}
	data := os.Getenv("KYRECOVERY_DATA_DIR")
	if data == "" {
		data = "./data"
	}
	socket, err := server.InstallerSocket(data)
	if err != nil {
		return err
	}
	var b applysetup.Common
	if err = applysetup.Decode(opts.File, &b); err != nil {
		return err
	}
	if err = db.ValidateInstaller(b); err != nil {
		return err
	}
	raw, _ := json.Marshal(b)
	r, err := applysetup.CallLocal(context.Background(), socket, raw, opts.Check)
	if err != nil {
		return err
	}
	if r.Service != b.Service || r.SetupID != b.SetupID {
		return errors.New("setup report identity mismatch")
	}
	return r.Write(os.Stdout)
}
