// Copyright (c) 2013-2016 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaindata

import (
	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/corelog"
)

// log is a logger that is initialized with no output filters.  This
// means the package will not perform any logging by default until the caller
// requests it.
var log zerolog.Logger

// The default amount of logging is none.
func init() {
	DisableLog()
}

// DisableLog disables all library log output.  Logging output is disabled
// by default until UseLogger is called.
func DisableLog() {
	log = corelog.Disabled
}

// UseLogger uses a specified Logger to output package logging info.
func UseLogger(logger zerolog.Logger) {
	log = logger
}
