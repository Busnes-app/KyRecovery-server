package server_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Busnes-app/ky-primitives/capsule"
	"github.com/Busnes-app/ky-primitives/hybridsig"
	"github.com/Busnes-app/ky-primitives/recoverykey"
	"github.com/Busnes-app/kyrecovery-server/internal/server"
)

func generatePairingCode(t *testing.T, srv *server.Server, cookie *http.Cookie, service string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"ttl_minutes": 10, "service_name": service, "app_name": service})
	req := httptest.NewRequest(http.MethodPost, "/api/pairing/generate", bytes.NewReader(body))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("generate: %d %s", rec.Code, rec.Body.String())
	}
	var gen struct {
		PairingCode string `json:"pairing_code"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &gen)
	return gen.PairingCode
}

func sealSignedFor(t *testing.T, pub recoverykey.PublicKey, signer *hybridsig.PrivateKey, service string) []byte {
	t.Helper()
	raw, _, err := capsule.SealSigned(service, "1.0.0", []capsule.File{{Path: "data/x.db", Content: []byte("payload"), Mode: 0600}}, nil, nil, 3, 5, pub, signer)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestDepositVerifiesHybridSignedManifestWhenProductPinsSigner(t *testing.T) {
	srv, cookie, database := newAdminServer(t)
	rk, err := recoverykey.Generate()
	if err != nil {
		t.Fatal(err)
	}
	importKey(t, srv, cookie, rk.Public(), 3, 5)

	signer, err := hybridsig.Generate()
	if err != nil {
		t.Fatal(err)
	}
	otherSigner, err := hybridsig.Generate()
	if err != nil {
		t.Fatal(err)
	}

	// Malformed signer_public_key must return 400 and leave the pairing code unconsumed.
	code := generatePairingCode(t, srv, cookie, "kynotes")
	for _, bad := range []string{
		"not-base64!!!",
		base64.StdEncoding.EncodeToString([]byte("wrong-alg\x00short")),
		base64.StdEncoding.EncodeToString(signer.Public().Bytes()) + "\n",
	} {
		body, _ := json.Marshal(map[string]string{
			"pairing_code":      code,
			"service_name":      "kynotes",
			"app_name":          "kynotes",
			"signer_public_key": bad,
		})
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/pairing/claim", bytes.NewReader(body)))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("bad signer_public_key %q: got %d (%s), want 400", bad, rec.Code, rec.Body.String())
		}
	}

	// Claiming the same pairing code with a valid hybridsig public key succeeds and pins signer_key_id.
	pubB64 := base64.StdEncoding.EncodeToString(signer.Public().Bytes())
	claimBody, _ := json.Marshal(map[string]string{
		"pairing_code":      code,
		"service_name":      "kynotes",
		"app_name":          "kynotes",
		"signer_public_key": pubB64,
	})
	claimRec := httptest.NewRecorder()
	srv.ServeHTTP(claimRec, httptest.NewRequest(http.MethodPost, "/api/pairing/claim", bytes.NewReader(claimBody)))
	if claimRec.Code != http.StatusOK {
		t.Fatalf("claim with signer: %d %s", claimRec.Code, claimRec.Body.String())
	}
	var claim map[string]any
	if err := json.Unmarshal(claimRec.Body.Bytes(), &claim); err != nil {
		t.Fatal(err)
	}
	if claim["signer_key_id"] != signer.Public().ID() {
		t.Fatalf("claim signer_key_id = %v, want %q", claim["signer_key_id"], signer.Public().ID())
	}
	token := claim["api_token"].(string)

	// 1. Valid hybrid-signed capsule is accepted and audited as manifest_verified=true.
	signed := sealSignedFor(t, rk.Public(), signer, "kynotes")
	if rec := deposit(srv, token, signed); rec.Code != http.StatusCreated {
		t.Fatalf("signed deposit: got %d (%s), want 201", rec.Code, rec.Body.String())
	}
	events, err := database.ListAuditEvents(t.Context(), 10)
	if err != nil {
		t.Fatal(err)
	}
	foundAudit := false
	for _, ev := range events {
		if ev.Action == "capsule_deposited" {
			if strings.Contains(ev.DetailsJSON, `"manifest_verified":true`) && strings.Contains(ev.DetailsJSON, signer.Public().ID()) {
				foundAudit = true
			}
		}
	}
	if !foundAudit {
		t.Fatalf("expected capsule_deposited audit event with manifest_verified=true and signer_key_id=%s", signer.Public().ID())
	}

	// 2. Unsigned capsule for a signer-pinned product is rejected.
	unsigned := sealFor(t, rk.Public(), "kynotes")
	if rec := deposit(srv, token, unsigned); rec.Code != http.StatusBadRequest {
		t.Fatalf("unsigned deposit for signer-pinned product: got %d (%s), want 400", rec.Code, rec.Body.String())
	}

	// 3. Capsule signed by a different key is rejected.
	wrongKey := sealSignedFor(t, rk.Public(), otherSigner, "kynotes")
	if rec := deposit(srv, token, wrongKey); rec.Code != http.StatusBadRequest {
		t.Fatalf("wrong-signer deposit: got %d (%s), want 400", rec.Code, rec.Body.String())
	}

	// 4. Tampered manifest signature is rejected.
	var container map[string]any
	if err := json.Unmarshal(signed, &container); err != nil {
		t.Fatal(err)
	}
	sigBytes, err := base64.StdEncoding.DecodeString(container["manifest_signature"].(string))
	if err != nil {
		t.Fatal(err)
	}
	sigBytes[len(sigBytes)-1] ^= 0xff
	container["manifest_signature"] = base64.StdEncoding.EncodeToString(sigBytes)
	tampered, err := json.Marshal(container)
	if err != nil {
		t.Fatal(err)
	}
	if rec := deposit(srv, token, tampered); rec.Code != http.StatusBadRequest {
		t.Fatalf("tampered-signature deposit: got %d (%s), want 400", rec.Code, rec.Body.String())
	}

	// 5. Legacy product paired without signer_public_key continues to accept unsigned and signed capsules.
	legacyToken, legacyClaim := pairProduct(t, srv, cookie, "kypassword")
	if _, has := legacyClaim["signer_key_id"]; has {
		t.Fatalf("legacy claim unexpectedly carried signer_key_id: %v", legacyClaim["signer_key_id"])
	}
	if rec := deposit(srv, legacyToken, sealFor(t, rk.Public(), "kypassword")); rec.Code != http.StatusCreated {
		t.Fatalf("legacy unsigned deposit: got %d (%s), want 201", rec.Code, rec.Body.String())
	}
	if rec := deposit(srv, legacyToken, sealSignedFor(t, rk.Public(), signer, "kypassword")); rec.Code != http.StatusCreated {
		t.Fatalf("legacy signed deposit: got %d (%s), want 201", rec.Code, rec.Body.String())
	}
}
