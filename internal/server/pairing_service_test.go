package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Busnes-app/ky-primitives/recoverykey"
)

func TestClaimServiceMismatchIs409AndCodeCanBeRetried(t *testing.T) {
	srv, cookie, _ := newAdminServer(t)
	k, _ := recoverykey.Generate()
	importKey(t, srv, cookie, k.Public(), 3, 5)
	req := httptest.NewRequest(http.MethodPost, "/api/pairing/generate", bytes.NewBufferString(`{"service_name":"kynotes"}`))
	req.AddCookie(cookie)
	generated := httptest.NewRecorder()
	srv.ServeHTTP(generated, req)
	var gen map[string]any
	if err := json.Unmarshal(generated.Body.Bytes(), &gen); err != nil || generated.Code != 200 {
		t.Fatalf("generate: %d %s", generated.Code, generated.Body.String())
	}
	claim := func(service string) *httptest.ResponseRecorder {
		payload := map[string]any{"pairing_code": gen["pairing_code"]}
		if service != "" {
			payload["service_name"] = service
		}
		body, _ := json.Marshal(payload)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/pairing/claim", bytes.NewReader(body)))
		return rr
	}
	if rr := claim("kypost"); rr.Code != http.StatusConflict {
		t.Fatalf("mismatch: %d %s", rr.Code, rr.Body.String())
	}
	rr := claim("")
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if rr.Code != http.StatusOK || got["service_name"] != "kynotes" {
		t.Fatalf("omitted service retry: %d %s", rr.Code, rr.Body.String())
	}
}
