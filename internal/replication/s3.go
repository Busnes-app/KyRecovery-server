package replication

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

// S3Client provides AWS SigV4 signed operations for S3 and Cloudflare R2.
type S3Client struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	client    *http.Client
	now       func() time.Time
	// The signer caches derived keys by access key ID, so it is never shared between
	// clients holding different secrets.
	signer *v4.Signer
}

// NewS3Client creates a new S3 client.
func NewS3Client(endpoint, bucket, region, accessKey, secretKey string) *S3Client {
	if region == "" {
		region = "us-east-1"
	}
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.s3.%s.amazonaws.com", bucket, region)
	}
	return &S3Client{
		Endpoint:  strings.TrimRight(endpoint, "/"),
		Bucket:    bucket,
		Region:    region,
		AccessKey: accessKey,
		SecretKey: secretKey,
		client:    &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }},
		now:       time.Now,
		// S3 object keys are escaped once, never twice as other SigV4 services are.
		signer: v4.NewSigner(func(o *v4.SignerOptions) { o.DisableURIPathEscaping = true }),
	}
}

// PutObject uploads a stream/file to the target bucket with SigV4 signing.
func (c *S3Client) PutObject(ctx context.Context, key string, data io.Reader, size int64, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// SigV4 signs a hash of the body, so the body has to be read twice. A capsule
	// is a file, so hash one pass and rewind rather than holding the whole archive
	// in memory; anything not seekable falls back to buffering.
	var (
		body           io.Reader
		payloadHashHex string
	)
	if seeker, ok := data.(io.ReadSeeker); ok {
		h := sha256.New()
		n, err := io.Copy(h, seeker)
		if err != nil {
			return fmt.Errorf("failed hashing payload for upload: %w", err)
		}
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed rewinding payload for upload: %w", err)
		}
		payloadHashHex = hex.EncodeToString(h.Sum(nil))
		size, body = n, seeker
	} else {
		bodyBytes, err := io.ReadAll(data)
		if err != nil {
			return fmt.Errorf("failed reading payload for upload: %w", err)
		}
		sum := sha256.Sum256(bodyBytes)
		payloadHashHex = hex.EncodeToString(sum[:])
		size, body = int64(len(bodyBytes)), bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.buildURL(key), nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-amz-content-sha256", payloadHashHex)
	// Signed before Content-Type and the body are attached, so neither is in the
	// signature, as before.
	creds := aws.Credentials{AccessKeyID: c.AccessKey, SecretAccessKey: c.SecretKey}
	if err := c.signer.SignHTTP(ctx, creds, req, payloadHashHex, "s3", c.Region, c.now()); err != nil {
		return fmt.Errorf("failed signing S3 request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Body, req.ContentLength = io.NopCloser(body), size

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("S3 PUT request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("S3 upload returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// TestConnection verifies bucket accessibility by executing a signed HEAD or PUT ping.
func (c *S3Client) TestConnection(ctx context.Context) error {
	pingData := []byte("kyrecovery-ping-" + time.Now().UTC().Format(time.RFC3339))
	return c.PutObject(ctx, ".kyrecovery-ping", bytes.NewReader(pingData), int64(len(pingData)), "text/plain")
}

func (c *S3Client) buildURL(key string) string {
	key = strings.TrimPrefix(key, "/")
	if strings.Contains(c.Endpoint, c.Bucket) {
		return fmt.Sprintf("%s/%s", c.Endpoint, key)
	}
	return fmt.Sprintf("%s/%s/%s", c.Endpoint, c.Bucket, key)
}
