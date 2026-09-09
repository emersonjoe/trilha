package blob

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// S3 talks to an S3-compatible bucket — AWS, MinIO, Garage, R2 — with the
// signature written here instead of an SDK. It is about two hundred lines,
// most of them the canonical request, and it is the difference between this
// module being optional and the framework growing a dependency tree.
//
// What it does not do is everything S3 does. It puts, gets, heads, deletes,
// lists and presigns, which is what a file store needs; anything past that is
// a reason to use the SDK in your own code, not a reason for this to grow.
type S3 struct {
	Bucket   string
	Region   string
	Endpoint string // empty is AWS: https://<bucket>.s3.<region>.amazonaws.com
	Key      string
	Secret   string
	Session  string // STS, when there is one

	// PathStyle addresses the bucket as /bucket/key instead of as a
	// subdomain. MinIO and most self-hosted servers want it; AWS does not.
	PathStyle bool

	// HTTP is the client. Nil is http.DefaultClient with a timeout, because a
	// storage call with no deadline is a handler that hangs.
	HTTP *http.Client

	// now exists for the test: a signature is a function of the clock, and a
	// test that cannot fix the clock cannot check a signature.
	now func() time.Time
}

func (s *S3) client() *http.Client {
	if s.HTTP != nil {
		return s.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (s *S3) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now().UTC()
}

// endpoint builds the URL of one key.
func (s *S3) url(key string) string {
	host := s.Endpoint
	if host == "" {
		return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.Bucket, s.Region, key)
	}
	host = strings.TrimSuffix(host, "/")
	if s.PathStyle {
		return host + "/" + s.Bucket + "/" + key
	}
	// A non-AWS endpoint addressed by subdomain: scheme first, then the
	// bucket in front of the host.
	if i := strings.Index(host, "://"); i > 0 {
		return host[:i+3] + s.Bucket + "." + host[i+3:] + "/" + key
	}
	return host + "/" + key
}

func (s *S3) Put(ctx context.Context, key string, r io.Reader, size int64, ctype string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, s.url(key), r)
	if err != nil {
		return err
	}
	req.ContentLength = size
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	// UNSIGNED-PAYLOAD: the body is a stream and hashing it would mean
	// buffering the whole upload to sign it. The transport is TLS and the
	// request is signed; what is not signed is the body's digest, which is
	// what every SDK does for a streaming put as well.
	if err := s.sign(req, "UNSIGNED-PAYLOAD"); err != nil {
		return err
	}
	res, err := s.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return s3Error(res)
}

func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url(key), nil)
	if err != nil {
		return nil, err
	}
	if err := s.sign(req, emptyHash); err != nil {
		return nil, err
	}
	res, err := s.client().Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusNotFound {
		res.Body.Close()
		return nil, ErrNotFound
	}
	if err := s3Error(res); err != nil {
		res.Body.Close()
		return nil, err
	}
	return res.Body, nil
}

func (s *S3) Stat(ctx context.Context, key string) (Info, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, s.url(key), nil)
	if err != nil {
		return Info{}, err
	}
	if err := s.sign(req, emptyHash); err != nil {
		return Info{}, err
	}
	res, err := s.client().Do(req)
	if err != nil {
		return Info{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return Info{}, ErrNotFound
	}
	if err := s3Error(res); err != nil {
		return Info{}, err
	}
	mod, _ := http.ParseTime(res.Header.Get("Last-Modified"))
	return Info{Key: key, Size: res.ContentLength, Type: res.Header.Get("Content-Type"), ModTime: mod}, nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.url(key), nil)
	if err != nil {
		return err
	}
	if err := s.sign(req, emptyHash); err != nil {
		return err
	}
	res, err := s.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	return s3Error(res)
}

// listResult is the shape of a ListObjectsV2 answer, in the parts this uses.
type listResult struct {
	Contents []struct {
		Key string `xml:"Key"`
	} `xml:"Contents"`
	IsTruncated           bool   `xml:"IsTruncated"`
	NextContinuationToken string `xml:"NextContinuationToken"`
}

func (s *S3) Keys(ctx context.Context, fn func(string) error) error {
	token := ""
	for {
		q := url.Values{"list-type": {"2"}, "max-keys": {"1000"}}
		if token != "" {
			q.Set("continuation-token", token)
		}
		base := s.url("")
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(base, "/")+"/?"+q.Encode(), nil)
		if err != nil {
			return err
		}
		if err := s.sign(req, emptyHash); err != nil {
			return err
		}
		res, err := s.client().Do(req)
		if err != nil {
			return err
		}
		if err := s3Error(res); err != nil {
			res.Body.Close()
			return err
		}
		var out listResult
		err = xml.NewDecoder(res.Body).Decode(&out)
		res.Body.Close()
		if err != nil {
			return err
		}
		for _, o := range out.Contents {
			if err := fn(o.Key); err != nil {
				return err
			}
		}
		// A bucket does not fit in one answer, and a sweep that stops at the
		// first thousand keys reports every other object as an orphan.
		if !out.IsTruncated || out.NextContinuationToken == "" {
			return nil
		}
		token = out.NextContinuationToken
	}
}

