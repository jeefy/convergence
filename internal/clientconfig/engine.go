package clientconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Result struct {
	Path    string
	Changed bool
	Diff    string
	Note    string
}

func (c *client) get(src []byte) (map[string]any, error) {
	if c.format == YAML {
		return yamlGet(src)
	}
	return jsonGet(src)
}

func (c *client) set(src []byte, p []string, v any) ([]byte, error) {
	if c.format == YAML {
		return yamlSet(src, p, v)
	}
	return jsonSet(src, p, v)
}

func (c *client) del(src []byte, p []string) ([]byte, error) {
	if c.format == YAML {
		return yamlDelete(src, p)
	}
	return jsonDelete(src, p)
}

// Configure points the client at Convergence.
func Configure(name string, o Options) (Result, error) {
	c, path, err := find(name)
	if err != nil {
		return Result{}, err
	}
	orig, existed, err := readOptional(path)
	if err != nil {
		return Result{}, err
	}
	doc, err := c.get(orig)
	if err != nil {
		return Result{}, fmt.Errorf("%s: %w", path, err)
	}
	out, edits := orig, []edit(nil)
	for _, op := range c.ops(o, Root(o.GatewayURL)) {
		prev, had := lookup(doc, op.path...)
		if op.onlyIfMissing && had {
			continue
		}
		next, err := c.set(out, op.path, op.val)
		if err != nil {
			return Result{}, fmt.Errorf("%s: %w", path, err)
		}
		out = next
		if had && jsonEqual(prev, op.val) {
			continue
		}
		e := edit{Path: op.path, Had: had, New: mustJSON(op.val)}
		if had {
			e.Prev = mustJSON(prev)
		}
		edits = append(edits, e)
	}
	res := Result{Path: path, Note: c.note, Changed: !bytes.Equal(orig, out)}
	if !res.Changed {
		return res, nil
	}
	res.Diff = lineDiff(path, orig, out)
	if o.DryRun {
		return res, nil
	}
	return res, commit(path, orig, existed, out, edits)
}

// Restore undoes Configure. A file that is byte-identical to what we wrote is
// restored exactly from the backup; a file edited since is refused unless
// Force, in which case only our keys are reverted.
func Restore(name string, o Options) (Result, error) {
	c, path, err := find(name)
	if err != nil {
		return Result{}, err
	}
	m, err := readManifest(path)
	if err != nil {
		return Result{}, err
	}
	if m == nil {
		return Result{}, fmt.Errorf("nothing to restore for %s: no %s found", name, path+manifestSuffix)
	}
	cur, present, err := readOptional(path)
	if err != nil {
		return Result{}, err
	}
	res := Result{Path: path, Changed: true}
	var out []byte
	remove := false
	switch {
	case present && sum(cur) == m.SHA256 && !m.Dirty:
		if m.Existed {
			if out, err = os.ReadFile(path + backupSuffix); err != nil {
				return res, fmt.Errorf("reading backup: %w", err)
			}
		} else {
			remove = true
		}
	case !present:
		return res, cleanup(path, o.DryRun)
	case !o.Force:
		return res, fmt.Errorf("%s was modified after Convergence configured it; refusing to restore. "+
			"Re-run with --force to revert only the Convergence keys and keep your other edits", path)
	default:
		if out, err = revertEdits(c, cur, m.Edits); err != nil {
			return res, err
		}
		if !m.Existed && isEmptyDoc(c, out) {
			remove = true
		}
	}
	if remove {
		res.Diff = lineDiff(path, cur, nil)
	} else {
		res.Diff = lineDiff(path, cur, out)
	}
	if o.DryRun {
		return res, nil
	}
	if remove {
		if err := os.Remove(path); err != nil {
			return res, err
		}
	} else if err := writeAtomic(path, out, 0o644); err != nil {
		return res, err
	}
	return res, cleanup(path, false)
}

func cleanup(path string, dry bool) error {
	if dry {
		return nil
	}
	for _, s := range []string{backupSuffix, manifestSuffix} {
		if err := os.Remove(path + s); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func isEmptyDoc(c *client, b []byte) bool {
	d, err := c.get(b)
	return err == nil && len(d) == 0
}

func revertEdits(c *client, src []byte, edits []edit) ([]byte, error) {
	doc, err := c.get(src)
	if err != nil {
		return nil, err
	}
	out := src
	for i := len(edits) - 1; i >= 0; i-- {
		e := edits[i]
		curVal, ok := lookup(doc, e.Path...)
		var ours any
		_ = json.Unmarshal(e.New, &ours)
		if !ok || !jsonEqual(curVal, ours) {
			continue // user changed or removed it since: leave alone
		}
		if e.Had {
			var prev any
			if err := json.Unmarshal(e.Prev, &prev); err != nil {
				return nil, err
			}
			out, err = c.set(out, e.Path, prev)
		} else {
			out, err = c.del(out, e.Path)
		}
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

type Status struct {
	Client  string
	Path    string
	Present bool
	Pointed bool
}

func List() ([]Status, error) {
	h, err := home()
	if err != nil {
		return nil, err
	}
	var out []Status
	for i := range clients {
		c := &clients[i]
		p := c.path(h)
		s := Status{Client: c.name, Path: p}
		if b, ok, err := readOptional(p); err == nil && ok {
			s.Present = true
			if d, err := c.get(b); err == nil {
				s.Pointed = c.pointed(d)
			}
		}
		out = append(out, s)
	}
	return out, nil
}

func lineDiff(path string, a, b []byte) string {
	al, bl := splitLines(a), splitLines(b)
	p := 0
	for p < len(al) && p < len(bl) && al[p] == bl[p] {
		p++
	}
	s := 0
	for s < len(al)-p && s < len(bl)-p && al[len(al)-1-s] == bl[len(bl)-1-s] {
		s++
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "--- %s\n+++ %s\n@@ line %d @@\n", path, path, p+1)
	for _, l := range al[p : len(al)-s] {
		sb.WriteString("-" + l + "\n")
	}
	for _, l := range bl[p : len(bl)-s] {
		sb.WriteString("+" + l + "\n")
	}
	return sb.String()
}

func splitLines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}
