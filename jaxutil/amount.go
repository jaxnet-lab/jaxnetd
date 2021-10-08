// Copyright (c) 2013, 2014 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package jaxutil

import (
	"errors"
	"math"
	"strconv"
)

// JXNAmountUnit describes a method of converting an Amount to something
// other than the base unit of a bitcoin.  The value of the JXNAmountUnit
// is the exponent component of the decadic multiple to convert from
// an amount in bitcoin to an amount counted in units.
type JXNAmountUnit int
type JAXAmountUnit int

// These constants define various units used when describing a bitcoin
// monetary amount.
const (
	AmountMegaJXN        JXNAmountUnit = 6
	AmountKiloJXN        JXNAmountUnit = 3
	AmountJXN            JXNAmountUnit = 0
	AmountMilliJXN       JXNAmountUnit = -3
	AmountMicroJXN       JXNAmountUnit = -6
	AmountHaberStornetta JXNAmountUnit = -8

	AmountMegaJAX  JAXAmountUnit = 6
	AmountKiloJAX  JAXAmountUnit = 3
	AmountJAX      JAXAmountUnit = 0
	AmountMilliJAX JAXAmountUnit = -3
	AmountJuro     JAXAmountUnit = -4
)

// String returns the unit as a string.  For recognized units, the SI
// prefix is used, or "Satoshi" for the base unit.  For all unrecognized
// units, "1eN BTC" is returned, where N is the JXNAmountUnit.
func (u JXNAmountUnit) String() string {
	switch u {
	case AmountMegaJXN:
		return "MJXN"
	case AmountKiloJXN:
		return "kJXN"
	case AmountJXN:
		return "JXN"
	case AmountMilliJXN:
		return "mJXN"
	case AmountMicroJXN:
		return "μJXN"
	case AmountHaberStornetta:
		return "HaberStornetta"

	default:
		return "1e" + strconv.FormatInt(int64(u), 10) + " JXN"
	}
}

// String returns the unit as a string.  For recognized units, the SI
// prefix is used, or "Satoshi" for the base unit.  For all unrecognized
// units, "1eN BTC" is returned, where N is the JXNAmountUnit.
func (u JAXAmountUnit) String() string {
	switch u {
	case AmountMegaJAX:
		return "MJAX"
	case AmountKiloJAX:
		return "kJAX"
	case AmountJAX:
		return "JAX"
	case AmountMilliJAX:
		return "mJAX"
	case AmountJuro:
		return "Juro"

	default:
		return "1e" + strconv.FormatInt(int64(u), 10) + " JAX"
	}
}

// Amount represents the base bitcoin monetary unit (colloquially referred
// to as a `Satoshi').  A single Amount is equal to 1e-8 of a bitcoin.
type Amount int64

// round converts a floating point number, which may or may not be representable
// as an integer, to the Amount integer type by rounding to the nearest integer.
// This is performed by adding or subtracting 0.5 depending on the sign, and
// relying on integer truncation to round the value to the nearest Amount.
func round(f float64) Amount {
	if f < 0 {
		return Amount(f - 0.5)
	}
	return Amount(f + 0.5)
}

// NewAmount creates an Amount from a floating point value representing
// some value in bitcoin.  NewAmount errors if f is NaN or +-Infinity, but
// does not check that the amount is within the total amount of bitcoin
// producible as f may not refer to an amount at a single moment in time.
//
// NewAmount is for specifically for converting BTC to Satoshi.
// For creating a new Amount with an int64 value which denotes a quantity of Satoshi,
// do a simple type conversion from type int64 to Amount.
// See GoDoc for example: http://godoc.org/gitlab.com/jaxnet/jaxnetd/btcutil#example-Amount
func NewAmount(f float64) (Amount, error) {
	// The amount is only considered invalid if it cannot be represented
	// as an integer type.  This may happen if f is NaN or +-Infinity.
	switch {
	case math.IsNaN(f):
		fallthrough
	case math.IsInf(f, 1):
		fallthrough
	case math.IsInf(f, -1):
		return 0, errors.New("invalid bitcoin amount")
	}

	return round(f * SatoshiPerBitcoin), nil
}

// ToUnit converts a monetary amount counted in bitcoin base units to a
// floating point value representing an amount of bitcoin.
func (a Amount) ToUnit(u JXNAmountUnit) float64 {
	return float64(a) / math.Pow10(int(u+8))
}

// ToBTC is the equivalent of calling ToUnit with AmountBTC.
func (a Amount) ToBTC() float64 {
	return a.ToUnit(AmountJXN)
}

// ToJax ...
func (a Amount) ToJax() float64 {
	return float64(a) / 10_000
}

// ToJaxNet ...
func (a Amount) ToJaxNet() float64 {
	return float64(a) / 100_000_000
}

func (a Amount) ToCoin(isBeacon bool) float64 {
	if isBeacon {
		return a.ToJaxNet()
	}
	return a.ToJax()
}

// Format formats a monetary amount counted in bitcoin base units as a
// string for a given unit.  The conversion will succeed for any unit,
// however, known units will be formated with an appended label describing
// the units with SI notation, or "Satoshi" for the base unit.
func (a Amount) Format(u JXNAmountUnit) string {
	units := " " + u.String()
	return strconv.FormatFloat(a.ToUnit(u), 'f', -int(u+8), 64) + units
}

// String is the equivalent of calling Format with AmountBTC.
func (a Amount) String() string {
	return a.Format(AmountJXN)
}

// MulF64 multiplies an Amount by a floating point value.  While this is not
// an operation that must typically be done by a full node or wallet, it is
// useful for services that build on top of bitcoin (for example, calculating
// a fee by multiplying by a percentage).
func (a Amount) MulF64(f float64) Amount {
	return round(float64(a) * f)
}
