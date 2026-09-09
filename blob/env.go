package blob

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// FromEnv picks the store from the environment, which is what makes the same
// binary run on a laptop and on a machine with a bucket.
//
//	TRILHA_BLOB_URL unset          → ./data/blob on disk
//	TRILHA_BLOB_URL=file:///var/x  → that directory
//	TRILHA_BLOB_URL=s3://bucket?region=sa-east-1&endpoint=https://minio.local&path_style=1
//
// The credentials come from AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY and
// AWS_SESSION_TOKEN, which is where every other tool already looks for them —
// inventing a third pair of variable names would mean one more thing to set
// and one more thing to leak.
//
// A URL it cannot read is a panic at boot and not a store that silently writes
// to a directory nobody meant: an application that starts with the wrong
// storage is an application that loses files quietly.
func FromEnv() Store {
	raw := strings.TrimSpace(os.Getenv("TRILHA_BLOB_URL"))
	if raw == "" {
		return NewDisk("data/blob")
	}
	u, err := url.Parse(raw)
	if err != nil {
		panic("blob: TRILHA_BLOB_URL is not a URL: " + err.Error())
	}
	switch u.Scheme {
	case "file":
		dir := u.Path
		if u.Host != "" { // file://relative/path
			dir = u.Host + u.Path
		}
		if dir == "" {
			panic("blob: TRILHA_BLOB_URL has no directory")
		}
		return NewDisk(dir)
	case "s3":
		q := u.Query()
		s := &S3{
			Bucket:    u.Host,
			Region:    q.Get("region"),
			Endpoint:  q.Get("endpoint"),
			PathStyle: q.Get("path_style") == "1" || q.Get("path_style") == "true",
			Key:       os.Getenv("AWS_ACCESS_KEY_ID"),
			Secret:    os.Getenv("AWS_SECRET_ACCESS_KEY"),
			Session:   os.Getenv("AWS_SESSION_TOKEN"),
		}
		if s.Bucket == "" {
			panic("blob: s3:// needs a bucket")
		}
		if s.Region == "" {
			s.Region = "us-east-1"
		}
		if s.Key == "" || s.Secret == "" {
			panic("blob: s3:// needs AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY")
		}
		// An endpoint that is not AWS almost always wants path style, and
		// getting it wrong is a DNS error nobody connects to this setting.
		if s.Endpoint != "" && !q.Has("path_style") {
			s.PathStyle = true
		}
		return s
	}
	panic(fmt.Sprintf("blob: TRILHA_BLOB_URL scheme %q is not one this package knows (file, s3)", u.Scheme))
}
