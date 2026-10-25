package httpapi

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// filter is one `f=path:op:value` query param, matched against a row's JSON
// form so any field of GuildRow is filterable without per-field code.
type filter struct {
	path []string
	op   string
	vals []string
}

var filterOps = []string{"eq", "ne", "gt", "gte", "lt", "lte", "in", "nin", "contains"}

// rowFields is the JSON shape of a GuildRow, used to reject unknown paths.
var rowFields = toDoc(GuildRow{})

func toDoc(row GuildRow) map[string]any {
	b, _ := json.Marshal(row)
	var doc map[string]any
	json.Unmarshal(b, &doc)
	return doc
}

func lookup(doc map[string]any, path []string) (any, bool) {
	var v any = doc
	for _, k := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, false
		}
		if v, ok = m[k]; !ok {
			return nil, false
		}
	}
	return v, true
}

func isObject(v any) bool {
	_, ok := v.(map[string]any)
	return ok
}

func parseFilters(raw []string) ([]filter, error) {
	out := make([]filter, 0, len(raw))
	for _, f := range raw {
		parts := strings.SplitN(f, ":", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("filter %q: want path:op:value", f)
		}
		ft := filter{path: strings.Split(parts[0], "."), op: parts[1], vals: []string{parts[2]}}
		if v, ok := lookup(rowFields, ft.path); !ok || isObject(v) {
			return nil, fmt.Errorf("filter %q: unknown field %s", f, parts[0])
		}
		if !slices.Contains(filterOps, ft.op) {
			return nil, fmt.Errorf("filter %q: unknown op %s", f, ft.op)
		}
		if ft.op == "in" || ft.op == "nin" {
			ft.vals = strings.Split(parts[2], ",")
		}
		out = append(out, ft)
	}
	return out, nil
}

func (f filter) match(doc map[string]any) bool {
	v, _ := lookup(doc, f.path)
	switch f.op {
	case "in":
		return slices.ContainsFunc(f.vals, func(want string) bool { return equal(v, want) })
	case "nin":
		return !slices.ContainsFunc(f.vals, func(want string) bool { return equal(v, want) })
	case "contains":
		s, ok := v.(string)
		return ok && strings.Contains(strings.ToLower(s), strings.ToLower(f.vals[0]))
	}
	c, ok := compare(v, f.vals[0])
	switch f.op {
	case "eq":
		return ok && c == 0
	case "ne":
		return !ok || c != 0
	case "gt":
		return ok && c > 0
	case "gte":
		return ok && c >= 0
	case "lt":
		return ok && c < 0
	case "lte":
		return ok && c <= 0
	}
	return false
}

func equal(v any, want string) bool {
	c, ok := compare(v, want)
	return ok && c == 0
}

// compare orders v against want parsed as v's type; !ok when they can't be
// compared, which matches only ne/nin.
func compare(v any, want string) (int, bool) {
	switch v := v.(type) {
	case float64:
		w, err := strconv.ParseFloat(want, 64)
		return cmp.Compare(v, w), err == nil
	case bool:
		w, err := strconv.ParseBool(want)
		return 0, err == nil && v == w
	case string:
		return cmp.Compare(strings.ToLower(v), strings.ToLower(want)), true
	}
	return 0, false
}

// filterRows applies the listing's q search and f filters.
func filterRows(rows []GuildRow, query url.Values) ([]GuildRow, error) {
	filters, err := parseFilters(query["f"])
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query.Get("q"))
	return slices.DeleteFunc(rows, func(g GuildRow) bool {
		if q != "" && !strings.Contains(strings.ToLower(g.Name), q) && g.ID != q {
			return true
		}
		if len(filters) == 0 {
			return false
		}
		doc := toDoc(g)
		return slices.ContainsFunc(filters, func(f filter) bool { return !f.match(doc) })
	}), nil
}
