// Copyright (c) 2014-2017 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpcclient

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
// nolint
func UseLogger(logger zerolog.Logger) {
	log = logger
}

// LogClosure is a closure that can be printed with %v to be used to
// generate expensive-to-create data for a detailed log level and avoid doing
// the work if the data isn't printed.
// nolint
type logClosure func() string

// nolint
// String invokes the log closure and returns the results string.
func (c logClosure) String() string {
	return c()
}
