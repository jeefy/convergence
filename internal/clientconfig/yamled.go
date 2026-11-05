package clientconfig

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Path elements of the form "name=X" select the list item whose `name` is X.
const itemPrefix = "name="

func yamlGet(src []byte) (map[string]any, error) {
	root := map[string]any{}
	if len(bytes.TrimSpace(src)) == 0 {
		return root, nil
	}
	if err := yaml.Unmarshal(src, &root); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}
	if root == nil {
		root = map[string]any{}
	}
	return root, nil
}

func strNode(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
}

func mapIndex(n *yaml.Node, key string) int {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return i
		}
	}
	return -1
}

func seqIndex(n *yaml.Node, name string) int {
	for i, it := range n.Content {
		if it.Kind == yaml.MappingNode {
			if j := mapIndex(it, "name"); j >= 0 && it.Content[j+1].Value == name {
				return i
			}
		}
	}
	return -1
}

func yamlDoc(src []byte) (*yaml.Node, *yaml.Node, error) {
	var doc yaml.Node
	if len(bytes.TrimSpace(src)) > 0 {
		if err := yaml.Unmarshal(src, &doc); err != nil {
			return nil, nil, fmt.Errorf("parsing YAML: %w", err)
		}
	}
	if doc.Kind == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("top-level YAML is not a mapping; refusing to modify")
	}
	return &doc, doc.Content[0], nil
}

func yamlEncode(doc *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), enc.Close()
}

func yamlSet(src []byte, path []string, val any) ([]byte, error) {
	doc, cur, err := yamlDoc(src)
	if err != nil {
		return nil, err
	}
	var vn yaml.Node
	if err := vn.Encode(val); err != nil {
		return nil, err
	}
	for d, key := range path {
		last := d == len(path)-1
		if cur.Kind == yaml.SequenceNode {
			i := seqIndex(cur, strings.TrimPrefix(key, itemPrefix))
			switch {
			case i >= 0 && last:
				cur.Content[i] = &vn
				return yamlEncode(doc)
			case i >= 0:
				cur = cur.Content[i]
			case last:
				cur.Content = append(cur.Content, &vn)
				return yamlEncode(doc)
			default:
				return nil, fmt.Errorf("list item %q not found", key)
			}
			continue
		}
		i := mapIndex(cur, key)
		switch {
		case i >= 0 && last:
			cur.Content[i+1] = &vn
			return yamlEncode(doc)
		case i >= 0:
			cur = cur.Content[i+1]
			if cur.Kind != yaml.MappingNode && cur.Kind != yaml.SequenceNode {
				return nil, fmt.Errorf("%q exists but is not a mapping or list; refusing to overwrite", key)
			}
		case last:
			cur.Content = append(cur.Content, strNode(key), &vn)
			return yamlEncode(doc)
		default:
			kind, tag := yaml.MappingNode, "!!map"
			if strings.HasPrefix(path[d+1], itemPrefix) {
				kind, tag = yaml.SequenceNode, "!!seq"
			}
			child := &yaml.Node{Kind: kind, Tag: tag}
			cur.Content = append(cur.Content, strNode(key), child)
			cur = child
		}
	}
	return yamlEncode(doc)
}

func yamlDelete(src []byte, path []string) ([]byte, error) {
	doc, cur, err := yamlDoc(src)
	if err != nil {
		return nil, err
	}
	for d, key := range path {
		last := d == len(path)-1
		if cur.Kind == yaml.SequenceNode {
			i := seqIndex(cur, strings.TrimPrefix(key, itemPrefix))
			if i < 0 {
				return src, nil
			}
			if last {
				cur.Content = append(cur.Content[:i], cur.Content[i+1:]...)
				return yamlEncode(doc)
			}
			cur = cur.Content[i]
			continue
		}
		i := mapIndex(cur, key)
		if i < 0 {
			return src, nil
		}
		if last {
			cur.Content = append(cur.Content[:i], cur.Content[i+2:]...)
			return yamlEncode(doc)
		}
		cur = cur.Content[i+1]
	}
	return src, nil
}
