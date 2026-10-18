// Package pitext computes the terminal's digits without packaging another asset.
package pitext

import "math/big"

// Digits returns pi with the requested decimal places. Machin's identity uses
// fixed-point integers and sixteen guard places; the final division truncates
// only after both alternating arctangent series have converged.
func Digits(places int) string {
	if places < 0 || places > 4096 {
		panic("pi precision must be within 0-4096 decimal places")
	}
	const guard = 16
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places+guard)), nil)
	atan := func(reciprocal int64) *big.Int {
		term := new(big.Int).Quo(new(big.Int).Set(scale), big.NewInt(reciprocal))
		sum := new(big.Int).Set(term)
		square := big.NewInt(reciprocal * reciprocal)
		var part big.Int
		for n := int64(1); ; n++ {
			term.Quo(term, square)
			part.Quo(term, big.NewInt(2*n+1))
			if part.Sign() == 0 {
				break
			}
			if n&1 != 0 {
				sum.Sub(sum, &part)
			} else {
				sum.Add(sum, &part)
			}
		}
		return sum
	}
	value := atan(5)
	value.Mul(value, big.NewInt(16))
	other := atan(239)
	other.Mul(other, big.NewInt(4))
	value.Sub(value, other)
	value.Quo(value, new(big.Int).Exp(big.NewInt(10), big.NewInt(guard), nil))
	s := value.String()
	if places == 0 {
		return s
	}
	return s[:1] + "." + s[1:]
}
