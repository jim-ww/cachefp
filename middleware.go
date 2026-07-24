// Package cachefp is net/http middleware that identifies returning visitors
// using a browser-cache probing technique: a set of per-deployment probe
// URLs are selectively fetched so that which ones come back from cache (vs.
// trigger a real request) encodes a persistent identifier, independent of
// cookies or local storage.
//
// Wrap the handlers you want identified. On a visitor's first pass the
// middleware detours them through a single background page: it fetches
// each probe route via JS and talks to the middleware's JSON endpoints
// between probes, then makes one final navigation back to the URL originally
// requested. Every request after that is a single cookie read — FromContext
// returns the Identifier with no extra round trip.

package cachefp

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ProbeBits controls how many probe routes are generated per deployment;
// 2^ProbeBits-1 unique identifiers are possible. 32 is a sane default and
// matches a native machine word on most platforms.
const DefaultProbeBits = 32

// Options configures a Middleware.
type Options struct {
	// MountPrefix is the path prefix under which the middleware serves its
	// internal probe routes. Must not collide with routes in the wrapped
	// application. Defaults to "/_cachefp".
	MountPrefix string
	// CookieName is where the resolved identifier is stored once known.
	// Defaults to "cfpid".
	CookieName string
	// CookieMaxAge controls how long an established identifier cookie lasts.
	// Defaults to 1 year.
	CookieMaxAge time.Duration
	// Store holds the small amount of persistent state (the per-deployment
	// cache ID and next write index). Defaults to a JSONStore at
	// StoragePath; set Store directly to use a different backend (a
	// database row, a KV store, etc.) instead of a JSON file.
	Store Store
	// StoragePath is where the default JSONStore keeps its file. Ignored if
	// Store is set. Defaults to "cachefp_data.json".
	StoragePath string
	// ProbeBits sets the number of probe routes; see DefaultProbeBits.
	ProbeBits int
	// MaxAttempts caps how many write/read passes are made before giving up
	// on a visitor and marking them non-identifiable, to avoid looping
	// forever for browsers that don't exhibit the caching behavior this
	// technique relies on. Defaults to 2.
	MaxAttempts int
	// Logf, if set, receives diagnostic log lines. Defaults to log.Printf.
	Logf func(format string, args ...any)
}

func (o Options) withDefaults() Options {
	if o.MountPrefix == "" {
		o.MountPrefix = "/_cachefp"
	}
	o.MountPrefix = strings.TrimSuffix(o.MountPrefix, "/")
	if o.CookieName == "" {
		o.CookieName = "cfpid"
	}
	if o.CookieMaxAge == 0 {
		o.CookieMaxAge = 365 * 24 * time.Hour
	}
	if o.StoragePath == "" {
		o.StoragePath = "cachefp_data.json"
	}
	if o.ProbeBits == 0 {
		o.ProbeBits = DefaultProbeBits
	}
	if o.MaxAttempts == 0 {
		o.MaxAttempts = 2
	}
	if o.Logf == nil {
		o.Logf = log.Printf
	}
	return o
}

// Middleware identifies visitors via cache probing and exposes the result
// through FromContext.
type Middleware struct {
	opts        Options
	store       Store
	routes      *RouteTable
	profiles    *ProfileStore
	writeTokens *WriteTokenSet
	cacheID     string
	maxN        uint64
}

// New builds a Middleware. It reads (or creates) its persistent state
// immediately, via Options.Store if set, or a JSONStore at
// Options.StoragePath otherwise.
func New(opts Options) *Middleware {
	opts = opts.withDefaults()
	store := opts.Store
	if store == nil {
		store = NewJSONStore(opts.StoragePath)
	}
	cacheID := getString(store, "cacheID", generateUUID("xxxxxxxx", "0123456789abcdef"))
	store.Set("cacheID", cacheID)
	if _, ok := store.Get("index"); !ok {
		store.Set("index", uint64(1))
	}

	return &Middleware{
		opts:        opts,
		store:       store,
		routes:      NewRouteTable(cacheID, opts.ProbeBits),
		profiles:    NewProfileStore(),
		writeTokens: NewWriteTokenSet(),
		cacheID:     cacheID,
		maxN:        1<<uint(opts.ProbeBits) - 1,
	}
}

// Hash renders a decoded identifier as a short, human-readable string.
func Hash(id uint64) string { return hashNumber(id) }

const (
	cookieUID      = "cfp_uid"
	cookieMID      = "cfp_mid"
	cookieReturn   = "cfp_return"
	cookieAttempts = "cfp_attempts"
)

