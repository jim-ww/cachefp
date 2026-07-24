# cachefp

A Go implementation of [supercookie](https://github.com/jonasstrehle/supercookie) — the
favicon-cache browser fingerprinting technique — packaged as `net/http` middleware
instead of a standalone demo site.

## How it works

Modern browsers cache favicons the same way they cache any other resource. A server
can tell whether a browser already has a given favicon URL cached: if it does, no
request arrives; if it doesn't, one does. By generating a set of per-deployment probe
URLs and selectively causing some of them to be cached (encoding a bit pattern) and
later checking which ones are still cached (decoding it), a persistent identifier can
be assigned to a visitor without using cookies, localStorage, or any other
clearable storage mechanism.

`cachefp` wraps this in a middleware: it drives the whole probe sequence through a
single, invisible background page (no visible redirects), and once an
identifier is established it's cached in a regular cookie so every later request is
just a cookie read — the expensive part only happens once per visitor.

## Usage

```go
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/jim-ww/cachefp"
)

func main() {
	app := http.NewServeMux()
	app.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		id, ok := cachefp.FromContext(r.Context())
		switch {
		case !ok:
			fmt.Fprintln(w, "no identifier yet for this request")
		case id.Identified:
			fmt.Fprintf(w, "visitor identified as %s (#%d)\n", id.Hash, id.ID)
		default:
			fmt.Fprintln(w, "visitor could not be identified")
		}
	})

	mw := cachefp.New(cachefp.Options{})
	log.Fatal(http.ListenAndServe(":8080", mw.Wrap(app)))
}
```

See `cmd/example` for a runnable version. `cachefp.Options` also lets you set a
custom `Store` (default is a small JSON file), the mount prefix for the middleware's
internal routes, cookie name/lifetime, and how many probe bits to generate.

## Browser compatibility

Carried over from the original project's findings. Not independently
re-verified per OS/version here; treat as a starting point, not a guarantee.

#### Current versions (as tested by the original project)

| Browser | Windows | macOS | Linux | iOS | Android | Notes |
|---|:---:|:---:|:---:|:---:|:---:|---|
| Chrome _(v111.0)_ | ✅ | ✅ | ✅ | ? | ✅ | — |
| Safari _(v14.0)_ | — | ✅ | — | ✅ | — | — |
| Edge _(v87.0)_ | ✅ | ✅ | ❌ | ❌ | ✅ | — |
| Firefox _(v86.0)_ | ✅ | ✅ | ❌ | ❌ | ❌ | Fingerprint differs in incognito mode |
| Brave _(v1.19.92)_ | ❌ | ❌ | ❌ | ❔ | ❌ | — |

#### Previous versions (vulnerable)

| Browser | Windows | macOS | Linux | iOS | Android |
|---|:---:|:---:|:---:|:---:|:---:|
| Brave _(v1.14.0)_ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Firefox _(< v84.0)_ | ✅ | ✅ | ❔ | ❌ | ✅ |

Current Chromium reliably survives a cookie clear as long as the browser
cache itself isn't cleared and the session isn't torn down. Firefox-family browsers are unreliable
for this technique because Firefox stores favicons through its own "Places" service
rather than the standard HTTP cache, independent of any hardening.

## Scalability & performance

> By varying the number of bits — which corresponds to the number of probe routes —
> this technique can be scaled almost arbitrarily. It can distinguish 2^N unique
> visitors, where N is the number of probe routes walked on the client side. The
> time taken for the write/read pass grows with N; keeping N no larger than needed
> for the current visitor count keeps that pass short.

`cachefp` follows the same idea but doesn't require a fixed N up front: like the
original, each visitor's read/write pass only walks `ceil(log2(index)) + 1` probe
routes (where `index` is the number of visitors assigned so far), not the full
`Options.ProbeBits` (default 32) — so the pass stays short early on and only grows
as more visitors get enrolled, while `ProbeBits` remains the hard ceiling (2^32-1
identifiers) on how large the deployment can ever grow.

## Disclaimer

Provided for educational and security-research purposes. You are responsible for
complying with applicable law (GDPR/ePrivacy, etc.) and for obtaining consent
before identifying real users with it.
