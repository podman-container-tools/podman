// Code generated from kyber512/internal/mat.go by gen.go

package internal

import (
	"github.com/cloudflare/circl/pke/kyber/internal/common"
)

// A k by k matrix of polynomials.
type Mat [K]Vec

// Expands the given seed to the corresponding matrix A or its transpose Aᵀ.
func (m *Mat) Derive(seed *[32]byte, transpose bool) {
	if !common.DeriveX4Available {
		if transpose {
			for i := 0; i < K; i++ {
				for j := 0; j < K; j++ {
					m[i][j].DeriveUniform(seed, uint8(i), uint8(j))
				}
			}
		} else {
			for i := 0; i < K; i++ {
				for j := 0; j < K; j++ {
					m[i][j].DeriveUniform(seed, uint8(j), uint8(i))
				}
			}
		}
		return
	}

	var ps [4]*common.Poly
	var xs [4]uint8
	var ys [4]uint8
	x := uint8(0)
	y := uint8(0)

	for x != K {
		idx := 0
		for ; idx < 4; idx++ {
			ps[idx] = &m[x][y]

			if transpose {
				xs[idx] = x
				ys[idx] = y
			} else {
				xs[idx] = y
				ys[idx] = x
			}

			y++
			if y == K {
				x++
				y = 0

				if x == K {
					if idx == 0 {
						// If there is just one left, then a plain DeriveUniform
						// is quicker than the X4 variant.
						ps[0].DeriveUniform(seed, xs[0], ys[0])
						return
					}

					for idx++; idx < 4; idx++ {
						ps[idx] = nil
					}

					break
				}
			}
		}

		common.PolyDeriveUniformX4(ps, seed, xs, ys)
	}
}

// Transposes A in place.
func (m *Mat) Transpose() {
	for i := 0; i < K-1; i++ {
		for j := i + 1; j < K; j++ {
			t := m[i][j]
			m[i][j] = m[j][i]
			m[j][i] = t
		}
	}
}
