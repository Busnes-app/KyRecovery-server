package client_test

import (
	"context"
	"encoding/base64"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Busnes-app/ky-primitives/recoverykey"
	"github.com/Busnes-app/kyrecovery-server/internal/audit"
	"github.com/Busnes-app/kyrecovery-server/internal/auth"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
	"github.com/Busnes-app/kyrecovery-server/internal/pairing"
	"github.com/Busnes-app/kyrecovery-server/internal/server"
	"github.com/Busnes-app/kyrecovery-server/pkg/client"
)

func TestClientClaimsPairingCode(t *testing.T) {
	ctx := context.Background()

	// 1. Start test server
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer database.Close()

	ledger := audit.NewLedger(database)
	srv, err := server.New(server.Config{
		Port:         0,
		DataDir:      t.TempDir(),
		DatabasePath: ":memory:",
		Auth:         auth.OIDCConfig{Enabled: false},
	}, database, ledger)
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	ts := httptest.NewServer(srv)
	defer ts.Close()

	// A product can only pair once the ceremony has pinned a recovery key.
	recKey, err := recoverykey.Generate()
	if err != nil {
		t.Fatalf("recoverykey.Generate failed: %v", err)
	}
	pub := recKey.Public()
	if err := database.InsertRecoveryKey(ctx, db.RecoveryKeyRecord{
		KeyID: pub.ID(), PublicKey: pub.Bytes(), Threshold: 3, TotalShares: 5,
		ImportedBy: "test", ImportedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("InsertRecoveryKey failed: %v", err)
	}

	// 2. Generate a pairing code on server
	pairedRecord, err := pairing.GeneratePairingCode(ctx, database, 15*time.Minute, "kybookmarks", "Pending App")
	if err != nil {
		t.Fatalf("GeneratePairingCode failed: %v", err)
	}

	// 3. Client claims pairing code using SDK
	_, claimResp, err := client.ClaimPairing(ctx, ts.URL, pairedRecord.PairingCode, "KyBookmarks Cluster Primary")
	if err != nil {
		t.Fatalf("ClaimPairing failed: %v", err)
	}
	if claimResp.APIToken == "" || claimResp.Status != "paired" || claimResp.ServiceName != "kybookmarks" {
		t.Fatalf("unexpected claim response: %+v", claimResp)
	}
	if claimResp.RecoveryPublicKey != base64.StdEncoding.EncodeToString(pub.Bytes()) {
		t.Fatalf("claim did not hand out the pinned key: %q", claimResp.RecoveryPublicKey)
	}

	// A product whose capsules name a service pairs for that service, so its deposits
	// are not refused as belonging to "generic".
	second, err := pairing.GeneratePairingCode(ctx, database, 15*time.Minute, "kynotes", "Pending App")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := client.ClaimPairingForService(ctx, ts.URL, second.PairingCode, "KyPost", "kypost"); err == nil || !strings.Contains(err.Error(), "HTTP 409") {
		t.Fatalf("mismatched service: %v", err)
	}
	_, named, err := client.ClaimPairingForService(ctx, ts.URL, second.PairingCode, "KyNotes", "kynotes")
	if err != nil || named.ServiceName != "kynotes" {
		t.Fatalf("ClaimPairingForService: %+v %v", named, err)
	}

}
