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
	defer file.Close()

	if err := publicsuffix.Read(file); err != nil {
		panic(err.Error())
	}
}
```

## Algorithm

The algorithm follows the steps defined by the [Public Suffix List](https://publicsuffix.org/list/).

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

## Why another publicsuffix?

This implementation aims for performance close to `golang.org/x/net/publicsuffix` whilst adding the capability to update the PSL similar to `github.com/weppos/publicsuffix-go`

Main features of this library are:
 - Start up initialisation of a static list
 - Ability to update the internal list with latest release of github repository
 - Support for alternate data sources
 - Ability to write/read the list for efficient shutdown/start up loading times
 - Concurrency safe

Benchmark comparison between the three libraries, can be found in /publicsuffix/publicsuffix_test.go:

##### `golang.org/x/net/publicsuffix` vs this library
```
benchmark                    old ns/op     new ns/op     delta
BenchmarkPublicSuffix1-8     210           415           +97.62%
BenchmarkPublicSuffix2-8     246           618           +151.22%
BenchmarkPublicSuffix3-8     174           295           +69.54%
BenchmarkPublicSuffix4-8     130           443           +240.77%
BenchmarkPublicSuffix5-8     138           446           +223.19%
BenchmarkPublicSuffix6-8     189           427           +125.93%
BenchmarkPublicSuffix7-8     234           600           +156.41%

benchmark                    old allocs     new allocs     delta
BenchmarkPublicSuffix1-8     0              5              +Inf%
BenchmarkPublicSuffix2-8     0              7              +Inf%
BenchmarkPublicSuffix3-8     0              3              +Inf%
BenchmarkPublicSuffix4-8     0              5              +Inf%
BenchmarkPublicSuffix5-8     0              5              +Inf%
BenchmarkPublicSuffix6-8     0              5              +Inf%
BenchmarkPublicSuffix7-8     0              7              +Inf%

benchmark                    old bytes     new bytes     delta
BenchmarkPublicSuffix1-8     0             64            +Inf%
BenchmarkPublicSuffix2-8     0             192           +Inf%
BenchmarkPublicSuffix3-8     0             64            +Inf%
BenchmarkPublicSuffix4-8     0             96            +Inf%
BenchmarkPublicSuffix5-8     0             96            +Inf%
BenchmarkPublicSuffix6-8     0             74            +Inf%
BenchmarkPublicSuffix7-8     0             160           +Inf%
```

##### `github.com/weppos/publicsuffix-go` vs this library
```
benchmark                    old ns/op     new ns/op     delta
BenchmarkPublicSuffix1-8     135465        415           -99.69%
BenchmarkPublicSuffix2-8     138498        618           -99.55%
BenchmarkPublicSuffix3-8     133217        295           -99.78%
BenchmarkPublicSuffix4-8     140399        443           -99.68%
BenchmarkPublicSuffix5-8     141108        446           -99.68%
BenchmarkPublicSuffix6-8     139077        427           -99.69%
BenchmarkPublicSuffix7-8     139605        600           -99.57%

benchmark                    old allocs     new allocs     delta
BenchmarkPublicSuffix1-8     70             5              -92.86%
BenchmarkPublicSuffix2-8     72             7              -90.28%
BenchmarkPublicSuffix3-8     66             3              -95.45%
BenchmarkPublicSuffix4-8     66             5              -92.42%
BenchmarkPublicSuffix5-8     66             5              -92.42%
BenchmarkPublicSuffix6-8     74             5              -93.24%
BenchmarkPublicSuffix7-8     73             7              -90.41%

benchmark                    old bytes     new bytes     delta
BenchmarkPublicSuffix1-8     45211         64            -99.86%
BenchmarkPublicSuffix2-8     47749         192           -99.60%
BenchmarkPublicSuffix3-8     44771         64            -99.86%
BenchmarkPublicSuffix4-8     44834         96            -99.79%
BenchmarkPublicSuffix5-8     44834         96            -99.79%
BenchmarkPublicSuffix6-8     47395         74            -99.84%
BenchmarkPublicSuffix7-8     47732         160           -99.66%
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
