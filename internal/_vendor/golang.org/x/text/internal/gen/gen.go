// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gen contains common code for the various code generation tools in the
// text repository. Its usage ensures consistency between tools.
//
// This package defines command line flags that are common to most generation
// tools. The flags allow for specifying specific Unicode and CLDR versions
// in the public Unicode data repository (http://www.unicode.org/Public).
//
// A local Unicode data mirror can be set through the flag -local or the
// environment variable UNICODE_DIR. The former takes precedence. The local
// directory should follow the same structure as the public repository.
//
// IANA data can also optionally be mirrored by putting it in the iana directory
// rooted at the top of the local mirror. Beware, though, that IANA data is not
// versioned. So it is up to the developer to use the right version.
package gen

import (
	"flag"
	"fmt"
	"go/format"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"unicode"

	"github.com/gophersaurus/govend/internal/_vendor/golang.org/x/text/cldr"
)

var (
	url = flag.String("url",
		"http://www.unicode.org/Public",
		"URL of Unicode database directory")
	iana = flag.String("iana",
		"http://www.iana.org",
		"URL of the IANA repository")
	unicodeVersion = flag.String("unicode", unicode.Version, "unicode version to use")
	cldrVersion    = flag.String("cldr", cldr.Version, "cldr version to use")
	// Allow an environment variable to specify the local directory.
	// go generate doesn't allow specifying arguments; this is a useful
	// alternative to specifying a local mirror.
	localDir = flag.String("local",
		os.Getenv("UNICODE_DIR"),
		"directory containing local data files; for debugging only.")
)

// Init performs common initialization for a gen command. It parses the flags
// and sets up the standard logging parameters.
func Init() {
	log.SetPrefix("")
	log.SetFlags(log.Lshortfile)
	flag.Parse()
}

const header = `// This file was generated by go generate; DO NOT EDIT

package %s

`

// TODO: generate standardized version info.

// UnicodeVersion reports the requested Unicode version.
func UnicodeVersion() string {
	return *unicodeVersion
}

// UnicodeVersion reports the requested CLDR version.
func CLDRVersion() string {
	return *cldrVersion
}

// IsLocal reports whether the user specified a local directory.
func IsLocal() bool {
	return *localDir != ""
}

// OpenUCDFile opens the requested UCD file. The file is specified relative to
// the public Unicode root directory. It will call log.Fatal if there are any
// errors.
func OpenUCDFile(file string) io.ReadCloser {
	return openUnicode(path.Join(*unicodeVersion, "ucd", file))
}

// OpenCLDRCoreZip opens the CLDR core zip file. It will call log.Fatal if there
// are any errors.
func OpenCLDRCoreZip() io.ReadCloser {
	return OpenUnicodeFile("cldr", *cldrVersion, "core.zip")
}

// OpenUnicodeFile opens the requested file of the requested category from the
// root of the Unicode data archive. The file is specified relative to the
// public Unicode root directory. If version is "", it will use the default
// Unicode version. It will call log.Fatal if there are any errors.
func OpenUnicodeFile(category, version, file string) io.ReadCloser {
	if version == "" {
		version = UnicodeVersion()
	}
	return openUnicode(path.Join(category, version, file))
}

// OpenIANAFile opens the requested IANA file. The file is specified relative
// to the IANA root, which is typically either http://www.iana.org or the
// iana directory in the local mirror. It will call log.Fatal if there are any
// errors.
func OpenIANAFile(path string) io.ReadCloser {
	return Open(*iana, "iana", path)
}

// Open opens subdir/path if a local directory is specified and the file exists,
// where subdir is a directory relative to the local root, or fetches it from
// urlRoot/path otherwise. It will call log.Fatal if there are any errors.
func Open(urlRoot, subdir, path string) io.ReadCloser {
	if *localDir != "" {
		path = filepath.FromSlash(path)
		if f, err := os.Open(filepath.Join(*localDir, subdir, path)); err == nil {
			return f
		}
	}
	return get(urlRoot, path)
}

func openUnicode(path string) io.ReadCloser {
	if *localDir != "" {
		path = filepath.FromSlash(path)
		f, err := os.Open(filepath.Join(*localDir, path))
		if err != nil {
			log.Fatal(err)
		}
		return f
	}
	return get(*url, path)
}

func get(root, path string) io.ReadCloser {
	url := root + "/" + path
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("HTTP GET: %v", err)
	}
	if resp.StatusCode != 200 {
		log.Fatalf("Bad GET status for %q: %q", url, resp.Status)
	}
	return resp.Body
}

// WriteGoFile writes applies gofmt to the given bytes and writes them to a file
// with the given name. It writes a standard file comment and package statement.
// It will call log.Fatal if there are any errors.
func WriteGoFile(filename, pkg string, b []byte) {
	w, err := os.Create(filename)
	if err != nil {
		log.Fatalf("Could not create file %s: %v", filename, err)
	}
	defer w.Close()
	_, err = fmt.Fprintf(w, header, pkg)
	if err != nil {
		log.Fatalf("Error writing header: %v", err)
	}
	// Strip leading newlines.
	for len(b) > 0 && b[0] == '\n' {
		b = b[1:]
	}
	formatted, err := format.Source(b)
	if err != nil {
		// Print the original buffer even in case of an error so that the
		// returned error can be meaningfully interpreted.
		w.Write(b)
		log.Fatalf("Error formatting file %s: %v", filename, err)
	}
	if _, err := w.Write(formatted); err != nil {
		log.Fatalf("Error writing file %s: %v", filename, err)
	}
}
