package replication

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Authorization headers produced by the hand-written SigV4 signer this replaced. Any
// change here makes every virtual-host S3 destination refuse uploads.
func TestS3SignatureMatchesPreviousSigner(t *testing.T) {
	const scope = "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/"
	const signed = "/s3/aws4_request, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature="
	for _, c := range []struct {
		endpoint, bucket, region, key, body, contentType string
		stream                                           bool
		wantURL, wantType, wantAuth                      string
	}{
		{"https://vault.s3.us-west-2.amazonaws.com", "vault", "us-west-2", "capsules/cap-KySignOn-1.kycap", "payload", "application/octet-stream", false,
			"https://vault.s3.us-west-2.amazonaws.com/capsules/cap-KySignOn-1.kycap", "application/octet-stream",
			scope + "us-west-2" + signed + "dd428787c4157c62f5bde077abab5a817af5ded031c6e42172b2078f0e012d17"},
		{"https://acct.r2.cloudflarestorage.com/vault", "vault", "auto", "/dir/ü x~-_.kycap", "payload", "application/octet-stream", true,
			"https://acct.r2.cloudflarestorage.com/vault/dir/%C3%BC%20x~-_.kycap", "application/octet-stream",
			scope + "auto" + signed + "edd2009a8b3e936b6b5ffe8defeb02c275b695d1ec0a47c0cf2bd61e09513380"},
		{"", "my-vault", "", ".kyrecovery-ping", "kyrecovery-ping-2013-05-24T00:00:00Z", "text/plain", false,
			"https://my-vault.s3.us-east-1.amazonaws.com/.kyrecovery-ping", "text/plain",
			scope + "us-east-1" + signed + "8033b83592e66a0350a1daa135ebd01c142dbbe637e3306a08fbfb4a47de5c62"},
		{"http://vault.minio.lan:9000/", "vault", "us-east-1", "empty.kycap", "", "", false,
			"http://vault.minio.lan:9000/empty.kycap", "application/octet-stream",
			scope + "us-east-1" + signed + "bc2840056dbf78be81fea55d0811ae5601544f295861dddf073b67235e3e5fa5"},
	} {
		client := NewS3Client(c.endpoint, c.bucket, c.region, "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
		client.now = func() time.Time { return time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC) }
		client.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(r.Body)
			if r.URL.String() != c.wantURL || r.Header.Get("Content-Type") != c.wantType || string(body) != c.body || r.ContentLength != int64(len(c.body)) {
				t.Errorf("%s: sent %s type %q length %d body %q", c.key, r.URL, r.Header.Get("Content-Type"), r.ContentLength, body)
			}
			if got := r.Header.Get("Authorization"); got != c.wantAuth {
				t.Errorf("%s:\n got %s\nwant %s", c.key, got, c.wantAuth)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
		})
		var data io.Reader = bytes.NewReader([]byte(c.body))
		if c.stream {
			data = io.MultiReader(data)
		}
		if err := client.PutObject(context.Background(), c.key, data, int64(len(c.body)), c.contentType); err != nil {
			t.Fatal(err)
		}
	}
}
