package clientconfig

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	backupSuffix   = ".convergence.bak"
	manifestSuffix = ".convergence.bak.json"
)

type edit struct {
	Path []string        `json:"path"`
	Had  bool            `json:"had"`
	Prev json.RawMessage `json:"prev,omitempty"`
	New  json.RawMessage `json:"new"`
}

type manifest struct {
	Existed bool   `json:"existed"`
	SHA256  string `json:"sha256"`
	// Dirty: the file was edited by someone else between two of our runs, so the
	// backup no longer describes the pre-Convergence state byte for byte.
	Dirty bool   `json:"dirty,omitempty"`
	Edits   []edit `json:"edits,omitempty"`
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func readManifest(path string) (*manifest, error) {
	b, err := os.ReadFile(path + manifestSuffix)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("corrupt manifest %s: %w", path+manifestSuffix, err)
	}
	return &m, nil
}

func writeAtomic(path string, data []byte, defMode os.FileMode) error {
	mode := defMode
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".convergence-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func readOptional(path string) ([]byte, bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	return b, err == nil, err
}

// commit persists the backup + manifest and writes the new content. The
// original backup and first-seen previous values survive repeated runs.
func commit(path string, orig []byte, existed bool, out []byte, edits []edit) error {
	prev, err := readManifest(path)
	if err != nil {
		return err
	}
	m := manifest{Existed: existed, Edits: edits}
	if prev != nil {
		m.Existed = prev.Existed
		m.Dirty = prev.Dirty || prev.SHA256 != sum(orig)
		m.Edits = mergeEdits(prev.Edits, edits)
	} else if err := writeAtomic(path+backupSuffix, orig, 0o600); err != nil {
		return err
	}
	m.SHA256 = sum(out)
	mb, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(path+manifestSuffix, append(mb, '\n'), 0o600); err != nil {
		return err
	}
	return writeAtomic(path, out, 0o644)
}

func mergeEdits(old, cur []edit) []edit {
	out := append([]edit(nil), old...)
	for _, c := range cur {
		found := false
		for i := range out {
			if bytes.Equal(mustJSON(out[i].Path), mustJSON(c.Path)) {
				out[i].New = c.New
				found = true
			}
		}
		if !found {
			out = append(out, c)
		}
	}
	return out
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
