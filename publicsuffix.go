// Package publicsuffix provides functions to query the public suffix list found
// at:
//
// 		https://publicsuffix.org/
//
// When first initialised, this library uses a statically compiled list which
// may be out of date - callers should use Update to attempt to fetch a new
// version from the official GitHub repository. Alternate data sources (such as
// a network share, etc) can be used by implementing the ListRetriever
// interface.
//
// A list can be serialised using Write, and loaded using Read - this allows the
// caller to write the updated internal list to disk at shutdown and resume
// using it immediately on the next start.
//
// All exported functions are concurrency safe and the internal list uses
// copy-on-write during updates to avoid blocking queries.
package publicsuffix

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/net/idna"
)

// rulesInfo contains the map of rules and the commit version that generated them
type rulesInfo struct {
	Map     map[string][]rule
	Release string
}

// rule contains the data related to a domain from the PSL
type rule struct {
	DottedName string
	RuleType   ruleType
	ICANN      bool
}

type subdomain struct {
	name       string
	dottedName string
}

// ruleType encapsulates integer for enum
type ruleType int

const (
	normal ruleType = iota
	wildcard
	exception
)

// icannBegin marks the beginning of ICANN domains in the public suffix list
// source file.
const icannBegin = "BEGIN ICANN DOMAINS"

// icannEnd marks the ending of ICANN domains in the public suffix list
// source file.
const icannEnd = "END ICANN DOMAINS"

var (
	// validSuffixRE is used to check that the entries in the public suffix
	// list are in canonical form (after Punycode encoding). Specifically,
	// capital letters are not allowed.
	validSuffixRE = regexp.MustCompile(`^[a-z0-9_\!\*\-\.]+$`)

	// rules caches the PSL from the last commit available
	// handles read/write concurrency
	rules atomic.Value

	// subdomainPool pools subdomain arrays to avoid reallocation cost
	subdomainPool = sync.Pool{
		New: func() interface{} {
			// 5 should cover the average domain
			return make([]subdomain, 5)
		},
	}
)

func init() {
	if err := Read(bytes.NewReader(listBytes)); err != nil {
		panic(fmt.Sprintf("error while initialising Public Suffix List from list.go: %s", err.Error()))
	}

	// not used after initialisation, set to nil for garbage collector
	listBytes = nil
}

func load() rulesInfo {
	return rules.Load().(rulesInfo)
}

// Write atomically encodes the currently loaded public suffix list as JSON and compresses and
// writes it to w.
func Write(w io.Writer) error {
	// Wrap w in zlib Writer
	var zlibWriter = zlib.NewWriter(w)
	defer zlibWriter.Close()

	// Encode directly into the zlib writer, which in turn writes into w.
	return json.NewEncoder(zlibWriter).Encode(load())
}

// Read loads a public suffix list serialised and compressed by Write and uses it for future
// lookups.
func Read(r io.Reader) error {
	var zlibReader, err = zlib.NewReader(r)
	if err != nil {
		return fmt.Errorf("zlib error: %s", err.Error())
	}
	defer zlibReader.Close()

	var tempRulesInfo = rulesInfo{}
	if err := json.NewDecoder(zlibReader).Decode(&tempRulesInfo); err != nil {
		return fmt.Errorf("json error: %s", err.Error())
	}

	rules.Store(tempRulesInfo)

	return nil
}

// Update fetches the latest public suffix list from the official github
// repository and uses it for future lookups.
//
// 		https://github.com/publicsuffix/list
//
func Update() error {
	return UpdateWithListRetriever(gitHubListRetriever{})
}

// UpdateWithListRetriever attempts to update the internal public suffix list
// using listRetriever as a data source.
//
// UpdateWithListRetriever is provided to allow callers to provide custom update
// sources, such as reading from a network store or local cache instead of
// fetching from the GitHub repository.
func UpdateWithListRetriever(listRetriever ListRetriever) error {
	var latestTag, err = listRetriever.GetLatestReleaseTag()
	if err != nil {
		return fmt.Errorf("error while retrieving last commit information: %s", err.Error())
	}

	if load().Release == latestTag {
		return nil
	}

	var rawList io.Reader
	rawList, err = listRetriever.GetList(latestTag)
	if err != nil {
		return fmt.Errorf("error while retrieving Public Suffix List last release (%s): %s", latestTag, err.Error())
	}

	var rulesInfo *rulesInfo
	rulesInfo, err = newList(rawList, latestTag)
	if err != nil {
		return err
	}

	rules.Store(*rulesInfo)

	return nil
}

// HasPublicSuffix returns true if the TLD of domain is in the public suffix
// list.
func HasPublicSuffix(domain string) bool {
	var _, _, found = searchList(domain)

	return found
}

// PublicSuffix returns the public suffix of the domain using a copy of the
// internal public suffix list.
//
// The returned bool is true when the public suffix is managed by the Internet
// Corporation for Assigned Names and Numbers. If false, the public suffix is
// privately managed. For example, foo.org and foo.co.uk are ICANN domains,
// foo.dyndns.org and foo.blogspot.co.uk are private domains.
func PublicSuffix(domain string) (string, bool) {
	var publicsuffix, icann, _ = searchList(domain)

	return publicsuffix, icann
}

