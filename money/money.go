//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2013-2019  Jonas Eschenburg <jonas@bitwrk.net>
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

// Package money provides a well-tested framework to parse, format and calculate
// with monetary amounts, principally Bitcoin.
package money

import (
	"errors"
	"fmt"
	"regexp"
)

func init() {
	for _, u := range units {
		unitsBySymbol[u.symbol] = u
		list, ok := unitsByCurrency[u.currency]
		if !ok {
			list = make([]Unit, 0, 4)
		}
		list = append(list, u)
		unitsByCurrency[u.currency] = list
	}
}

type Currency int

func (c Currency) DefaultUnit() *Unit {
	if list, ok := unitsByCurrency[c]; ok {
		u := list[0]
		return &u
	}
	return nil
}

const (
	BTC Currency = iota
	EUR
	USD
	BRL
	GBP
)

func (c Currency) String() string {
	switch c {
	case BTC:
		return "BTC"
	case EUR:
		return "EUR"
	case USD:
		return "USD"
	case BRL:
		return "BRL"
	case GBP:
		return "GBP"
	}
	return fmt.Sprintf("<unknown currency:%d>", int(c))
}

func (c *Currency) Parse(s string) error {
	switch s {
	case "BTC":
		*c = BTC
	case "EUR":
		*c = EUR
	case "USD":
		*c = USD
	case "BRL":
		*c = BRL
	case "GBP":
		*c = GBP
	default:
		return errors.New("Unknown currency: " + s)
	}
	return nil
}

func (c *Currency) MustParse(s string) {
	err := c.Parse(s)
	if err != nil {
		panic(err)
	}
}

type Unit struct {
	symbol   string
	currency Currency
	// The sub-unit's value in multiples of the base unit
	factor int64
	// Whether this unit will be auto-selected for formatting
	selectable bool
}

func (u Unit) String() string {
	return u.symbol
}

func ParseUnit(symbol string) (Unit, error) {
	if u, ok := unitsBySymbol[symbol]; ok {
		return u, nil
	} else {
		return Unit{}, errors.New("No such unit: " + symbol)
	}
}

func MustParseUnit(symbol string) Unit {
	if u, err := ParseUnit(symbol); err != nil {
		panic(err)
	} else {
		return u
	}
}

var units = [...]Unit{
	{"BTC", BTC, 100000000, true},
	{"mBTC", BTC, 100000, true},
	{"uBTC", BTC, 100, true},
	{"satoshi", BTC, 1, false},
	{"EUR", EUR, 1000000000, true},
	{"€", EUR, 1000000000, false},
	{"USD", USD, 1000000000, true},
	{"$", USD, 1000000000, false},
	{"BRL", BRL, 1000000000, true},
	{"R$", BRL, 1000000000, false},
	{"GBP", GBP, 1000000000, true},
	{"£", GBP, 1000000000, false},
}

var unitsBySymbol = make(map[string]Unit)
var unitsByCurrency = make(map[Currency][]Unit)

type Money struct {
	Amount   int64
	Currency Currency
}

var pattern = regexp.MustCompile(`^([A-Za-z€$£]+) ?(-)?([0-9]+)(?:\.([0-9]+))?$`)

func (m *Money) Parse(s string) error {
	matches := pattern.FindStringSubmatch(s)
	if matches == nil {
		return errors.New("Input doesn't match pattern for monetary amounts: " + s)
	}
	symbol := matches[1]
	unit, ok := unitsBySymbol[symbol]
	if !ok {
		return errors.New("Unsupported currency: " + symbol)
	}

	sign := 1
	if matches[2] == "-" {
		sign = -1
	}

	var result int64 = 0

	fractional := matches[4]
	for base, i := unit.factor, 0; i < len(fractional); i++ {
		base /= 10
		if base == 0 {
			return errors.New("Too many digits in fractional part of " + s)
		}
		result += base * int64(fractional[i]-'0')
	}

	integral := matches[3]
	for base, i := unit.factor, len(integral)-1; i >= 0; i-- {
		c := integral[i]
		result += base * int64(c-'0')
		base *= 10
		if base >= 1000000000000000000 {
			return errors.New("Too many digits in integral part of " + s)
		}
	}

	m.Amount = int64(sign) * result
	m.Currency = unit.currency
	return nil
}

