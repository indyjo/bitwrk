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

package money

import (
	"bytes"
	"encoding/json"
	"testing"
)

func Test_Parse(t *testing.T) {
	m := new(Money)
	err := m.Parse("satoshi 0.2")
	if err != nil {
		t.Log("Correctly got error: " + err.Error())
	} else {
		t.Error("Parse accepted erroneous input: satoshi 0.2")
	}

	err = m.Parse("BTC99999999999999")
	if err != nil {
		t.Log("Correctly got error: " + err.Error())
	} else {
		t.Error("Parse accepted erroneous input: BTC99999999999999")
	}

	type testcase struct {
		s string
		v int64
	}

	for _, c := range []testcase{
		{"satoshi-1", -1},
		{"satoshi 0", 0},
		{"satoshi 1", 1},
		{"satoshi 10373476", 10373476},
		{"BTC 0.1", 10000000},
		{"BTC 0.01", 1000000},
		{"BTC 0.001", 100000},
		{"BTC 0.0001", 10000},
		{"BTC0.00000001", 1},
		{"mBTC0.00001", 1},
		{"uBTC0.01", 1},
		{"BTC -23.2549", -2325490000},
	} {
		err = m.Parse(c.s)
		if err != nil {
			t.Error("Unexpected parse error: " + err.Error())
			continue
		}
		if m.Amount == c.v {
			t.Logf("Correctly parsed %v as %d", c.s, c.v)
		} else {
			t.Errorf("Wrong parse result: %d instead of %d when parsing %v", m.Amount, c.v, c.s)
		}
	}

	for _, s := range []string{
		"12",
		"BTC_12",
		"-BTC12",
		"1 BTC",
		"BTC0,2",
	} {
		err = m.Parse(s)
		if err == nil {
			t.Error("Incorrect input accepted: " + s)
		} else {
			t.Log("Expected parse error: " + err.Error())
		}
	}

}

func Test_String(t *testing.T) {
	type testcase struct {
		a, b string
	}

	for _, c := range []testcase{
		{"satoshi-1", "satoshi -1"},
		{"satoshi 1", "satoshi 1"},
		{"BTC 123456789", "BTC 123456789"},
		{"BTC 123456789.0", "BTC 123456789"},
		{"BTC 123456789.00", "BTC 123456789"},
		{"BTC 12345678.9", "BTC 12345678.9"},
		{"BTC 1234567.89", "BTC 1234567.89"},
		{"BTC 123456.789", "BTC 123456.789"},
		{"BTC 12345.6789", "BTC 12345.6789"},
		{"BTC 1234.56789", "BTC 1234.56789"},
		{"BTC 123.456789", "BTC 123.456789"},
		{"BTC 12.3456789", "BTC 12.3456789"},
		{"BTC 1.23456789", "BTC 1.23456789"},
		{"BTC 0.12345678", "mBTC 123.45678"},
		{"BTC 0.01234567", "mBTC 12.34567"},
		{"BTC 0.00123456", "mBTC 1.23456"},
		{"BTC 0.00012345", "uBTC 123.45"},
		{"BTC 0.00001234", "uBTC 12.34"},
		{"BTC 0.00000123", "uBTC 1.23"},
		{"BTC 0.00000012", "satoshi 12"},
		{"BTC 0.00000001", "satoshi 1"},
		{"BTC 0.00000000", "BTC 0"},
		{"mBTC 0", "BTC 0"},
		{"uBTC 0", "BTC 0"},
		{"BTC 0.1", "mBTC 100"},
		{"BTC 0.01", "mBTC 10"},
		{"BTC 0.001", "mBTC 1"},
		{"BTC 0.0001", "uBTC 100"},
		{"satoshi 0", "BTC 0"},
	} {
		m := new(Money)
		if err := m.Parse(c.a); err != nil {
			t.Errorf("Unexpected error parsing '%v': %v", c.a, err.Error())
			continue
		}
		r := m.String()
		if r != c.b {
			t.Errorf("'%v' incorrectly printed as '%v' (instead of '%v')", c.a, r, c.b)
		}
	}
}

func Test_JSON(t *testing.T) {
	var m Money
	for _, bs := range [][]byte{
		[]byte(`"satoshi -1"`),
		[]byte(`"satoshi 1"`),
		[]byte(`"BTC 123.456"`),
		[]byte(`"mBTC 123.456"`),
		[]byte(`"uBTC 123.45"`),
	} {
		err := json.Unmarshal(bs, &m)
		if err != nil {
			t.Errorf("Unexpected error on unmarshal: %v", err)
			continue
		}
		t.Logf("Money is now: %v", m)
		bs2, err := json.Marshal(m)
		if err != nil {
			t.Errorf("Unexpected error on marshal: %v", err)
			continue
		}

		if !bytes.Equal(bs, bs2) {
			t.Errorf("Marshaled result %v differs from original %v", string(bs2), string(bs))
		}
	}
}

func Test_formatAmount(t *testing.T) {
    type testCase struct {
        v, f int64
        r string
    }
    
    for _, c := range []testCase {
        {1, 1, "1"},
        {10, 1, "10"},
        {100, 1, "100"},
        {1000, 1, "1000"},
        {1, 1000, "0.001"},
        {10, 1000, "0.01"},
        {100, 1000, "0.1"},
        {1000, 1000, "1"},
    } {
        r := formatAmount(c.v, c.f)
        if r == c.r {
            t.Logf("formatAmount(%v, %v) correctly returned %#v", c.v, c.f, c.r)
        } else {
            t.Errorf("formatAmount(%v, %v) returned %#v instead of %#v.", c.v, c.f, r, c.r)
        }
    }
}

