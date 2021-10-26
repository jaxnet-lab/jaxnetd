// Copyright (c) 2013, 2014 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package jaxutil

import (
	"errors"
	"math"
	"strconv"

	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
)

type IAmount interface {
	String() string
	ToInt() int64
	ToCoin() float64
}

func IAmountVal(val int64, beacon bool) IAmount {
	if beacon {
		return JaxNetAmount(val)
	}

	return JaxAmount(val)
}

func NewIAmount(f float64, beacon bool) (IAmount, error) {
	if beacon {
		return NewJaxNetAmount(f)
	}

	return NewJaxAmount(f)
}

// JXNAmountUnit describes a method of converting an Amount to something
// other than the base unit of a bitcoin.  The value of the JXNAmountUnit
// is the exponent component of the decadic multiple to convert from
// an amount in bitcoin to an amount counted in units.
type JXNAmountUnit int

const (
	AmountMegaJXN        JXNAmountUnit = 6
	AmountKiloJXN        JXNAmountUnit = 3
	AmountJXN            JXNAmountUnit = 0
	AmountMilliJXN       JXNAmountUnit = -3
	AmountMicroJXN       JXNAmountUnit = -6
	AmountHaberStornetta JXNAmountUnit = -8
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

type JaxNetAmount int64

func NewJaxNetAmount(f float64) (JaxNetAmount, error) {
	// The amount is only considered invalid if it cannot be represented
	// as an integer type.  This may happen if f is NaN or +-Infinity.
	switch {
	case math.IsNaN(f):
		fallthrough
	case math.IsInf(f, 1):
		fallthrough
	case math.IsInf(f, -1):
		return 0, errors.New("invalid JXN amount")
	}

	return JaxNetAmount(math.Round(f * chaincfg.HaberStornettaPerJAXNETCoin)), nil
}

// ToUnit converts a monetary amount counted in bitcoin base units to a
// floating point value representing an amount of bitcoin.
func (a JaxNetAmount) ToUnit(u JXNAmountUnit) float64 {
	return float64(a) / math.Pow10(int(u+8))
}

// Format formats a monetary amount counted in bitcoin base units as a
// string for a given unit.  The conversion will succeed for any unit,
// however, known units will be formated with an appended label describing
// the units with SI notation, or "Satoshi" for the base unit.
func (a JaxNetAmount) Format(u JXNAmountUnit) string {
	units := " " + u.String()
	return strconv.FormatFloat(a.ToUnit(u), 'f', -int(u+8), 64) + units
}

func (a JaxNetAmount) ToInt() int64 {
	return int64(a)
}

func (a JaxNetAmount) ToCoin() float64 {
	return float64(a) / chaincfg.HaberStornettaPerJAXNETCoin
}

// String is the equivalent of calling Format with AmountBTC.
func (a JaxNetAmount) String() string {
	return a.Format(AmountJXN)
}

// JAXAmountUnit describes a method of converting an Amount to something
// other than the base unit of a bitcoin.  The value of the JXNAmountUnit
// is the exponent component of the decadic multiple to convert from
// an amount in bitcoin to an amount counted in units.
type JAXAmountUnit int

// These constants define various units used when describing a bitcoin
// monetary amount.
const (
	AmountMegaJAX  JAXAmountUnit = 6
	AmountKiloJAX  JAXAmountUnit = 3
	AmountJAX      JAXAmountUnit = 0
	AmountMilliJAX JAXAmountUnit = -3
	AmountJuro     JAXAmountUnit = -4
)

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

type JaxAmount int64

func NewJaxAmount(f float64) (JaxAmount, error) {
	// The amount is only considered invalid if it cannot be represented
	// as an integer type.  This may happen if f is NaN or +-Infinity.
	switch {
	case math.IsNaN(f):
		fallthrough
	case math.IsInf(f, 1):
		fallthrough
	case math.IsInf(f, -1):
		return 0, errors.New("invalid JXN amount")
	}

	return JaxAmount(math.Round(f * chaincfg.JuroPerJAXCoin)), nil
}

// ToUnit converts a monetary amount counted in bitcoin base units to a
// floating point value representing an amount of bitcoin.
func (a JaxAmount) ToUnit(u JAXAmountUnit) float64 {
	return float64(a) / math.Pow10(int(u+4))
}

// Format formats a monetary amount counted in bitcoin base units as a
// string for a given unit.  The conversion will succeed for any unit,
// however, known units will be formated with an appended label describing
// the units with SI notation, or "Satoshi" for the base unit.
func (a JaxAmount) Format(u JAXAmountUnit) string {
	units := " " + u.String()
	return strconv.FormatFloat(a.ToUnit(u), 'f', -int(u+8), 64) + units
}

func (a JaxAmount) ToInt() int64 {
	return int64(a)
}

func (a JaxAmount) ToCoin() float64 {
	return float64(a) / chaincfg.JuroPerJAXCoin
}

// String is the equivalent of calling Format with AmountBTC.
func (a JaxAmount) String() string {
	return a.Format(AmountJAX)
}
