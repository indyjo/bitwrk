//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2013  Jonas Eschenburg <jonas@bitwrk.net>
//
//  This program is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  This program is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with this program.  If not, see <http://www.gnu.org/licenses/>.

package bitcoin

import (
	"bitelliptic"
	"fmt"
	"math/big"
)

func legendre(a, p *big.Int) int {
	var r big.Int
	r.Rsh(p, 1)
	r.Exp(a, &r, p)
	switch r.BitLen() {
	case 0:
		return 0
	case 1:
		return 1
	}
	return -1
}

// Returns 'ret' such that
//      ret^2 == a (mod p),
// using the Tonelli/Shanks algorithm (cf. Henri Cohen, "A Course
// in Algebraic Computational Number Theory", algorithm 1.5.1).
// 'p' must be prime!
// If 'a' is not a square, this is not necessarily detected by
// the algorithms; a bogus result must be expected in this case.
func sqrtMod(a, p *big.Int) (*big.Int, error) {
	// Translated manually from OpenSSL's implementation in bn_sqrt.c
	// Simplified to not check for primeness of p

	var r int
	var b, q, t, x, y big.Int
	var e, i, j int

	/* now write  |p| - 1  as  2^e*q  where  q  is odd */
	e = 1
	for p.Bit(e) == 0 {
		e++
	}
	/* we'll set  q  later (if needed) */
	//return nil, fmt.Errorf("a: %x p: %x e:%v", a, p, e)

	if e == 1 {
		/* The easy case:  (|p|-1)/2  is odd, so 2 has an inverse
		 * modulo  (|p|-1)/2,  and square roots can be computed
		 * directly by modular exponentiation.
		 * We have
		 *     2 * (|p|+1)/4 == 1   (mod (|p|-1)/2),
		 * so we can use exponent  (|p|+1)/4,  i.e.  (|p|-3)/4 + 1.
		 */
		q.Abs(p).Rsh(&q, 2).Add(&q, big.NewInt(1))
		q.Exp(a, &q, p)
		return &q, nil
	} else if e == 2 {
		// omitted, not used for secp256k1
		panic("Not general case, e")
	}

	// !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
	// Rest of this func isn't needed for secp256k1
	// -> probably not working...
	// !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!

	/* e > 2, so we really have to use the Tonelli/Shanks algorithm.
	 * First, find some  y  that is not a square. */
	q.Abs(p) // use 'q' as temp

	i = 2
	for {
		/* For efficiency, try small numbers first;
		 * if this fails, try random numbers.
		 */
		if i < 22 {
			y.SetInt64(int64(i))
		} else {
			panic("No nonquadratic residual found")
			/*
				if (!BN_pseudo_rand(y, BN_num_bits(p), 0, 0)) goto end;
				if (BN_ucmp(y, p) >= 0)
					{
					if (!(p->neg ? BN_add : BN_sub)(y, y, p)) goto end;
					}
				// now 0 <= y < |p|
				if (BN_is_zero(y))
					if (!BN_set_word(y, i)) goto end;
			*/
		}

		r = legendre(&y, &q) /* here 'q' is |p| */

		i++
		if r != 1 || i >= 82 {
			break
		}
	}

	if r != -1 {
		/* Many rounds and still no non-square -- this is more likely
		 * a bug than just bad luck.
		 * Even if  p  is not prime, we should have found some  y
		 * such that r == -1.
		 */
		return nil, fmt.Errorf("Too many iterations")
	}

	/* Here's our actual 'q': */
	q.Rsh(&q, uint(e))

	/* Now that we have some non-square, we can find an element
	 * of order  2^e  by computing its q'th power. */
	y.Exp(&y, &q, p)

	/* Now we know that (if  p  is indeed prime) there is an integer
	 * k,  0 <= k < 2^e,  such that
	 *
	 *      a^q * y^k == 1   (mod p).
	 *
	 * As  a^q  is a square and  y  is not,  k  must be even.
	 * q+1  is even, too, so there is an element
	 *
	 *     X := a^((q+1)/2) * y^(k/2),
	 *
	 * and it satisfies
	 *
	 *     X^2 = a^q * a     * y^k
	 *         = a,
	 *
	 * so it is the square root that we are looking for.
	 */

	/* t := (q-1)/2  (note that  q  is odd) */
	t.Rsh(&q, 1)

	/* x := a^((q-1)/2) */
	if t.BitLen() == 0 {
		/* special case: p = 2^e + 1 */
		panic("Special case not handled")
	} else {
		x.Exp(a, &t, p)
		if a.BitLen() == 0 {
			/* special case: a == 0  (mod p) */
			return new(big.Int), nil
		}
	}

	/* b := a*x^2  (= a^q) */
	b.Mul(&x, &x).Mod(&b, p).Mul(a, &b).Mod(&b, p)

	/* x := a*x    (= a^((q+1)/2)) */
	x.Mul(a, &x).Mod(&x, p)

	one := big.NewInt(1)
	for {
		/* Now  b  is  a^q * y^k  for some even  k  (0 <= k < 2^E
		 * where  E  refers to the original value of  e,  which we
		 * don't keep in a variable),  and  x  is  a^((q+1)/2) * y^(k/2).
		 *
		 * We have  a*b = x^2,
		 *    y^2^(e-1) = -1,
		 *    b^2^(e-1) = 1.
		 */

		if b.Cmp(one) == 0 {
			return new(big.Int).Set(&x), nil
		}

		/* find smallest  i  such that  b^(2^i) = 1 */
		i = 1
		t.Mul(&b, &b).Mod(&t, p)
		for t.Cmp(one) != 0 {
			i++
			if i == e {
				return nil, fmt.Errorf("Not a square: t=%v i=%v", t, i)
			}
			t.Mul(&t, &t).Mod(&t, p)
		}

		/* t := y^2^(e - i - 1) */
		t.Set(&y)
		for j = e - i - 1; j > 0; j-- {
			t.Mul(&t, &t).Mod(&t, p)
		}
		y.Mul(&t, &t).Mod(&y, p)
		x.Mul(&x, &t).Mod(&x, p)
		b.Mul(&b, &y).Mod(&b, p)
		e = i
	}

	panic("Shouldn't end up here")
}

