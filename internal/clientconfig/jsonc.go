package clientconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

// stripJSONC blanks comments and trailing commas so the result is plain JSON
// (byte offsets are preserved).
func stripJSONC(b []byte) []byte {
	out := append([]byte(nil), b...)
	inStr := false
	for i := 0; i < len(out); i++ {
		c := out[i]
		switch {
		case inStr:
			switch c {
			case '\\':
				i++
			case '"':
				inStr = false
			}
		case c == '"':
			inStr = true
		case c == '/' && i+1 < len(out) && out[i+1] == '/':
			for i < len(out) && out[i] != '\n' {
				out[i] = ' '
				i++
			}
		case c == '/' && i+1 < len(out) && out[i+1] == '*':
			out[i], out[i+1] = ' ', ' '
			i += 2
			for i+1 < len(out) && (out[i] != '*' || out[i+1] != '/') {
				if out[i] != '\n' {
					out[i] = ' '
				}
				i++
			}
			if i+1 < len(out) {
				out[i], out[i+1] = ' ', ' '
				i++
			}
		}
	}
	inStr = false
	for i := 0; i < len(out); i++ {
		c := out[i]
		if inStr {
			switch c {
			case '\\':
				i++
			case '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case ',':
			j := i + 1
			for j < len(out) && (out[j] == ' ' || out[j] == '\t' || out[j] == '\n' || out[j] == '\r') {
				j++
			}
			if j < len(out) && (out[j] == '}' || out[j] == ']') {
				out[i] = ' '
			}
		}
	}
	return out
}

func jsonGet(src []byte) (map[string]any, error) {
	clean := bytes.TrimSpace(stripJSONC(src))
	root := map[string]any{}
	if len(clean) == 0 {
		return root, nil
	}
	if err := json.Unmarshal(clean, &root); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}
	return root, nil
}

func lookup(m map[string]any, path ...string) (any, bool) {
	var cur any = m
	for _, p := range path {
		switch c := cur.(type) {
		case map[string]any:
			v, ok := c[p]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			found := false
			for _, it := range c {
				if m, ok := it.(map[string]any); ok && itemPrefix+fmt.Sprint(m["name"]) == p {
					cur, found = it, true
					break
				}
			}
			if !found {
				return nil, false
			}
		default:
			return nil, false
		}
	}
	return cur, true
}

func lookupString(m map[string]any, path ...string) string {
	v, _ := lookup(m, path...)
	s, _ := v.(string)
	return s
}

type member struct {
	key              string
	keyStart         int
	valStart, valEnd int
}

type objSpan struct {
	open, close int
	members     []member
}

func (o objSpan) find(k string) *member {
	for i := range o.members {
		if o.members[i].key == k {
			return &o.members[i]
		}
	}
	return nil
}

type scanner struct {
	b []byte
	i int
}

func (s *scanner) skipWS() error {
	for s.i < len(s.b) {
		switch c := s.b[s.i]; {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			s.i++
		case c == '/' && s.i+1 < len(s.b) && s.b[s.i+1] == '/':
			for s.i < len(s.b) && s.b[s.i] != '\n' {
				s.i++
			}
		case c == '/' && s.i+1 < len(s.b) && s.b[s.i+1] == '*':
			end := bytes.Index(s.b[s.i+2:], []byte("*/"))
			if end < 0 {
				return fmt.Errorf("unterminated comment at offset %d", s.i)
			}
			s.i += end + 4
		default:
			return nil
		}
	}
	return nil
}

func (s *scanner) str() (string, error) {
	start := s.i
	s.i++
	for s.i < len(s.b) && s.b[s.i] != '"' {
		if s.b[s.i] == '\\' {
			s.i++
		}
		s.i++
	}
	if s.i >= len(s.b) {
		return "", fmt.Errorf("unterminated string at offset %d", start)
	}
	s.i++
	var out string
	if err := json.Unmarshal(s.b[start:s.i], &out); err != nil {
		return "", err
	}
	return out, nil
}

func (s *scanner) value() error {
	if err := s.skipWS(); err != nil {
		return err
	}
	if s.i >= len(s.b) {
		return fmt.Errorf("unexpected end of input")
	}
	switch s.b[s.i] {
	case '{':
		_, err := s.object()
		return err
	case '[':
		s.i++
		for {
			if err := s.skipWS(); err != nil {
				return err
			}
			if s.i >= len(s.b) {
				return fmt.Errorf("unterminated array")
			}
			if s.b[s.i] == ']' {
				s.i++
				return nil
			}
			if s.b[s.i] == ',' {
				s.i++
				continue
			}
			start := s.i
			if err := s.value(); err != nil {
				return err
			}
			if s.i == start {
				return fmt.Errorf("unexpected %q in array at offset %d", s.b[s.i], s.i)
			}
		}
	case '"':
		_, err := s.str()
		return err
	}
	for s.i < len(s.b) && !bytes.ContainsRune([]byte(",}] \t\r\n/"), rune(s.b[s.i])) {
		s.i++
	}
	return nil
}

