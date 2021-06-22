// Copyright (c) 2014 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package jaxjson_test

import (
	"testing"

	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
)

// TestErrorCodeStringer tests the stringized output for the ErrorCode type.
func TestErrorCodeStringer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   jaxjson.ErrorCode
		want string
	}{
		{jaxjson.ErrDuplicateMethod, "ErrDuplicateMethod"},
		{jaxjson.ErrInvalidUsageFlags, "ErrInvalidUsageFlags"},
		{jaxjson.ErrInvalidType, "ErrInvalidType"},
		{jaxjson.ErrEmbeddedType, "ErrEmbeddedType"},
		{jaxjson.ErrUnexportedField, "ErrUnexportedField"},
		{jaxjson.ErrUnsupportedFieldType, "ErrUnsupportedFieldType"},
		{jaxjson.ErrNonOptionalField, "ErrNonOptionalField"},
		{jaxjson.ErrNonOptionalDefault, "ErrNonOptionalDefault"},
		{jaxjson.ErrMismatchedDefault, "ErrMismatchedDefault"},
		{jaxjson.ErrUnregisteredMethod, "ErrUnregisteredMethod"},
		{jaxjson.ErrNumParams, "ErrNumParams"},
		{jaxjson.ErrMissingDescription, "ErrMissingDescription"},
		{0xffff, "Unknown ErrorCode (65535)"},
	}

	// Detect additional error codes that don't have the stringer added.
	if len(tests)-1 != int(jaxjson.TstNumErrorCodes) {
		t.Errorf("It appears an error code was added without adding an " +
			"associated stringer test")
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		result := test.in.String()
		if result != test.want {
			t.Errorf("String #%d\n got: %s want: %s", i, result,
				test.want)
			continue
		}
	}
}

// TestError tests the error output for the Error type.
func TestError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   jaxjson.Error
		want string
	}{
		{
			jaxjson.Error{Description: "some error"},
			"some error",
		},
		{
			jaxjson.Error{Description: "human-readable error"},
			"human-readable error",
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		result := test.in.Error()
		if result != test.want {
			t.Errorf("Error #%d\n got: %s want: %s", i, result,
				test.want)
			continue
		}
	}
}