// EffectiveTLDPlusOne returns the effective top level domain plus one more
// label. For example, the eTLD+1 for "foo.bar.golang.org" is "golang.org".
func EffectiveTLDPlusOne(domain string) (string, error) {
	var suffix, _ = PublicSuffix(domain)

	if len(domain) <= len(suffix) {
		return "", fmt.Errorf("publicsuffix: cannot derive eTLD+1 for domain %q", domain)
	}

	var i = len(domain) - len(suffix) - 1
	if domain[i] != '.' {
		return "", fmt.Errorf("publicsuffix: invalid public suffix %q for domain %q", suffix, domain)
	}

	return domain[1+strings.LastIndex(domain[:i], "."):], nil
}

// Release returns the release of the current internal public suffix list.
func Release() string {
	return load().Release
}

// searchList looks for the given domain in the Public Suffix List and returns
// the suffix, a flag indicating if it's managed by the Internet Corporation,
// and a flag indicating if it was found in the list
func searchList(domain string) (string, bool, bool) {
	// If the domain ends on a dot the subdomains can't be obtained - no PSL applicable
	if strings.LastIndex(domain, ".") == len(domain)-1 {
		return "", false, false
	}

	var buffer = subdomainPool.Get().([]subdomain)[:0]
	var subdomains = decomposeDomain(domain, buffer)
	defer subdomainPool.Put(subdomains)

	var rulesInfo = load()
	var match = false

	// the longest matching rule (the one with the most levels) will be used
	for _, sub := range subdomains {
		var rules, found = rulesInfo.Map[sub.name]
		if !found {
			continue
		}

		// Look for all the rules matching the concatenated name
		for _, rule := range rules {
			match = true

			switch rule.RuleType {
			case wildcard:
				// first check if the rule is contained within the domain without the *.
				if !strings.HasSuffix(sub.dottedName, rule.DottedName[2:]) {
					match = false
					continue
				}

				if len(domain) < len(rule.DottedName) {
					// Handle corner case where the domain doesn't have a left side and a wildcard rule matches,
					// i.e ".ck" with rule "*.ck" must return .ck as per golang implementation
					if domain[0] == '.' && strings.Compare(domain, rule.DottedName[1:]) == 0 {
						return domain, rule.ICANN, match
					}

					match = false
					continue
				}

				var nbLevels = strings.Count(rule.DottedName, ".") + 1
				var dot = len(domain) - 1

				for i := 0; i < nbLevels && dot != -1; i++ {
					dot = strings.LastIndex(domain[:dot], ".")
				}

				return domain[dot+1:], rule.ICANN, match

			case exception:
				// first check if the rule is contained within the domain without !
				if !strings.HasSuffix(sub.dottedName, rule.DottedName[1:]) {
					match = false
					continue
				}

				var dot = strings.Index(rule.DottedName, ".")

				return rule.DottedName[dot+1:], rule.ICANN, match

			default:
				// first check if the rule is contained within the domain
				if !strings.HasSuffix(sub.dottedName, rule.DottedName) {
					match = false
					continue
				}

				return rule.DottedName, rule.ICANN, match
			}
		}
	}

	// If no rules match, the prevailing rule is "*".
	var dot = strings.LastIndex(domain, ".")

	return domain[dot+1:], false, false
}

// newList reads and parses r to create a new rulesInfo identified by release.
func newList(r io.Reader, release string) (*rulesInfo, error) {
	var icann = false
	var scanner = bufio.NewScanner(r)
	var tempRulesMap = make(map[string][]rule)
	var mapKey string

	for scanner.Scan() {
		var line = strings.TrimSpace(scanner.Text())

		if strings.Contains(line, icannBegin) {
			icann = true
			continue
		}

		if strings.Contains(line, icannEnd) {
			icann = false
			continue
		}

		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		var err error
		line, err = idna.ToASCII(line)
		if err != nil {
			return nil, fmt.Errorf("error while converting to ASCII %s: %s", line, err.Error())
		}

		if !validSuffixRE.MatchString(line) {
			return nil, fmt.Errorf("bad publicsuffix.org list data: %q", line)
		}

		var rule = rule{ICANN: icann, DottedName: line}
		var concatenatedLine = strings.Replace(line, ".", "", -1)

		switch {
		case strings.HasPrefix(concatenatedLine, "*"):
			rule.RuleType = wildcard
			mapKey = concatenatedLine[1:]
		case strings.HasPrefix(concatenatedLine, "!"):
			rule.RuleType = exception
			mapKey = concatenatedLine[1:]
		default:
			rule.RuleType = normal
			mapKey = concatenatedLine
		}

		tempRulesMap[mapKey] = append(tempRulesMap[mapKey], rule)
	}

	var tempRulesInfo = rulesInfo{Release: release, Map: tempRulesMap}

	return &tempRulesInfo, nil
}

// decomposeDomain breaks domain down into a slice of labels.
func decomposeDomain(domain string, subdomains []subdomain) []subdomain {
	var sub = subdomain{dottedName: domain, name: strings.Replace(domain, ".", "", -1)}

	subdomains = append(subdomains, sub)

	var name = domain
	for {
		var dot = strings.Index(name, ".")
		if dot == -1 {
			break
		}

		name = name[dot+1:]
		var sub = subdomain{dottedName: name, name: strings.Replace(name, ".", "", -1)}
		subdomains = append(subdomains, sub)
	}

	return subdomains
}