func MustParse(s string) Money {
	var m Money
	if err := m.Parse(s); err != nil {
		panic(err)
	}
	return m
}

// Formats a valid string representation of the amount, including symbol
func (m Money) String() string {
	return m.Format(m.SelectUnit(), true)
}

// Selects a unit in a suitable scale for formatting
func (m Money) SelectUnit() Unit {
	units, ok := unitsByCurrency[m.Currency]
	if !ok {
		panic(fmt.Sprintf("No unit found for currency: %v", m.Currency))
	}

	v := m.Amount
	if v < 0 {
		v = -v
	} else if v == 0 {
		unit := m.Currency.DefaultUnit()
		if unit == nil {
			panic(fmt.Sprintf("No default unit found for currency: %v", m.Currency))
		}
		return *unit
	}

	// Find the largest selectable unit that will produce an amount
	// without leading zero. If not possible, fall back to the smallest
	// selectable unit.
	var result Unit
	found := false
	for _, unit := range units {
		if unit.selectable {
			result = unit
			found = true
		}
		if unit.factor <= v && found {
			break
		}
	}

	if !found {
		panic("No selectable unit found")
	}

	return result
}

// Formats in a specific unit scale
func (m Money) Format(unit Unit, includeSymbol bool) string {
	if m.Currency != unit.currency {
		panic(fmt.Sprintf("Currencies don't match: [%v] [%v -> %v]", m.Currency, unit.symbol, unit.currency))
	}

	v := m.Amount
	sign := ""
	if v < 0 {
		v = -v
		sign = "-"
	}
	if includeSymbol {
		return unit.symbol + " " + sign + formatAmount(v, unit.factor)
	} else {
		return sign + formatAmount(v, unit.factor)
	}
}

// Formats a positive value
func formatAmount(v, f int64) string {
	result := make([]byte, 0, 64)

	intPart := v / f
	fracPart := v - intPart*f

	result = append(result, fmt.Sprintf("%v", intPart)...)
	if fracPart == 0 {
		return string(result)
	}

	result = append(result, '.')

	for b := f / 10; b > 0 && fracPart > 0; b /= 10 {
		digit := fracPart / b
		result = append(result, byte('0'+digit))
		fracPart -= digit * b
	}

	return string(result)
}

func (m Money) MarshalJSON() ([]byte, error) {
	return []byte("\"" + m.String() + "\""), nil
}

func (m *Money) UnmarshalJSON(data []byte) error {
	// Verify it's a string
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("Illegal monetary value: %v", data)
	}
	// Parse the part between the quotes
	return m.Parse(string(data[1 : len(data)-1]))
}

func (a Money) Add(b Money) (r Money) {
	if a.Currency != b.Currency {
		panic("Currencies don't match in Add()")
	}
	r.Currency = a.Currency
	r.Amount = a.Amount + b.Amount
	return
}

func (a Money) Sub(b Money) (r Money) {
	if a.Currency != b.Currency {
		panic("Currencies don't match in Sub()")
	}
	r.Currency = a.Currency
	r.Amount = a.Amount - b.Amount
	return
}

func (a Money) Neg() (r Money) {
	r.Currency = a.Currency
	r.Amount = -a.Amount
	return
}

func Min(a Money, b Money) (r Money) {
	if a.Currency != b.Currency {
		panic("Currencies don't match in Min()")
	}
	r.Currency = a.Currency
	r.Amount = a.Amount
	if b.Amount < a.Amount {
		r.Amount = b.Amount
	}
	return
}

func Max(a Money, b Money) (r Money) {
	if a.Currency != b.Currency {
		panic("Currencies don't match in Max()")
	}
	r.Currency = a.Currency
	r.Amount = a.Amount
	if b.Amount > a.Amount {
		r.Amount = b.Amount
	}
	return
}
