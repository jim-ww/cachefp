package cachefp

import (
	"slices"
	"strconv"
	"strings"
)

// routeTable is the ordered, fixed set of favicon-cache probe routes for one
// cache identifier, plus the bit<->route conversions used to encode/decode
// a visitor's cache profile.
type routeTable struct {
	cacheID string
	routes  []string
	index   map[string]int
}

func newRouteTable(cacheID string, n int) *routeTable {
	base := createRoutes(cacheID, n)
	routes := make([]string, n)
	index := make(map[string]int, n)
	for i, r := range base {
		full := cacheID + ":" + r
		routes[i] = full
		index[full] = i
	}
	return &routeTable{cacheID: cacheID, routes: routes, index: index}
}

func (t *routeTable) Len() int { return len(t.routes) }

func (t *routeTable) HasRoute(route string) bool {
	_, ok := t.index[route]
	return ok
}

func (t *routeTable) RouteByIndex(i int) string {
	if i < 0 || i >= len(t.routes) {
		return ""
	}
	return t.routes[i]
}

func (t *routeTable) IndexByRoute(route string) int {
	if i, ok := t.index[route]; ok {
		return i
	}
	return -1
}

// NextRoute returns the route after `route`, or "" once the table is exhausted.
func (t *routeTable) NextRoute(route string) string {
	i, ok := t.index[route]
	if !ok {
		return ""
	}
	return t.RouteByIndex(i + 1)
}

// Vector returns the routes whose bit is set in identifier (bit i <-> routes[i]).
func (t *routeTable) Vector(identifier uint32) []string {
	bin := []byte(fmt32b(identifier, len(t.routes)))
	slices.Reverse(bin)
	vector := make([]string, 0, len(t.routes))
	for i, c := range bin {
		if c == '1' {
			vector = append(vector, t.routes[i])
		}
	}
	return vector
}

// Identifier reconstructs the numeric identifier from a set of visited routes,
// only considering the first `size` routes in the table.
func (t *routeTable) Identifier(visited map[string]bool, size int) uint64 {
	if size > len(t.routes) {
		size = len(t.routes)
	}
	if size <= 0 {
		return 0
	}
	bits := make([]byte, size)
	for i := 0; i < size; i++ {
		if visited[t.routes[i]] {
			bits[i] = '0'
		} else {
			bits[i] = '1'
		}
	}
	slices.Reverse(bits)
	val, _ := strconv.ParseUint(string(bits), 2, 64)
	return val
}

func fmt32b(v uint32, width int) string {
	s := strconv.FormatUint(uint64(v), 2)
	if len(s) < width {
		s = strings.Repeat("0", width-len(s)) + s
	}
	return s
}
