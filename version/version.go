/*
 * Copyright (c) 2020 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package version

import (
	"bytes"
	"fmt"
	"strings"
)

// semanticAlphabet
const semanticAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"

// These constants define the application version and follow the semantic
// versioning 2.0.0 spec (http://semver.org/).
const (
	appMajor uint = 0
	appMinor uint = 4
	appPatch uint = 9

	// appPreRelease MUST only contain characters from semanticAlphabet
	// per the semantic versioning spec.
	appPreRelease = ""
)

var (
	tag     = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

// appBuild is defined as a variable so it can be overridden during the build
// process with '-ldflags "-X main.appBuild foo' if needed.  It MUST only
// contain characters from semanticAlphabet per the semantic versioning spec.
var appBuild string

// GetVersion returns the application version as a properly formed string per the
// semantic versioning 2.0.0 spec (http://semver.org/).
func GetVersion() string {
	// Start with the major, minor, and patch versions.
	version := fmt.Sprintf("%d.%d.%d", appMajor, appMinor, appPatch)

	// Append pre-release version if there is one.  The hyphen called for
	// by the semantic versioning spec is automatically appended and should
	// not be contained in the pre-release string.  The pre-release version
	// is not appended if it contains invalid characters.
	preRelease := normalizeVerString(appPreRelease)
	if preRelease != "" {
		version = fmt.Sprintf("%s-%s", version, preRelease)
	}

	// Append build metadata if there is any.  The plus called for
	// by the semantic versioning spec is automatically appended and should
	// not be contained in the build metadata string.  The build metadata
	// string is not appended if it contains invalid characters.
	build := normalizeVerString(appBuild)
	if build != "" {
		version = fmt.Sprintf("%s+%s", version, build)
	}

	return version
}

type Ver struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Tag     string `json:"tag"`
	Date    string `json:"date"`
	BuiltBy string `json:"builtBy"`
}

func GetExtendedVersion() Ver {
	return Ver{
		Version: GetVersion(),
		Commit:  commit,
		Tag:     tag,
		Date:    date,
		BuiltBy: builtBy,
	}
}

// normalizeVerString returns the passed string stripped of all characters which
// are not valid according to the semantic versioning guidelines for pre-release
// version and build metadata strings.  In particular they MUST only contain
// characters in semanticAlphabet.
func normalizeVerString(str string) string {
	var result bytes.Buffer
	for _, r := range str {
		if strings.ContainsRune(semanticAlphabet, r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}