func uncompressPoint(x *big.Int, curve *bitelliptic.BitCurve, y_even bool) (rx, ry *big.Int, err error) {
	rx = x
	ry = new(big.Int).Set(x)
	p := curve.P
	ry.Mul(ry, x).Mod(ry, p)
	ry.Mul(ry, x).Mod(ry, p)
	ry.Add(ry, curve.B).Mod(ry, p)
	ry, err = sqrtMod(ry, p)
	if err != nil {
		return nil, nil, err
	}

	if y_even != (0 == ry.Bit(0)) {
		ry.Neg(ry).Mod(ry, p)
	}

	return
}

func RecoverPubKeyFromSignature(r, s *big.Int, msg []byte, curve *bitelliptic.BitCurve, recid uint) (qx, qy *big.Int, err error) {
	if recid > 3 {
		return nil, nil, fmt.Errorf("Illegal recid %v - must be in 0..3", recid)
	}
	order := curve.N
	i := recid / 2
	field := curve.P
	x := new(big.Int).Set(order)
	x.Mul(x, big.NewInt(int64(i)))
	x.Add(x, r)
	if x.Cmp(field) >= 0 {
		err = fmt.Errorf("%v >= %v", x, field)
		return
	}

	rx, ry, err := uncompressPoint(x, curve, 0 == (recid%2))
	if err != nil {
		return nil, nil, err
	}
	if !curve.IsOnCurve(rx, ry) {
		return nil, nil, fmt.Errorf("Point %d, %d not on curve", rx, ry)
	}

	e := new(big.Int).SetBytes(msg)
	if 8*len(msg) > curve.BitSize {
		e.Rsh(e, uint(8-(curve.BitSize&7)))
	}
	e.Neg(e).Mod(e, order)

	var rr, sor, eor big.Int
	rr.ModInverse(r, order)
	//return nil, nil, fmt.Errorf("r: %d, r_inv: %d", r, &rr)
	//Q = (R.multiply(s).add(G.multiply(minus_e))).multiply(inv_r);
	sor.Mul(s, &rr)
	eor.Mul(e, &rr)

	Georx, Geory := curve.ScalarBaseMult(eor.Bytes())
	Rsorx, Rsory := curve.ScalarMult(rx, ry, sor.Bytes())
	qx, qy = curve.Add(Georx, Geory, Rsorx, Rsory)

	return
}
