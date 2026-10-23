package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Busnes-app/ky-primitives/applysetup"
	"github.com/Busnes-app/kyrecovery-server/internal/audit"
	"github.com/Busnes-app/kyrecovery-server/internal/auth"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
	"github.com/Busnes-app/kyrecovery-server/internal/pairing"
	"github.com/Busnes-app/kyrecovery-server/internal/server"
	"github.com/Busnes-app/kyrecovery-server/pkg/client"
)

// version is this KyRecovery release; release-image.yml refuses a tag that disagrees.
const version = "0.4.2"

func Run(args []string) {
	if len(args) < 2 {
		printUsage()
		return
	}

	command := args[1]
	switch command {
	case "apply-setup":
		if err := runInstaller(args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "Product setup refused:", err)
			os.Exit(1)
		}
	case "serve":
		cmdServe(args[2:])
	case "audit":
		cmdAudit(args[2:])
	case "pair":
		cmdPair(args[2:])
	case "version", "--version":
		fmt.Println(version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q. Run 'kyrecovery help' for usage.\n", command)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`KyRecovery — Disaster Recovery & Verification Service for the Ky Suite

Usage:
  kyrecovery <command> [options]

Commands:
  serve           Start KyRecovery web dashboard and REST API daemon
  audit           Inspect or verify cryptographic audit log chain
  pair            Manage paired product connectors and ephemeral 6-digit codes
  version         Print the KyRecovery version

Run 'kyrecovery <command> --help' for details on each command.`)
}

// 1. Serve
func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 8095, "HTTP server listener port")
	dataDir := fs.String("data-dir", defaultDataDir(), "Directory for SQLite database and capsule storage")
	ssoIssuer := fs.String("sso-issuer", os.Getenv("KY_SSO_ISSUER"), "KySignOn OIDC Issuer URL (e.g. http://localhost:8080)")
	ssoClientID := fs.String("sso-client-id", os.Getenv("KY_SSO_CLIENT_ID"), "KySignOn OAuth Client ID")
	ssoClientSecret := fs.String("sso-client-secret", os.Getenv("KY_SSO_CLIENT_SECRET"), "KySignOn OAuth Client Secret")
	ssoRedirectURL := fs.String("sso-redirect-url", os.Getenv("KY_SSO_REDIRECT_URL"), "KySignOn OAuth Callback Redirect URL")
	ssoAdminEmail := fs.String("sso-admin-email", os.Getenv("KY_ADMIN_EMAIL"), "Admin user email address")
	resticDir := fs.String("restic-dir", os.Getenv("KYRECOVERY_RESTIC_DIR"), "rest-server data directory, mounted read-only, for backup status (empty disables)")
	cookieSecure := fs.String("cookie-secure", os.Getenv("KYRECOVERY_COOKIE_SECURE"), "Force the Secure flag on session cookies: true, false, or empty to follow the request transport")
	fs.Parse(args)

	var cookieSecureFlag *bool
	switch strings.ToLower(strings.TrimSpace(*cookieSecure)) {
	case "true", "1", "yes":
		v := true
		cookieSecureFlag = &v
	case "false", "0", "no":
		v := false
		cookieSecureFlag = &v
	case "":
	default:
		fmt.Fprintf(os.Stderr, "Invalid --cookie-secure value %q (expected true, false or empty)\n", *cookieSecure)
		os.Exit(1)
	}

	if err := os.MkdirAll(*dataDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create data directory: %v\n", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(*dataDir, "kyrecovery.db")
	database, err := db.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Database initialization error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	if *ssoRedirectURL == "" && *ssoIssuer != "" {
		*ssoRedirectURL = fmt.Sprintf("http://localhost:%d/api/auth/callback", *port)
	}

	authCfg := auth.OIDCConfig{
		Enabled:      *ssoIssuer != "",
		IssuerURL:    *ssoIssuer,
		ClientID:     *ssoClientID,
		ClientSecret: *ssoClientSecret,
		RedirectURL:  *ssoRedirectURL,
		AdminEmail:   *ssoAdminEmail,
	}

	authMgr := auth.NewManager(authCfg, database)
	if err := bootstrapAdmin(context.Background(), database, authMgr, *dataDir, os.Getenv("KY_ADMIN_INITIAL_PASSWORD"), os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Admin bootstrap failed: %v\n", err)
		os.Exit(1)
	}

	ledger := audit.NewLedger(database)
	var trusted []netip.Prefix
	for _, value := range strings.Split(os.Getenv("KYRECOVERY_TRUSTED_PROXIES"), ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		prefix, e := netip.ParsePrefix(value)
		if e != nil {
			addr, ae := netip.ParseAddr(value)
			if ae == nil {
				prefix = netip.PrefixFrom(addr.Unmap(), addr.Unmap().BitLen())
				e = nil
			}
		}
		if e != nil || prefix.Bits() == 0 {
			fmt.Fprintln(os.Stderr, "Invalid trusted proxy")
			os.Exit(1)
		}
		trusted = append(trusted, prefix.Masked())
	}
	srv, err := server.New(server.Config{
		TrustedProxies: trusted,
		Port:           *port,
		DataDir:        *dataDir,
		DatabasePath:   dbPath,
		Auth:           authCfg,
		CookieSecure:   cookieSecureFlag,
		ResticDir:      *resticDir,
	}, database, ledger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Server initialization error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	socket, err := server.InstallerSocket(*dataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Installer socket unavailable")
		os.Exit(1)
	}
	closeSetup, err := applysetup.StartLocal(ctx, socket, srv.ApplyInstaller, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Installer listener unavailable")
		os.Exit(1)
	}
	defer closeSetup()

	fmt.Printf("⚡ KyRecovery Server listening on http://0.0.0.0:%d (data: %s)\n", *port, *dataDir)
	if cookieSecureFlag == nil {
		fmt.Println("   Serving plain HTTP: sessions started over HTTP get cookies without the Secure flag.")
		fmt.Println("   Terminate TLS in front of KyRecovery (or set --cookie-secure=true) for production use.")
	}
	if err := srv.Start(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

// 2. Audit
func cmdAudit(args []string) {
	if len(args) == 0 || args[0] != "verify" {
		fmt.Println("Usage: kyrecovery audit verify [--data-dir DIR]")
		return
	}

	fs := flag.NewFlagSet("audit verify", flag.ExitOnError)
	dataDir := fs.String("data-dir", defaultDataDir(), "Data directory containing SQLite database")
	fs.Parse(args[1:])

	dbPath := filepath.Join(*dataDir, "kyrecovery.db")
	database, err := db.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed opening database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	ledger := audit.NewLedger(database)
	anchor, err := ledger.Verify(context.Background())
	if err != nil {
		fmt.Printf("✗ Audit Chain Broken: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Cryptographic Audit Chain Valid (%d events verified)\n", anchor.Count)
	fmt.Printf("  Latest Hash: %s\n", anchor.Hash)
}

// 3. Pair
func cmdPair(args []string) {
	if len(args) == 0 {
		fmt.Println(`Usage:
  kyrecovery pair generate [--ttl 15] [--service <name>] [--app <name>] [--data-dir DIR]
  kyrecovery pair list [--data-dir DIR]
  kyrecovery pair claim --server <url> --code <6-digit-code> --service <service> --app <app-name>`)
		return
	}

	action := args[0]
	switch action {
	case "generate":
		fs := flag.NewFlagSet("pair generate", flag.ExitOnError)
		ttl := fs.Int("ttl", 15, "Pairing code expiration in minutes")
		service := fs.String("service", "auto-declare", "Expected service name")
		app := fs.String("app", "Connected App", "Expected app name")
		dataDir := fs.String("data-dir", defaultDataDir(), "Data directory")
		fs.Parse(args[1:])

		dbPath := filepath.Join(*dataDir, "kyrecovery.db")
		database, err := db.Open(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed opening database: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()

		rec, err := pairing.GeneratePairingCode(context.Background(), database, time.Duration(*ttl)*time.Minute, *service, *app)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed generating pairing code: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("=== PAIRING CODE GENERATED ===")
		fmt.Printf("Pairing Code: %s\n", rec.PairingCode)
		fmt.Printf("Expires In:   %d minutes (%s)\n", *ttl, rec.ExpiresAt.Format("15:04:05 UTC"))
		fmt.Println("\nEnter this 6-digit code in the Ky Suite app or busnes.app service to establish automated backup sync.")

	case "list":
		fs := flag.NewFlagSet("pair list", flag.ExitOnError)
		dataDir := fs.String("data-dir", defaultDataDir(), "Data directory")
		fs.Parse(args[1:])

		dbPath := filepath.Join(*dataDir, "kyrecovery.db")
		database, err := db.Open(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed opening database: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()

		list, err := database.ListPairedApps(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed listing paired apps: %v\n", err)
			os.Exit(1)
		}

		if len(list) == 0 {
			fmt.Println("No paired applications or active pairing codes found.")
			return
		}

		fmt.Println("=== PAIRED APPLICATIONS & CONNECTORS ===")
		now := time.Now()
		for _, a := range list {
			// A code is a credential only while it can still be claimed; show no other.
			code := "-"
			if a.Status == "pending" && now.Before(a.ExpiresAt) {
				code = a.PairingCode
			}
			fmt.Printf("- [%s] %s (%s) — Code: %s, Status: %s\n", a.ID, a.AppName, a.ServiceName, code, a.Status)
		}

	case "claim":
		fs := flag.NewFlagSet("pair claim", flag.ExitOnError)
		serverURL := fs.String("server", "http://localhost:8080", "KyRecovery server URL")
		code := fs.String("code", "", "6-digit pairing code")
		appName := fs.String("app", "Ky Suite client", "Client application name")
		fs.Parse(args[1:])

		if *code == "" {
			fmt.Fprintln(os.Stderr, "Error: --code is required")
			os.Exit(1)
		}

		_, resp, err := client.ClaimPairing(context.Background(), *serverURL, *code, *appName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Claiming failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("=== PAIRING SUCCESSFUL ===")
		fmt.Printf("App Name:     %s\n", resp.AppName)
		fmt.Printf("Service:      %s\n", resp.ServiceName)
		fmt.Printf("API Token:    %s\n", resp.APIToken)
		fmt.Println("\nSave this API Token in your service configuration to deposit sealed capsules.")

	default:
		fmt.Fprintf(os.Stderr, "Unknown pairing sub-action %q (valid: generate, list, claim)\n", action)
		os.Exit(1)
	}
}

// initialAdminPasswordFile holds a generated bootstrap password, never the log.
const initialAdminPasswordFile = "initial-admin-password"

// bootstrapAdmin creates the local "admin" account when it does not exist. Its password
// is supplied, or one generated into <dataDir>/initial-admin-password (0600) before the
// account is written; a file left by a start that died first is reused. If that file
// cannot be written, startup fails rather than put the password anywhere else.
func bootstrapAdmin(ctx context.Context, database *db.DB, mgr *auth.Manager, dataDir, supplied string, w io.Writer) error {
	existing, err := database.GetUserByUsername(ctx, "admin")
	if err != nil {
		return fmt.Errorf("look up admin: %w", err)
	}
	if existing != nil {
		return nil
	}
	pass, path := supplied, ""
	if pass == "" {
		path = filepath.Join(dataDir, initialAdminPasswordFile)
		if pass, err = initialAdminPassword(path); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	if _, _, err := mgr.EnsureAdminUser(ctx, pass); err != nil {
		return err
	}
	announceAdmin(w, path)
	return nil
}

// initialAdminPassword creates path exclusively with a fresh password, or returns the
// one already in it.
func initialAdminPassword(path string) (string, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		raw, err := os.ReadFile(path)
		if pw := strings.TrimSpace(string(raw)); err == nil && pw != "" {
			return pw, nil
		}
		return "", errors.New("file exists but holds no password; remove it and restart")
	}
	if err != nil {
		return "", err
	}
	pw, err := auth.GenerateRandomPassword(16)
	if err == nil {
		_, err = f.WriteString(pw + "\n")
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
		return "", err
	}
	return pw, nil
}

// announceAdmin reports first-start credentials. It never prints a password: a supplied
// one is already known to the operator, a generated one is named by its file (path).
func announceAdmin(w io.Writer, path string) {
	fmt.Fprintln(w, "================================================================================")
	fmt.Fprintln(w, "🚀 KyRecovery Server First-Time Startup Initialized")
	if path == "" {
		fmt.Fprintln(w, "   Local administrator 'admin' created with KY_ADMIN_INITIAL_PASSWORD.")
	} else {
		fmt.Fprintln(w, "   Local administrator 'admin' created. Its one-time password is in")
		fmt.Fprintf(w, "     %s\n", path)
		fmt.Fprintln(w, "   Sign in, change it, then delete that file.")
	}
	fmt.Fprintln(w, "================================================================================")
}

// defaultDataDir is KYRECOVERY_DATA_DIR, else ./data, so every subcommand opens the
// store serve and apply-setup use.
func defaultDataDir() string {
	if dir := os.Getenv("KYRECOVERY_DATA_DIR"); dir != "" {
		return dir
	}
	return "./data"
}
