package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/Busnes-app/ky-primitives/recoverykey"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
)

// recoveryKeyImport is the whole body the ceremony page may send. It has no field for
// shares on purpose; DisallowUnknownFields makes a body carrying one a 400.
type recoveryKeyImport struct {
	PublicKey   string `json:"public_key"`
	Threshold   int    `json:"threshold"`
	TotalShares int    `json:"total_shares"`
}

// recoveryKeyRotation is the import body plus the key it replaces. Naming the current key
// is what stops a ceremony tab left open across another rotation from replacing a key its
// operator never saw.
type recoveryKeyRotation struct {
	recoveryKeyImport
	ReplacesKeyID string `json:"replaces_key_id"`
}

func validTopology(k, n int) bool { return k >= 2 && n >= k && n <= 255 }

// decodeStrict decodes exactly one JSON object with no unknown fields into v.
func decodeStrict(r io.Reader, v any) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// parseRecoveryKey validates a ceremony's public half and topology into a record owned by
// the requesting admin. It writes the 400 itself and returns false on any problem.
func (s *Server) parseRecoveryKey(w http.ResponseWriter, r *http.Request, req recoveryKeyImport) (db.RecoveryKeyRecord, bool) {
	raw, err := base64.StdEncoding.DecodeString(req.PublicKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, "public_key is not standard base64")
		return db.RecoveryKeyRecord{}, false
	}
	pk, err := recoverykey.ParsePublicKey(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("public_key: %v", err))
		return db.RecoveryKeyRecord{}, false
	}
	if !validTopology(req.Threshold, req.TotalShares) {
		writeError(w, http.StatusBadRequest, "threshold and total_shares must satisfy 2 <= threshold <= total_shares <= 255")
		return db.RecoveryKeyRecord{}, false
	}
	return db.RecoveryKeyRecord{
		KeyID: pk.ID(), PublicKey: pk.Bytes(), Threshold: req.Threshold, TotalShares: req.TotalShares,
		Status: "current", ImportedBy: s.actor(r), ImportedAt: time.Now().UTC(),
	}, true
}

func (s *Server) handleRecoveryKey(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rec, err := s.db.GetRecoveryKey(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed reading recovery key")
			return
		}
		if rec == nil {
			writeError(w, http.StatusNotFound, "No recovery key imported; run the ceremony")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"key_id": rec.KeyID, "public_key": base64.StdEncoding.EncodeToString(rec.PublicKey),
			"threshold": rec.Threshold, "total_shares": rec.TotalShares,
			"imported_by": rec.ImportedBy, "imported_at": rec.ImportedAt,
		})
	case http.MethodPost:
		var req recoveryKeyImport
		if err := decodeStrict(r.Body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "Body must be exactly {public_key, threshold, total_shares}")
			return
		}
		rec, ok := s.parseRecoveryKey(w, r, req)
		if !ok {
			return
		}
		if err := s.db.InsertRecoveryKey(r.Context(), rec); err != nil {
			if errors.Is(err, db.ErrRecoveryKeyExists) {
				existing, _ := s.db.GetRecoveryKey(r.Context())
				id := ""
				if existing != nil {
					id = existing.KeyID
				}
				writeError(w, http.StatusConflict, fmt.Sprintf("recovery key %s is already imported; a new key replaces it only through POST /api/recovery-key/rotate", id))
				return
			}
			writeError(w, http.StatusInternalServerError, "Failed storing recovery key")
			return
		}
		_, _ = s.ledger.Record(r.Context(), "recovery_key_imported", s.actor(r), rec.KeyID, map[string]interface{}{
			"threshold": rec.Threshold, "total_shares": rec.TotalShares,
		})
		writeJSON(w, http.StatusCreated, map[string]any{"key_id": rec.KeyID, "threshold": rec.Threshold, "total_shares": rec.TotalShares})
	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// keyProduct is a paired product as the key screens show it: enough to find it, no token.
type keyProduct struct {
	ID          string `json:"id"`
	ServiceName string `json:"service_name"`
	AppName     string `json:"app_name"`
}

// keyCapsule is a capsule that still needs a key's cards to open.
type keyCapsule struct {
	ID          string    `json:"id"`
	ServiceName string    `json:"service_name"`
	AppName     string    `json:"app_name"`
	DepositedAt time.Time `json:"deposited_at"`
}

// keyDependents lists, per key ID, the retained capsules sealed to it and the paired
// products still pinned to it from their pairing.
func (s *Server) keyDependents(r *http.Request) (map[string][]keyCapsule, map[string][]keyProduct, error) {
	caps, err := s.db.ListCapsules(r.Context())
	if err != nil {
		return nil, nil, err
	}
	apps, err := s.db.ListPairedApps(r.Context())
	if err != nil {
		return nil, nil, err
	}
	byKey := map[string][]keyCapsule{}
	for _, c := range caps {
		byKey[c.RecoveryKeyID] = append(byKey[c.RecoveryKeyID], keyCapsule{c.ID, c.ServiceName, c.AppName, c.DepositedAt})
	}
	pinned := map[string][]keyProduct{}
	for _, a := range apps {
		if a.Status == "paired" {
			pinned[a.RecoveryKeyID] = append(pinned[a.RecoveryKeyID], keyProduct{a.ID, a.ServiceName, a.AppName})
		}
	}
	return byKey, pinned, nil
}

