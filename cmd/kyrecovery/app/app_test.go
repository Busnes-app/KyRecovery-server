package app

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Busnes-app/kyrecovery-server/internal/auth"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	done := make(chan []byte)
	go func() { b, _ := io.ReadAll(r); done <- b }()
	fn()
	os.Stdout = old
	w.Close()
	return string(<-done)
}

func seedPairings(t *testing.T, dir string) (live, claimed, revoked, expired string) {
	t.Helper()
	database, err := db.Open(filepath.Join(dir, "kyrecovery.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	now := time.Now().UTC()
	add := func(id, code string, expires time.Time) {
		if err := database.InsertPairedApp(ctx, db.PairedAppRecord{ID: id, ServiceName: "kynotes", AppName: id, APIToken: "kyrec_live_" + id, PairingCode: code, Status: "pending", ExpiresAt: expires, CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	add("pair-live", "111111", now.Add(time.Hour))
	add("pair-claimed", "222222", now.Add(time.Hour))
	add("pair-revoked", "333333", now.Add(time.Hour))
	add("pair-expired", "444444", now.Add(-time.Minute))
	if _, err := database.ClaimPairingCode(ctx, "222222", "kynotes", "pair-claimed", "key"); err != nil {
		t.Fatal(err)
	}
	if err := database.RevokePairedApp(ctx, "pair-revoked"); err != nil {
		t.Fatal(err)
	}
	return "111111", "222222", "333333", "444444"
}

func TestPairListShowsOnlyClaimableCodes(t *testing.T) {
	dir := t.TempDir()
	live, claimed, revoked, expired := seedPairings(t, dir)
	out := captureStdout(t, func() { cmdPair([]string{"list", "--data-dir", dir}) })
	if !strings.Contains(out, live) {
		t.Errorf("live pending code missing:\n%s", out)
	}
	for _, code := range []string{claimed, revoked, expired} {
		if strings.Contains(out, code) {
			t.Errorf("code %s of a claimed, revoked or expired row printed:\n%s", code, out)
		}
	}
}

func TestPairHonoursDataDirEnv(t *testing.T) {
	dir := t.TempDir()
	live, _, _, _ := seedPairings(t, dir)
	t.Setenv("KYRECOVERY_DATA_DIR", dir)
	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, "data"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(cwd)
	out := captureStdout(t, func() { cmdPair([]string{"list"}) })
	if !strings.Contains(out, live) {
		t.Fatalf("pair list ignored KYRECOVERY_DATA_DIR:\n%s", out)
	}
}

func openBootstrap(t *testing.T, dir string) (*db.DB, *auth.Manager) {
	t.Helper()
	database, err := db.Open(filepath.Join(dir, "kyrecovery.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return database, auth.NewManager(auth.OIDCConfig{}, database)
}

// A generated password goes to a 0600 file in the data directory, never the log; a
// retry after a start that died before the account was written reuses it.
func TestGeneratedAdminPasswordGoesToAFileNotTheLog(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, initialAdminPasswordFile)
	if _, err := initialAdminPassword(path); err != nil {
		t.Fatal(err)
	}
	database, mgr := openBootstrap(t, dir)
	var out bytes.Buffer
	if err := bootstrapAdmin(ctx, database, mgr, dir, "", &out); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("no password file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("password file mode %o, want 0600", info.Mode().Perm())
	}
	raw, _ := os.ReadFile(path)
	pw := strings.TrimSpace(string(raw))
	u, err := database.GetUserByUsername(ctx, "admin")
	if err != nil || u == nil {
		t.Fatalf("no admin: %v", err)
	}
	if !auth.VerifyPassword(pw, u.PasswordHash) {
		t.Fatal("the file's password does not open the admin account")
	}
	if pw == "" || strings.Contains(out.String(), pw) || !strings.Contains(out.String(), path) {
		t.Errorf("output must name the file and never the password:\n%s", out.String())
	}

	// The account exists now: a restart leaves it and the file alone and prints nothing.
	out.Reset()
	if err := bootstrapAdmin(ctx, database, mgr, dir, "", &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("restart announced an admin:\n%s", out.String())
	}
	if raw2, _ := os.ReadFile(path); string(raw2) != string(raw) {
		t.Error("restart rewrote the password file")
	}
}

func TestSuppliedAdminPasswordIsNeverPrintedOrWritten(t *testing.T) {
	dir := t.TempDir()
	database, mgr := openBootstrap(t, dir)
	var out bytes.Buffer
	if err := bootstrapAdmin(context.Background(), database, mgr, dir, "operator-chose-this", &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "operator-chose-this") {
		t.Errorf("supplied password printed:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, initialAdminPasswordFile)); !os.IsNotExist(err) {
		t.Errorf("supplied password written to a file: %v", err)
	}
}

// If the file cannot be written, startup fails: no account, nothing printed.
func TestAdminBootstrapFailsWhenThePasswordFileCannotBeWritten(t *testing.T) {
	dir := t.TempDir()
	database, mgr := openBootstrap(t, dir)
	var out bytes.Buffer
	if err := bootstrapAdmin(context.Background(), database, mgr, filepath.Join(dir, "missing"), "", &out); err == nil {
		t.Fatal("bootstrap succeeded without a password file")
	}
	if u, _ := database.GetUserByUsername(context.Background(), "admin"); u != nil {
		t.Error("an admin was created whose password nobody can read")
	}
	if out.Len() != 0 {
		t.Errorf("failed bootstrap printed:\n%s", out.String())
	}
}
