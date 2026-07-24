package cachefp

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"math/big"
	"strconv"
	"strings"
)

const (
	defaultUUIDPattern = "xxxx-xxxx-xxxx-xxxx-xxxx"
	defaultUUIDCharset = "abcdefghijklmnopqrstuvwxyz0123456789"
)

// generateUUID fills every 'x' in pattern with a random rune from charset.
func generateUUID(pattern, charset string) string {
	var sb strings.Builder
	sb.Grow(len(pattern))
	max := big.NewInt(int64(len(charset)))
	for _, r := range pattern {
		if r != 'x' {
			sb.WriteRune(r)
			continue
		}
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic(err)
		}
		sb.WriteByte(charset[n.Int64()])
	}
	return sb.String()
}

func newUUID() string {
	return generateUUID(defaultUUIDPattern, defaultUUIDCharset)
}

// hashNumber renders the last 12 hex chars of MD5(value) as uppercase byte pairs.
func hashNumber(value uint64) string {
	sum := md5.Sum([]byte(strconv.FormatUint(value, 10)))
	full := hex.EncodeToString(sum[:])
	last12 := full[len(full)-12:]
	pairs := make([]string, 0, 6)
	for i := 0; i < len(last12); i += 2 {
		pairs = append(pairs, last12[i:i+2])
	}
	return strings.ToUpper(strings.Join(pairs, " "))
}

var routeReplacer = strings.NewReplacer("=", "0", "+", "0", "/", "0")

// createRoutes derives `count` deterministic route slugs from base.
func createRoutes(base string, count int) []string {
	routes := make([]string, count)
	for i := range count {
		sum := md5.Sum([]byte(base + strconv.Itoa(i)))
		b64 := base64.StdEncoding.EncodeToString(sum[:])
		routes[i] = routeReplacer.Replace(b64)[:22]
	}
	return routes
}