// Wrap returns an http.Handler that identifies visitors before delegating
// to next. GET requests without a resolved identifier are sent to the
// background probe page and, once identification finishes (or gives up),
// navigated back to the URL they originally requested. Non-GET requests are
// passed through with no Identifier in context.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	prefix := m.opts.MountPrefix

	mux.HandleFunc("GET "+prefix+"/start", m.handleStart)
	mux.HandleFunc("GET "+prefix+"/l/{ref}", m.handleBeacon)
	mux.HandleFunc("GET "+prefix+"/begin/read", m.handleBeginRead)
	mux.HandleFunc("GET "+prefix+"/begin/write/{mid}", m.handleBeginWrite)
	mux.HandleFunc("GET "+prefix+"/f/{ref}", m.handleFavicon)
	mux.HandleFunc("GET "+prefix+"/identity", m.handleIdentity)
	mux.HandleFunc("/", m.handlePassthrough(next))

	return mux
}

func (m *Middleware) handlePassthrough(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if v := cookieValue(r, m.opts.CookieName); v != "" {
			if id, err := strconv.ParseUint(v, 10, 64); err == nil {
				ident := Identifier{ID: id, Hash: Hash(id), Identified: id != 0}
				next.ServeHTTP(w, r.WithContext(withIdentifier(r.Context(), ident)))
				return
			}
		}
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		setCookie(w, cookieReturn, sanitizeReturn(r.URL.RequestURI()), 5*time.Minute)
		http.Redirect(w, r, m.opts.MountPrefix+"/start", http.StatusFound)
	}
}

func sanitizeReturn(path string) string {
	if path == "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return "/"
	}
	return path
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

// GET {prefix}/start — the only page the visitor actually sees mid-flow. It
// stays put for the whole identification pass: a script fetches each probe
// route in turn (resolving instantly on a cache hit, or after a real round
// trip on a miss) and drives the JSON endpoints below, then performs a
// single final navigation back to the originally requested URL.
func (m *Middleware) handleStart(w http.ResponseWriter, r *http.Request) {
	prefix := m.opts.MountPrefix
	renderTemplate(w, "start.html", map[string]string{
		"prefix":     prefix,
		"beaconHref": prefix + "/l/" + m.cacheID,
	})
}

// GET {prefix}/l/{ref} — hit only when the visitor's browser did not already
// have this deployment's beacon favicon cached, i.e. an unknown visitor.
func (m *Middleware) handleBeacon(w http.ResponseWriter, r *http.Request) {
	setCookie(w, cookieMID, m.writeTokens.Generate(), time.Minute)
	writePixel(w)
}

type beginManifest struct {
	Routes []string `json:"routes"`
}

// GET {prefix}/begin/read — starts (or restarts) a decode pass: creates a
// fresh profile and returns the ordered list of probe routes the client
// should fetch. Which ones come back from cache (vs. trigger a request to
// /f/{ref}) is what encodes the identifier.
func (m *Middleware) handleBeginRead(w http.ResponseWriter, r *http.Request) {
	uid := newUUID()
	profile := m.profiles.FromRead(uid)
	if profile == nil {
		w.WriteHeader(http.StatusConflict)
		return
	}
	index := getUint64(m.store, "index", 1)
	size := bitLen(index) + 1
	profile.SetStorageSize(size)
	setCookie(w, cookieUID, uid, time.Minute)
	writeJSON(w, beginManifest{Routes: m.routeSlice(size)})
}

// GET {prefix}/begin/write/{mid} — starts an encode pass for a new visitor:
// assigns them the next identifier and returns the probe routes to fetch so
// that identifier gets baked into their cache.
func (m *Middleware) handleBeginWrite(w http.ResponseWriter, r *http.Request) {
	mid := r.PathValue("mid")
	if !m.writeTokens.Has(mid) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	clearCookie(w, cookieMID)
	m.writeTokens.Delete(mid)

	profile, routes := m.beginWrite()
	if profile == nil {
		w.WriteHeader(http.StatusConflict)
		return
	}
	writeJSON(w, beginManifest{Routes: routes})
}

// beginWrite creates a fresh write-mode profile for the next unassigned
// identifier and sets its session cookie, returning the probe routes to
// walk. Shared by handleBeginWrite and the retry path in handleIdentity.
func (m *Middleware) beginWrite() (*Profile, []string) {
	uid := newUUID()
	index := getUint64(m.store, "index", 1)
	m.opts.Logf("cachefp | unknown visitor uid=%q assigned index=%d", uid, index)
	profile := m.profiles.FromWrite(uid, index, m.routes)
	if profile == nil {
		return nil, nil
	}
	m.store.Set("index", index+1)
	return profile, m.routeSlice(bitLen(index) + 1)
}