// Presign hands out a URL the browser fetches directly, valid for ttl. The
// bytes never pass through the application, which is the point on a large
// file and behind a CDN.
//
// It is a capability: whoever has the URL has the file until it expires, with
// no session and no log of yours. Ctx-side, blob.ServeOpts{Proxy: true} is the
// answer when that is not acceptable.
func (s *S3) Presign(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if ttl <= 0 || ttl > 7*24*time.Hour {
		return "", fmt.Errorf("blob: a presigned URL lasts between a second and seven days, got %s", ttl)
	}
	now := s.clock().UTC()
	scope := now.Format("20060102") + "/" + s.Region + "/s3/aws4_request"
	q := url.Values{
		"X-Amz-Algorithm":     {"AWS4-HMAC-SHA256"},
		"X-Amz-Credential":    {s.Key + "/" + scope},
		"X-Amz-Date":          {now.Format("20060102T150405Z")},
		"X-Amz-Expires":       {fmt.Sprint(int(ttl.Seconds()))},
		"X-Amz-SignedHeaders": {"host"},
	}
	if s.Session != "" {
		q.Set("X-Amz-Security-Token", s.Session)
	}
	u, err := url.Parse(s.url(key))
	if err != nil {
		return "", err
	}
	canonical := strings.Join([]string{
		http.MethodGet,
		u.EscapedPath(),
		q.Encode(),
		"host:" + u.Host + "\n",
		"host",
		"UNSIGNED-PAYLOAD",
	}, "\n")
	sig := s.signature(now, scope, canonical)
	q.Set("X-Amz-Signature", sig)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

const emptyHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// sign adds the SigV4 headers to a request. The canonical request is built in
// the order the specification gives, and the order is the whole of it: every
// signature bug is a header sorted differently or a path escaped twice.
func (s *S3) sign(req *http.Request, payloadHash string) error {
	if s.Key == "" || s.Secret == "" {
		return fmt.Errorf("blob: S3 needs a key and a secret")
	}
	now := s.clock().UTC()
	amzDate := now.Format("20060102T150405Z")
	scope := now.Format("20060102") + "/" + s.Region + "/s3/aws4_request"

	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	if s.Session != "" {
		req.Header.Set("X-Amz-Security-Token", s.Session)
	}
	if req.Host == "" {
		req.Host = req.URL.Host
	}

	names, canonicalHeaders := canonicalHeaders(req)
	canonical := strings.Join([]string{
		req.Method,
		req.URL.EscapedPath(),
		req.URL.Query().Encode(),
		canonicalHeaders,
		names,
		payloadHash,
	}, "\n")
	sig := s.signature(now, scope, canonical)
	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", s.Key, scope, names, sig))
	return nil
}

// signature is the last three steps: hash the canonical request, build the
// string to sign, and walk the key down the scope.
func (s *S3) signature(now time.Time, scope, canonical string) string {
	sum := sha256.Sum256([]byte(canonical))
	toSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		now.Format("20060102T150405Z"),
		scope,
		hex.EncodeToString(sum[:]),
	}, "\n")
	k := hmacSHA256([]byte("AWS4"+s.Secret), now.Format("20060102"))
	k = hmacSHA256(k, s.Region)
	k = hmacSHA256(k, "s3")
	k = hmacSHA256(k, "aws4_request")
	return hex.EncodeToString(hmacSHA256(k, toSign))
}

// canonicalHeaders is host plus every x-amz-*, lower-cased, sorted, with the
// values trimmed. Anything else that happens to be on the request is left out
// on purpose: a header the client adds later would break a signature that
// promised it.
func canonicalHeaders(req *http.Request) (names, canonical string) {
	h := map[string]string{"host": req.Host}
	for k, v := range req.Header {
		lk := strings.ToLower(k)
		if lk == "host" || strings.HasPrefix(lk, "x-amz-") || lk == "content-type" {
			h[lk] = strings.TrimSpace(strings.Join(v, ","))
		}
	}
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte(':')
		b.WriteString(h[k])
		b.WriteByte('\n')
	}
	return strings.Join(keys, ";"), b.String()
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}

// s3Error turns a bad status into an error that says what the server said. An
// S3 error body is XML with a Code and a Message, and reading them is the
// difference between "403" and "the clock of this machine is nine minutes off".
func s3Error(res *http.Response) error {
	if res.StatusCode < 400 {
		return nil
	}
	var body struct {
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	}
	_ = xml.NewDecoder(io.LimitReader(res.Body, 8<<10)).Decode(&body)
	if body.Code != "" {
		return fmt.Errorf("blob: s3 %s: %s (%s)", res.Status, body.Code, body.Message)
	}
	return fmt.Errorf("blob: s3 %s", res.Status)
}
