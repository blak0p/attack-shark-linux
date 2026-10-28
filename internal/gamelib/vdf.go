package gamelib

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	maxFileBytes = 4 << 20
	maxVDFDepth  = 32
)

// vdfNode is a parsed Valve text-format block. Keys are lower-cased; values are
// either a string or a nested vdfNode.
type vdfNode map[string]any

func (n vdfNode) str(key string) string {
	s, _ := n[key].(string)
	return s
}

func (n vdfNode) node(key string) vdfNode {
	c, _ := n[key].(vdfNode)
	return c
}

func readSmallFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, maxFileBytes))
}

type vdfParser struct {
	s string
	i int
}

func parseVDF(s string) (vdfNode, error) {
	p := &vdfParser{s: s}
	return p.block(0)
}

// next returns the next token: kind 's' for a string, '{' or '}', or 0 at the end.
func (p *vdfParser) next() (string, byte, error) {
	for p.i < len(p.s) {
		c := p.s[p.i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			p.i++
		case c == '/' && p.i+1 < len(p.s) && p.s[p.i+1] == '/':
			for p.i < len(p.s) && p.s[p.i] != '\n' {
				p.i++
			}
		case c == '{' || c == '}':
			p.i++
			return "", c, nil
		case c == '"':
			p.i++
			var b strings.Builder
			for p.i < len(p.s) {
				c := p.s[p.i]
				p.i++
				if c == '"' {
					return b.String(), 's', nil
				}
				if c == '\\' && p.i < len(p.s) {
					switch e := p.s[p.i]; e {
					case 'n':
						b.WriteByte('\n')
					case 't':
						b.WriteByte('\t')
					default:
						b.WriteByte(e)
					}
					p.i++
					continue
				}
				b.WriteByte(c)
			}
			return "", 0, errors.New("unterminated string")
		default:
			return "", 0, fmt.Errorf("unexpected character %q", c)
		}
	}
	return "", 0, nil
}

func (p *vdfParser) block(depth int) (vdfNode, error) {
	if depth > maxVDFDepth {
		return nil, errors.New("blocks nested too deeply")
	}
	node := vdfNode{}
	for {
		key, kind, err := p.next()
		if err != nil {
			return nil, err
		}
		switch kind {
		case 0:
			if depth > 0 {
				return nil, errors.New("unexpected end of input")
			}
			return node, nil
		case '}':
			if depth == 0 {
				return nil, errors.New("unexpected closing brace")
			}
			return node, nil
		case '{':
			return nil, errors.New("unexpected opening brace")
		}
		val, vkind, err := p.next()
		if err != nil {
			return nil, err
		}
		switch vkind {
		case 's':
			node[strings.ToLower(key)] = val
		case '{':
			child, err := p.block(depth + 1)
			if err != nil {
				return nil, err
			}
			node[strings.ToLower(key)] = child
		default:
			return nil, errors.New("key without a value")
		}
	}
}
