package secrets_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/Busnes-app/kyrecovery-server/internal/secrets"
)

// FuzzKeyringOpen covers both directions of the stored-credential envelope. Arbitrary stored
// text never opens to a credential (only "" opens, to ""), and a failed Open returns no
// plaintext. A credential Seal encrypts opens to itself, and the same envelope with one bit
// of nonce or ciphertext flipped does not open.
func FuzzKeyringOpen(f *testing.F) {
	for _, s := range []string{
		"s3-secret", "enc:v0:abc", "enc:v1", " enc:v1:abc", "",
		// Edge cases: a well-formed but short envelope, padding games, a sealed-looking credential.
		"enc:v1:", "enc:v1:AAAAAAAAAAAAAAAA", "enc:v1:" + strings.Repeat("A", 40), "enc:v1:=\n", "\x00",
	} {
		f.Add(s, uint16(0))
	}
	k, err := secrets.Ephemeral()
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, s string, flip uint16) {
		if got, err := k.Open(s); s == "" {
			if err != nil || got != "" {
				t.Fatalf(`Open("") = %q, %v`, got, err)
			}
		} else if err == nil {
			t.Fatalf("Open(%q) = %q: a value this keyring never sealed opened", s, got)
		} else if got != "" {
			t.Fatalf("Open(%q) failed (%v) but returned %q", s, err, got)
		}

		sealed, err := k.Seal(s)
		if err != nil {
			t.Fatal(err)
		}
		if s == "" || secrets.IsSealed(s) {
			// Documented pass-through: empty stays empty, an envelope is not sealed twice.
			if sealed != s {
				t.Fatalf("Seal(%q) = %q, want it unchanged", s, sealed)
			}
			return
		}
		if !secrets.IsSealed(sealed) || (len(s) >= 8 && strings.Contains(sealed, s)) {
			t.Fatalf("Seal(%q) = %q is not an opaque envelope", s, sealed)
		}
		if got, err := k.Open(sealed); err != nil || got != s {
			t.Fatalf("Open(Seal(%q)) = %q, %v", s, got, err)
		}
		blob, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(sealed, "enc:v1:"))
		if err != nil {
			t.Fatal(err)
		}
		blob[int(flip/8)%len(blob)] ^= 1 << (flip % 8)
		tampered := "enc:v1:" + base64.StdEncoding.EncodeToString(blob)
		if got, err := k.Open(tampered); err == nil {
			t.Fatalf("envelope with bit %d flipped opened to %q", flip, got)
		}
	})
}
