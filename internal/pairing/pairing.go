package pairing

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/Busnes-app/kyrecovery-server/internal/db"
)

// GeneratePairingCode creates a new ephemeral pairing code for connecting a product. The
// returned record is the only place the code exists in plaintext: the store keeps its hash.
func GeneratePairingCode(ctx context.Context, database *db.DB, ttl time.Duration, serviceName, appName string) (*db.PairedAppRecord, error) {
	if ttl <= 0 {
		ttl = 15 * time.Minute // 15-minute default pairing window
	}
	if serviceName == "" {
		serviceName = "auto-declare"
	}
	if appName == "" {
		appName = "Pending Service"
	}

	// Generate 6-digit pairing code (e.g. 748291) or alphanumeric PAIR-XXXX
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return nil, fmt.Errorf("failed generating random pairing code: %w", err)
	}
	pairingCode := fmt.Sprintf("%06d", n.Int64()+100000)

	// No API token yet: ClaimPairingCode mints it and hands it to the claiming product only.
	now := time.Now().UTC()
	record := db.PairedAppRecord{
		ID:          fmt.Sprintf("pair-%d", now.UnixNano()),
		ServiceName: serviceName,
		AppName:     appName,
		PairingCode: pairingCode,
		Status:      "pending",
		ExpiresAt:   now.Add(ttl),
		CreatedAt:   now,
	}

	if err := database.InsertPairedApp(ctx, record); err != nil {
		return nil, fmt.Errorf("failed saving pairing record: %w", err)
	}

	return &record, nil
}