func (s *scanner) object() (objSpan, error) {
	if err := s.skipWS(); err != nil {
		return objSpan{}, err
	}
	if s.i >= len(s.b) || s.b[s.i] != '{' {
		return objSpan{}, fmt.Errorf("expected object at offset %d", s.i)
	}
	obj := objSpan{open: s.i}
	s.i++
	for {
		if err := s.skipWS(); err != nil {
			return obj, err
		}
		if s.i >= len(s.b) {
			return obj, fmt.Errorf("unterminated object")
		}
		switch s.b[s.i] {
		case '}':
			obj.close = s.i
			s.i++
			return obj, nil
		case ',':
			s.i++
			continue
		case '"':
		default:
			return obj, fmt.Errorf("unexpected %q at offset %d", s.b[s.i], s.i)
		}
		m := member{keyStart: s.i}
		k, err := s.str()
		if err != nil {
			return obj, err
		}
		m.key = k
		if err := s.skipWS(); err != nil {
			return obj, err
		}
		if s.i >= len(s.b) || s.b[s.i] != ':' {
			return obj, fmt.Errorf("expected ':' at offset %d", s.i)
		}
		s.i++
		if err := s.skipWS(); err != nil {
			return obj, err
		}
		m.valStart = s.i
		if err := s.value(); err != nil {
			return obj, err
		}
		m.valEnd = s.i
		obj.members = append(obj.members, m)
	}
}

func lineIndent(b []byte, pos int) string {
	start := bytes.LastIndexByte(b[:pos], '\n') + 1
	end := start
	for end < len(b) && (b[end] == ' ' || b[end] == '\t') {
		end++
	}
	return string(b[start:end])
}

func childIndent(b []byte, o objSpan) string {
	base := lineIndent(b, o.open)
	if len(o.members) > 0 && bytes.IndexByte(b[o.open:o.members[0].keyStart], '\n') >= 0 {
		return lineIndent(b, o.members[0].keyStart)
	}
	return base + "  "
}

func marshalIndented(v any, prefix string) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent(prefix, "  ")
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return string(bytes.TrimRight(buf.Bytes(), "\n")), nil
}

func nested(path []string, val any) any {
	if len(path) == 0 {
		return val
	}
	return map[string]any{path[0]: nested(path[1:], val)}
}

func jsonEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	var av, bv any
	return json.Unmarshal(ab, &av) == nil && json.Unmarshal(bb, &bv) == nil && reflect.DeepEqual(av, bv)
}

// jsonSet sets the value at path inside a JSON/JSONC document using minimal
// text edits, so comments, key order and formatting elsewhere are untouched.
func jsonSet(src []byte, path []string, val any) ([]byte, error) {
	if len(bytes.TrimSpace(stripJSONC(src))) == 0 {
		s, err := marshalIndented(nested(path, val), "")
		if err != nil {
			return nil, err
		}
		return []byte(s + "\n"), nil
	}
	cur, err := jsonGet(src)
	if err != nil {
		return nil, err
	}
	if existing, ok := lookup(cur, path...); ok && jsonEqual(existing, val) {
		return src, nil
	}
	sc := &scanner{b: src}
	obj, err := sc.object()
	if err != nil {
		return nil, fmt.Errorf("scanning document: %w", err)
	}
	var out []byte
	for d := 0; ; d++ {
		m := obj.find(path[d])
		ind := childIndent(src, obj)
		if m == nil {
			out, err = insertMember(src, obj, path[d], nested(path[d+1:], val), ind)
			break
		}
		if d == len(path)-1 {
			var s string
			if s, err = marshalIndented(val, ind); err != nil {
				return nil, err
			}
			out = append(append(append([]byte(nil), src[:m.valStart]...), s...), src[m.valEnd:]...)
			break
		}
		if src[m.valStart] != '{' {
			return nil, fmt.Errorf("%q exists but is not an object; refusing to overwrite", path[d])
		}
		sc = &scanner{b: src, i: m.valStart}
		if obj, err = sc.object(); err != nil {
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}
	if _, err := jsonGet(out); err != nil {
		return nil, fmt.Errorf("internal error: edit produced invalid JSON: %w", err)
	}
	return out, nil
}

func insertMember(src []byte, o objSpan, key string, val any, ind string) ([]byte, error) {
	kb, _ := json.Marshal(key)
	vs, err := marshalIndented(val, ind)
	if err != nil {
		return nil, err
	}
	entry := string(kb) + ": " + vs
	var out []byte
	if len(o.members) > 0 {
		at := o.members[len(o.members)-1].valEnd
		out = append(out, src[:at]...)
		out = append(out, ",\n"+ind+entry...)
		return append(out, src[at:]...), nil
	}
	base := lineIndent(src, o.open)
	inner := src[o.open+1 : o.close]
	out = append(out, src[:o.open+1]...)
	out = append(out, "\n"+ind+entry...)
	if len(bytes.TrimSpace(inner)) == 0 {
		return append(append(out, "\n"+base...), src[o.close:]...), nil
	}
	return append(out, src[o.open+1:]...), nil
}

// jsonDelete removes the member at path (no-op when absent), preserving the
// rest of the document byte for byte.
func jsonDelete(src []byte, path []string) ([]byte, error) {
	sc := &scanner{b: src}
	obj, err := sc.object()
	if err != nil {
		return nil, err
	}
	for d := 0; ; d++ {
		m := obj.find(path[d])
		if m == nil {
			return src, nil
		}
		if d < len(path)-1 {
			if src[m.valStart] != '{' {
				return src, nil
			}
			sc = &scanner{b: src, i: m.valStart}
			if obj, err = sc.object(); err != nil {
				return nil, err
			}
			continue
		}
		idx := 0
		for i := range obj.members {
			if obj.members[i].key == m.key {
				idx = i
			}
		}
		var from, to int
		switch {
		case len(obj.members) == 1:
			from, to = m.keyStart, m.valEnd
		case idx == len(obj.members)-1:
			from, to = obj.members[idx-1].valEnd, m.valEnd
		default:
			from, to = m.keyStart, obj.members[idx+1].keyStart
		}
		out := append(append([]byte(nil), src[:from]...), src[to:]...)
		if _, err := jsonGet(out); err != nil {
			return nil, fmt.Errorf("internal error: delete produced invalid JSON: %w", err)
		}
		return out, nil
	}
}
