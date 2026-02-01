package gnmi

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/openconfig/gnmi/proto/gnmi"
)

var keyRegex = regexp.MustCompile(`\[([^=\]]+)=([^\]]+)\]`)

func buildPaths(paths []string) ([]*gnmi.Path, error) {
	out := make([]*gnmi.Path, 0, len(paths))
	for _, raw := range paths {
		path, err := parsePath(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, path)
	}
	return out, nil
}

func parsePath(raw string) (*gnmi.Path, error) {
	clean := strings.TrimSpace(raw)
	if clean == "" || clean == "/" {
		return &gnmi.Path{}, nil
	}

	clean = strings.TrimPrefix(clean, "/")
	segments := strings.Split(clean, "/")
	elems := make([]*gnmi.PathElem, 0, len(segments))

	for _, seg := range segments {
		if seg == "" {
			continue
		}
		name := seg
		keys := map[string]string{}
		if idx := strings.Index(seg, "["); idx >= 0 {
			name = seg[:idx]
			for _, match := range keyRegex.FindAllStringSubmatch(seg[idx:], -1) {
				if len(match) != 3 {
					continue
				}
				keys[match[1]] = match[2]
			}
		}
		if name == "" {
			return nil, fmt.Errorf("invalid path segment: %q", seg)
		}
		elems = append(elems, &gnmi.PathElem{Name: name, Key: keys})
	}

	return &gnmi.Path{Elem: elems}, nil
}

func gnmiPathToString(path *gnmi.Path) string {
	if path == nil {
		return "/"
	}

	if len(path.Elem) == 0 && len(path.Element) > 0 {
		return "/" + strings.Join(path.Element, "/")
	}

	if len(path.Elem) == 0 {
		return "/"
	}

	var b strings.Builder
	for _, elem := range path.Elem {
		if elem == nil {
			continue
		}
		b.WriteString("/")
		b.WriteString(elem.Name)

		if len(elem.Key) == 0 {
			continue
		}
		keys := make([]string, 0, len(elem.Key))
		for key := range elem.Key {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			b.WriteString("[")
			b.WriteString(key)
			b.WriteString("=")
			b.WriteString(elem.Key[key])
			b.WriteString("]")
		}
	}

	if b.Len() == 0 {
		return "/"
	}
	return b.String()
}