// handleRecoveryKeys lists every key the store has held, with what still depends on each.
func (s *Server) handleRecoveryKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	keys, err := s.db.ListRecoveryKeys(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed reading recovery keys")
		return
	}
	byKey, pinned, err := s.keyDependents(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed reading key dependents")
		return
	}
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		products := pinned[k.KeyID]
		if products == nil {
			products = []keyProduct{}
		}
		out = append(out, map[string]any{
			"key_id": k.KeyID, "threshold": k.Threshold, "total_shares": k.TotalShares, "status": k.Status,
			"imported_by": k.ImportedBy, "imported_at": k.ImportedAt, "retired_by": k.RetiredBy, "retired_at": k.RetiredAt,
			"capsule_count": len(byKey[k.KeyID]), "pinned_products": products,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleRecoveryKeyRotate makes a new ceremony's key current. The old key stays as
// "previous": its capsules still need its cards, and the server cannot re-seal them.
func (s *Server) handleRecoveryKeyRotate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req recoveryKeyRotation
	if err := decodeStrict(r.Body, &req); err != nil || req.ReplacesKeyID == "" {
		writeError(w, http.StatusBadRequest, "Body must be exactly {public_key, threshold, total_shares, replaces_key_id}")
		return
	}
	rec, ok := s.parseRecoveryKey(w, r, req.recoveryKeyImport)
	if !ok {
		return
	}
	current, err := s.db.GetRecoveryKey(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed reading recovery key")
		return
	}
	if current == nil {
		writeError(w, http.StatusConflict, "No recovery key imported; there is nothing to rotate, run the ceremony")
		return
	}
	if current.KeyID != req.ReplacesKeyID {
		writeError(w, http.StatusConflict, fmt.Sprintf("replaces_key_id is not the current key; the current key is %s", current.KeyID))
		return
	}
	if known, err := s.db.GetRecoveryKeyByID(r.Context(), rec.KeyID); err != nil || known != nil {
		writeError(w, http.StatusConflict, "This key is already known to the store; run a new ceremony")
		return
	}
	details := map[string]any{"replaces_key_id": current.KeyID, "threshold": rec.Threshold, "total_shares": rec.TotalShares}
	if !s.auditFirst(w, r, "recovery_key_rotation_requested", rec.KeyID, details) {
		return
	}
	if err := s.db.RotateRecoveryKey(r.Context(), current.KeyID, rec); err != nil {
		switch {
		case errors.Is(err, db.ErrStaleRotation):
			writeError(w, http.StatusConflict, "The current key changed during the request; reload and check it")
		case errors.Is(err, db.ErrRecoveryKeyKnown):
			writeError(w, http.StatusConflict, "This key is already known to the store; run a new ceremony")
		default:
			writeError(w, http.StatusInternalServerError, "Failed storing recovery key")
		}
		return
	}
	if _, err := s.ledger.Record(r.Context(), "recovery_key_rotated", s.actor(r), rec.KeyID, details); err != nil {
		log.Printf("recovery key rotation: recording completion: %v", err)
	}
	_, pinned, err := s.keyDependents(r)
	if err != nil {
		pinned = map[string][]keyProduct{}
	}
	repair := pinned[current.KeyID]
	if repair == nil {
		repair = []keyProduct{}
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"key_id": rec.KeyID, "replaces_key_id": current.KeyID, "threshold": rec.Threshold, "total_shares": rec.TotalShares,
		"products_to_repair": repair,
	})
}

// handleRecoveryKeyRetire records that a previous key's cards are no longer needed. It is
// refused while any retained capsule is sealed to the key, naming them.
func (s *Server) handleRecoveryKeyRetire(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req struct {
		KeyID string `json:"key_id"`
	}
	if err := decodeStrict(r.Body, &req); err != nil || req.KeyID == "" {
		writeError(w, http.StatusBadRequest, "Body must be exactly {key_id}")
		return
	}
	key, err := s.db.GetRecoveryKeyByID(r.Context(), req.KeyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed reading recovery key")
		return
	}
	switch {
	case key == nil:
		writeError(w, http.StatusNotFound, "No such recovery key")
		return
	case key.Status == "current":
		writeError(w, http.StatusConflict, "The current key cannot be retired; rotate to a new key first")
		return
	case key.Status == "retired":
		writeError(w, http.StatusConflict, "This key is already retired")
		return
	}
	byKey, pinned, err := s.keyDependents(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed reading key dependents")
		return
	}
	if deps := byKey[key.KeyID]; len(deps) > 0 {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":    fmt.Sprintf("%d retained capsule(s) are sealed to this key and open only with its cards; keep the cards until they expire under retention or are purged", len(deps)),
			"capsules": deps,
		})
		return
	}
	stale := pinned[key.KeyID]
	if stale == nil {
		stale = []keyProduct{}
	}
	details := map[string]any{"pinned_products": len(stale)}
	if !s.auditFirst(w, r, "recovery_key_retirement_requested", key.KeyID, details) {
		return
	}
	if err := s.db.RetireRecoveryKey(r.Context(), key.KeyID, s.actor(r), time.Now()); err != nil {
		if errors.Is(err, db.ErrRetireRefused) {
			writeError(w, http.StatusConflict, "The key gained a dependent or changed state during the request; reload and check it")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed retiring recovery key")
		return
	}
	if _, err := s.ledger.Record(r.Context(), "recovery_key_retired", s.actor(r), key.KeyID, details); err != nil {
		log.Printf("recovery key retirement: recording completion: %v", err)
	}
	// A product still pinned here cannot deposit (only the current key is accepted) and was
	// not before this call either; it is named so the operator re-pairs it.
	writeJSON(w, http.StatusOK, map[string]any{"key_id": key.KeyID, "status": "retired", "pinned_products": stale})
}
