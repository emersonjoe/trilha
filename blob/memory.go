package blob

import (
	"bytes"
	"context"
	"io"
	"sort"
	"sync"
	"time"
)

// Memory keeps the files in the process. It is what a test wants: no
// directory to clean up, no order between tests, and the same behaviour as
// the real thing for everything the application does with it.
type Memory struct {
	mu sync.RWMutex
	m  map[string]memObj
}

type memObj struct {
	data []byte
	typ  string
	at   time.Time
}

// NewMemory builds one.
func NewMemory() *Memory { return &Memory{m: map[string]memObj{}} }

func (s *Memory) Put(ctx context.Context, key string, r io.Reader, size int64, ctype string) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = memObj{data: data, typ: ctype, at: time.Now()}
	return nil
}

// Get answers a reader that is also a seeker, so Serve behaves here the way it
// behaves on disk — a test that passes against Memory and fails against Disk
// because of Range would be a test that lied.
func (s *Memory) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.m[key]
	if !ok {
		return nil, ErrNotFound
	}
	return readSeekCloser{bytes.NewReader(o.data)}, nil
}

func (s *Memory) Stat(ctx context.Context, key string) (Info, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.m[key]
	if !ok {
		return Info{}, ErrNotFound
	}
	return Info{Key: key, Size: int64(len(o.data)), Type: o.typ, ModTime: o.at}, nil
}

func (s *Memory) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[key]; !ok {
		return ErrNotFound
	}
	delete(s.m, key)
	return nil
}

func (s *Memory) Keys(ctx context.Context, fn func(string) error) error {
	s.mu.RLock()
	keys := make([]string, 0, len(s.m))
	for k := range s.m {
		keys = append(keys, k)
	}
	s.mu.RUnlock()
	// In order, so a sweep reports the same list twice.
	sort.Strings(keys)
	for _, k := range keys {
		if err := fn(k); err != nil {
			return err
		}
	}
	return nil
}

type readSeekCloser struct{ *bytes.Reader }

func (readSeekCloser) Close() error { return nil }
