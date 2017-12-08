# publicsuffix

Package <tt>publicsuffix</tt> provides functions to query the public suffix list found at [Public Suffix List](http://publicsuffix.org/).

When first initialised, this library uses a statically compiled list which may be out of date - callers should use Update to attempt to fetch a new
version from the official GitHub repository. Alternate data sources (such as a network share, etc) can be used by implementing the ListRetriever interface.

A list can be serialised using Write, and loaded using Read - this allows the caller to write the updated internal list to disk at shutdown and resume
using it immediately on the next start.

All exported functions are concurrency safe and the internal list uses copy-on-write during updates to avoid blocking queries.

## Installation

```shell
$ go get github.com/globalsign/publicsuffix
```

## Usage

Example to demonstrate how to use the package.
```go
package main

import (
	"fmt"
	"os"

	"github.com/globalsign/publicsuffix"
)

func Example() {
	// Update to get the latest version of the Public Suffix List from the github repository
	if err := publicsuffix.Update(); err != nil {
		panic(err.Error())
	}

	// Check the list version
	var release = publicsuffix.Release()
	fmt.Printf("Public Suffix List release: %s", release)

	// Check if a given domain is in the Public Suffix List
	if publicsuffix.HasPublicSuffix("example.domain.com") {
		// Do something
	}

	// Get public suffix
	var suffix, icann = publicsuffix.PublicSuffix("another.example.domain.com")
	fmt.Printf("suffix: %s, icann: %v", suffix, icann)

	// Write the current Public List to a file
	var file, err = os.Create("list_backup")
	if err != nil {
		panic(err.Error())
	}
	defer file.Close()

	if err := publicsuffix.Write(file); err != nil {
		panic(err.Error())
	}

	// Read and load a list from a file
	file, err = os.Open("list_backup")
	if err != nil {
		panic(err.Error())
	}

	if err := publicsuffix.Read(file); err != nil {
		panic(err.Error())
	}

}
```

## Algorithm

Algorithm follows the steps defined by the [Public Suffix List](https://publicsuffix.org/list/).

1. Match domain against all rules and take note of the matching ones.
2. If no rules match, the prevailing rule is "*".
3. If more than one rule matches, the prevailing rule is the one which is an exception rule.
4. If there is no matching exception rule, the prevailing rule is the one with the most labels.
5. If the prevailing rule is a exception rule, modify it by removing the leftmost label.
6. The public suffix is the set of labels from the domain which match the labels of the prevailing rule, using the matching algorithm above.
7. The registered or registrable domain is the public suffix plus one additional label.

The list is parsed to build an internal map. It uses the concatenated name of the rule (no dots) as key and a slice of structs for all matching rules containing:
- Dotted name
- Rule type (normal, wildcard [*.], or exception [!])
- ICANN flag: indicates if the rule is within the ICANN delimiters in the list

The input domain is decomposed in all possible subdomains.
```Example:
// Input domain
example.blogspot.co.uk
// Decomposed 
"example.blogspot.co.uk", "blogspot.co.uk", "co.uk", "uk"
```

All options are then used in decreasing order to search in the map, so the subdomain with most levels has matching priority. If several rules match the map key, only the one
matching the dotted name will be considered a valid match. 
```Example:
// Domains in the list
i.ng
ing
```
They will both be stored in the internal map under key "ing", however the rule will only match if it actually contains the dotted name. 
```
map["ing"] = {{DottedName: "i.ng", RuleType: normal, ICANN: true}, 
			  {DottedName: "ing", RuleType: normal, ICANN: true}}
```

## Differences with `golang.org/x/net/publicsuffix` and `github.com/weppos/publicsuffix-go`

The idea behind this implementation is to mirror the behaviour of the existing golang library adding the flexibility weppos one introduces and maintaining a good performance. 

Main features of this library are:
 - Start up initialisation of a static list
 - Ability to update the internal list with latest release of github repository
 - Support for alternate data sources
 - Ability to write/read the list for efficient shutdown/start up loading times
 - Concurrency safe

Benchmark comparison between the three libraries, can be found in /publicsuffix/publicsuffix_test.go:
```
Benchmark values for this library
BenchmarkPublicSuffix1-8 3000000 514 ns/op 64 B/op 5 allocs/op
BenchmarkPublicSuffix2-8 2000000 748 ns/op 192 B/op 7 allocs/op
BenchmarkPublicSuffix3-8 5000000 372 ns/op 64 B/op 3 allocs/op
BenchmarkPublicSuffix4-8 3000000 543 ns/op 96 B/op 5 allocs/op
BenchmarkPublicSuffix5-8 3000000 565 ns/op 96 B/op 5 allocs/op
BenchmarkPublicSuffix6-8 2000000 678 ns/op 122 B/op 6 allocs/op
BenchmarkPublicSuffix7-8 2000000 700 ns/op 160 B/op 7 allocs/op
 
Benchmark values for `golang.org/x/net/publicsuffix` 
BenchmarkPublicSuffixNet1-8 10000000 218 ns/op 0 B/op 0 allocs/op
BenchmarkPublicSuffixNet2-8 5000000 265 ns/op 0 B/op 0 allocs/op
BenchmarkPublicSuffixNet3-8 10000000 181 ns/op 0 B/op 0 allocs/op
BenchmarkPublicSuffixNet4-8 10000000 133 ns/op 0 B/op 0 allocs/op
BenchmarkPublicSuffixNet5-8 10000000 141 ns/op 0 B/op 0 allocs/op
BenchmarkPublicSuffixNet6-8 10000000 195 ns/op 0 B/op 0 allocs/op
BenchmarkPublicSuffixNet7-8 5000000 238 ns/op 0 B/op 0 allocs/op
 
Benchmark values for `github.com/weppos/publicsuffix-go`
BenchmarkPublicSuffixWeppos1-8 10000 146959 ns/op 45221 B/op 70 allocs/op
BenchmarkPublicSuffixWeppos2-8 10000 154362 ns/op 47761 B/op 72 allocs/op
BenchmarkPublicSuffixWeppos3-8 10000 146561 ns/op 44782 B/op 66 allocs/op
BenchmarkPublicSuffixWeppos4-8 10000 156349 ns/op 44846 B/op 66 allocs/op
BenchmarkPublicSuffixWeppos5-8 10000 161006 ns/op 44846 B/op 66 allocs/op
BenchmarkPublicSuffixWeppos6-8 10000 158021 ns/op 47409 B/op 74 allocs/op
BenchmarkPublicSuffixWeppos7-8 10000 156046 ns/op 47745 B/op 73 allocs/op
```

## `cookiejar.PublicSuffixList` interface

This package implements the [`cookiejar.PublicSuffixList` interface](https://godoc.org/net/http/cookiejar#PublicSuffixList). It means it can be used as a value for the `PublicSuffixList` option when creating a `net/http/cookiejar`.

```go
import (
    "net/http/cookiejar"
    "github.com/globalsign/publicsuffix"
)

jar := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.CookieJarList})
```