func (m *Middleware) routeSlice(count int) []string {
	if count > m.routes.Len() {
		count = m.routes.Len()
	}
	routes := make([]string, count)
	for i := range routes {
		routes[i] = m.routes.RouteByIndex(i)
	}
	return routes
}

// setUIDCookie is a small helper so beginWrite's caller can attach the
// profile's session cookie to the response (beginWrite itself has no
// http.ResponseWriter, since it's also called from handleIdentity where the
// cookie is set alongside other response state).
func setUIDCookie(w http.ResponseWriter, profile *Profile) {
	setCookie(w, cookieUID, profile.uid, time.Minute)
}

// GET {prefix}/f/{ref} — the actual probe pixel. In read mode every hit is
// recorded (it means this route wasn't already cached) and the response is
// deliberately left empty. In write mode only routes in the visitor's
// assigned vector are served, so only those get cached.
func (m *Middleware) handleFavicon(w http.ResponseWriter, r *http.Request) {
	referrer := r.PathValue("ref")
	uid := cookieValue(r, cookieUID)
	if !m.profiles.Has(uid) || !m.routes.HasRoute(referrer) {
		denyCache(w)
		return
	}
	profile := m.profiles.Get(uid)
	if profile.IsReading() {
		profile.VisitRoute(referrer)
		denyCache(w)
		return
	}
	if !profile.VectorContains(referrer) {
		denyCache(w)
		return
	}
	writePixel(w)
}

// denyCache answers a probe request that must NOT end up cached by the
// browser: an uncached response here means the corresponding bit gets
// correctly read back as unset on the next pass. Without an explicit
// no-store, a bare 200 could be cached by the browser's own heuristics and
// permanently corrupt that bit.
func denyCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
}

type identityResult struct {
	Identified bool       `json:"identified"`
	Hash       string     `json:"hash,omitempty"`
	ID         uint64     `json:"id,omitempty"`
	Retry      *retryInfo `json:"retry,omitempty"`
	ReturnPath string     `json:"returnPath,omitempty"`
}

type retryInfo struct {
	Mode   string   `json:"mode"`
	Mid    string   `json:"mid,omitempty"`
	Routes []string `json:"routes,omitempty"`
}

// GET {prefix}/identity — decodes the just-completed read pass into an
// identifier. A degenerate result (nothing usable, or a saturated
// identifier) means this is genuinely a new visitor: the response asks the
// client to run a write pass (routes included directly, no beacon round
// trip needed since we already know the answer) followed by another read
// pass, up to MaxAttempts times before giving up.
func (m *Middleware) handleIdentity(w http.ResponseWriter, r *http.Request) {
	uid := cookieValue(r, cookieUID)
	profile := m.profiles.Get(uid)
	if profile == nil {
		writeJSON(w, identityResult{ReturnPath: sanitizeReturn(cookieValue(r, cookieReturn))})
		return
	}
	clearCookie(w, cookieUID)
	m.profiles.Delete(uid)

	identifier := profile.CalcIdentifier(m.routes)
	if identifier == m.maxN || profile.VisitedCount() == 0 || identifier == 0 {
		attempts, _ := strconv.Atoi(cookieValue(r, cookieAttempts))
		attempts++
		if attempts > m.opts.MaxAttempts {
			m.finish(w, r, 0, false)
			return
		}
		setCookie(w, cookieAttempts, strconv.Itoa(attempts), 5*time.Minute)

		writeProfile, routes := m.beginWrite()
		if writeProfile == nil {
			m.finish(w, r, 0, false)
			return
		}
		setUIDCookie(w, writeProfile)
		writeJSON(w, identityResult{Retry: &retryInfo{Mode: "write", Routes: routes}})
		return
	}

	m.opts.Logf("cachefp | visitor identified as %s (#%d)", Hash(identifier), identifier)
	m.finish(w, r, identifier, true)
}

func (m *Middleware) finish(w http.ResponseWriter, r *http.Request, id uint64, identified bool) {
	clearCookie(w, cookieAttempts)
	clearCookie(w, cookieReturn)
	value := "0"
	result := identityResult{
		Identified: identified,
		ReturnPath: sanitizeReturn(cookieValue(r, cookieReturn)),
	}
	if identified {
		value = strconv.FormatUint(id, 10)
		result.Hash = Hash(id)
		result.ID = id
	}
	setCookie(w, m.opts.CookieName, value, m.opts.CookieMaxAge)

	writeJSON(w, result)
}
