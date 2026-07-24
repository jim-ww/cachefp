package cachefp

import (
	"encoding/base64"
	"net/http"
)

// pixelPNG is a 1x1 transparent PNG served as the favicon probe/beacon.
const pixelB64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+ip1sAAAAASUVORK5CYII="

var pixelPNG = func() []byte {
	b, err := base64.StdEncoding.DecodeString(pixelB64)
	if err != nil {
		panic(err)
	}
	return b
}()

func writePixel(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	w.Write(pixelPNG)
}
