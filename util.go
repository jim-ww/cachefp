package cachefp

import (
	"math/bits"
)

// bitLen mirrors JS's Math.floor(Math.log2(n)) for n >= 1, i.e. the index
// of the highest set bit.
func bitLen(n uint64) int {
	if n < 1 {
		n = 1
	}
	return bits.Len64(n) - 1
}
