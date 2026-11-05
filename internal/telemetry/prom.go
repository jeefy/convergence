// Package telemetry scrapes Prometheus text endpoints exposed by the stack.
package telemetry

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Sample is one series sample.
type Sample struct {
	Name   string
	Labels map[string]string
	Value  float64
}

// ParseProm parses Prometheus text exposition format. Comment lines,
// malformed lines, NaN values and histogram buckets are skipped.
func ParseProm(r io.Reader) ([]Sample, error) {
	var out []Sample
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		s, ok := parseLine(line)
		if !ok || math.IsNaN(s.Value) || strings.HasSuffix(s.Name, "_bucket") {
			continue
		}
		out = append(out, s)
	}
	return out, sc.Err()
}

func parseLine(line string) (Sample, bool) {
	i := 0
	for i < len(line) && line[i] != '{' && line[i] != ' ' && line[i] != '\t' {
		i++
	}
	s := Sample{Name: line[:i], Labels: map[string]string{}}
	if s.Name == "" {
		return s, false
	}
	rest := line[i:]
	if strings.HasPrefix(rest, "{") {
		end, ok := parseLabels(rest, s.Labels)
		if !ok {
			return s, false
		}
		rest = rest[end:]
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return s, false
	}
	v, err := parseValue(fields[0])
	if err != nil {
		return s, false
	}
	s.Value = v
	return s, true
}

// parseLabels parses `{k="v",...}` and returns the index just past '}'.
func parseLabels(s string, dst map[string]string) (int, bool) {
	i := 1
	for {
		for i < len(s) && (s[i] == ' ' || s[i] == ',') {
			i++
		}
		if i >= len(s) {
			return 0, false
		}
		if s[i] == '}' {
			return i + 1, true
		}
		eq := strings.IndexByte(s[i:], '=')
		if eq < 0 || i+eq+1 >= len(s) || s[i+eq+1] != '"' {
			return 0, false
		}
		key := strings.TrimSpace(s[i : i+eq])
		i += eq + 2
		var b strings.Builder
		closed := false
		for i < len(s) {
			c := s[i]
			if c == '\\' && i+1 < len(s) {
				if s[i+1] == 'n' {
					b.WriteByte('\n')
				} else {
					b.WriteByte(s[i+1])
				}
				i += 2
				continue
			}
			i++
			if c == '"' {
				closed = true
				break
			}
			b.WriteByte(c)
		}
		if !closed {
			return 0, false
		}
		dst[key] = b.String()
	}
}

func parseValue(s string) (float64, error) {
	switch s {
	case "+Inf", "Inf":
		return math.Inf(1), nil
	case "-Inf":
		return math.Inf(-1), nil
	case "NaN":
		return math.NaN(), nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("bad value %q: %w", s, err)
	}
	return v, nil
}

// Sum adds all finite samples named name whose labels include match.
func Sum(samples []Sample, name string, match map[string]string) float64 {
	var t float64
	for _, s := range samples {
		if s.Name == name && labelsMatch(s.Labels, match) && !math.IsInf(s.Value, 0) {
			t += s.Value
		}
	}
	return t
}

// SumBy groups the sum of metric name by the value of label by.
func SumBy(samples []Sample, name, by string) map[string]float64 {
	m := map[string]float64{}
	for _, s := range samples {
		if s.Name == name && !math.IsInf(s.Value, 0) {
			m[s.Labels[by]] += s.Value
		}
	}
	return m
}

// Has reports whether any sample with this name exists.
func Has(samples []Sample, name string) bool {
	for _, s := range samples {
		if s.Name == name {
			return true
		}
	}
	return false
}

func labelsMatch(have, want map[string]string) bool {
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}
