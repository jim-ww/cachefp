package main

import (
	"cmp"
	"fmt"
	"log"
	"net/http"
	"os"

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

	addr := cmp.Or(os.Getenv("ADDR"), ":8080")
	mw := cachefp.New(cachefp.Options{})
	log.Println("listening on", addr)
	log.Fatal(http.ListenAndServe(addr, mw.Wrap(app)))
}
