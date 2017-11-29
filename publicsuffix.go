package publicsuffix

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"

	"golang.org/x/net/idna"
)

// RulesList encapsulates the list of domain rules and the commit version that generated them
type RulesList struct {
	List    []Rule
	Release string
}

// Rule contains the data related to a domain from the PSL
type Rule struct {
	Name       string
	DottedName string
	RuleType   RuleType
	ICANN      bool
}

// RuleType encapsulates integer for enum
type RuleType int

const (
	normal RuleType = iota
	wildcard
	exception
)

var (
	// validSuffixRE is used to check that the entries in the public suffix
	// list are in canonical form (after Punycode encoding). Specifically,
	// capital letters are not allowed.
	validSuffixRE = regexp.MustCompile(`^[a-z0-9_\!\*\-\.]+$`)

	// rules caches the PSL from the last commit available
	// handles read/write concurrency
	rules atomic.Value

	maxNbSubdomains = 1

	githubListRetriever = GitHubListRetriever{}
)

func init() {
	var initError = "error while initialising Public Suffix List from list.go: %s"

	var bytes bytes.Buffer
	bytes.Write(listBytes)

	var uncompressed, err = zlib.NewReader(&bytes)
	if err != nil {
		panic(fmt.Sprintf(initError, err.Error()))
	}

	if err := populateList(uncompressed, initialRelease); err != nil {
		panic(fmt.Sprintf(initError, err.Error()))
	}
}

func load() RulesList {
	return rules.Load().(RulesList)
}

// Write serialises and writes the Public Suffix List rules
func Write(w io.Writer) error {
	return json.NewEncoder(w).Encode(load())
}

// Read deserialises the reader and writes the data to the Public Suffix List rules
func Read(r io.Reader) error {
	var tempRules = RulesList{}
	if err := json.NewDecoder(r).Decode(&tempRules); err != nil {
		return err
	}
	rules.Store(tempRules)

	return nil
}

// Update updates the internal Public Suffix List rules using the github repo
func Update() error {
	return UpdateWithListRetriever(githubListRetriever)
}

// UpdateWithListRetriever updates the internal Public Suffix List rules with the latest version using the list retriever provided
func UpdateWithListRetriever(listRetriever ListRetriever) error {
	var latestTag, err = listRetriever.GetLatestReleaseTag()
	if err != nil {
		return fmt.Errorf("error while retrieving last commit information: %s", err.Error())
	}

	if latestTag == "" || load().Release == latestTag {
		return nil
	}

	var rawList io.Reader
	rawList, err = listRetriever.GetList(latestTag)
	if err != nil {
		return fmt.Errorf("error while retrieving Public Suffix List last release: %s", err.Error())
	}

	populateList(rawList, latestTag)

	return nil
}

// IsInPublicSuffixList returns true if the domain is found in the Public Suffix List
func IsInPublicSuffixList(domain string) bool {
	var _, _, found = searchList(domain)

	return found
}

// PublicSuffix returns the public suffix for the given domain, and a bool indicating if it's managed by the Internet Corporation
// for Assigned Names and Numbers
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

// Release returns the release of the current Public Suffix List
func Release() string {
	return load().Release
}

func searchList(domain string) (string, bool, bool) {
	var subdomains = decomposeDomain(domain)
	var rules = load()
	var found = false

	// the longest matching rule (the one with the most levels) will be used, so we
	// walk the subdomains backwards to ensure the smallest subdomain is tested last
	for i := len(subdomains) - 1; i >= 0; i-- {
		var subdomain = subdomains[i]

		var index = sort.Search(len(rules.List), func(i int) bool {
			return rules.List[i].Name >= subdomain
		})

		// if not found, continue
		if index == len(rules.List) {
			continue
		}
		// If found check the rule type
		if rules.List[index].Name == subdomain {
			var rule = rules.List[index]
			found = true

			if rule.RuleType == wildcard {
				if len(domain) < len(rule.DottedName) {
					continue
				}

				var nbLevels = len(strings.Split(rule.DottedName, "."))
				var dot = len(domain) - 1

				for i := 0; i < nbLevels; i++ {
					dot = strings.LastIndex(domain[:dot], ".")
				}

				return domain[dot+1:], rule.ICANN, found
			}
			//If the rule is an exception rule, modify it by removing the leftmost label
			if rule.RuleType == exception {
				var dot = strings.Index(rule.DottedName, ".")
				return rule.DottedName[dot+1:], rule.ICANN, found
			}

			return rule.DottedName, rule.ICANN, found
		}
	}

	// If no rules match, the prevailing rule is "*".
	var dot = strings.LastIndex(domain, ".")

	return domain[dot+1:], false, found
}

func populateList(r io.Reader, release string) error {
	var icann = false
	var bufferReader = bufio.NewReader(r)
	var tempRules = RulesList{}

	for {
		var line, err = bufferReader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if strings.Contains(line, "BEGIN ICANN DOMAINS") {
			icann = true
		}

		if strings.Contains(line, "END ICANN DOMAINS") {
			icann = false
		}

		line, err = idna.ToASCII(line)
		if err != nil {
			return fmt.Errorf("error while converting to ASCII %s: %s", line, err.Error())
		}

		if !validSuffixRE.MatchString(line) {
			return fmt.Errorf("bad publicsuffix.org list data: %q", line)
		}

		var rule = Rule{ICANN: icann, DottedName: line}
		var concatenatedLine = strings.Replace(line, ".", "", -1)

		var nbSubdomains = len(strings.Split(line, "."))
		if nbSubdomains > maxNbSubdomains {
			maxNbSubdomains = nbSubdomains
		}

		switch {
		case strings.HasPrefix(concatenatedLine, "*"):
			rule.RuleType = wildcard
			rule.Name = concatenatedLine[1:]
		case strings.HasPrefix(concatenatedLine, "!"):
			rule.RuleType = exception
			rule.Name = concatenatedLine[1:]
		default:
			rule.RuleType = normal
			rule.Name = concatenatedLine
		}

		tempRules.List = append(tempRules.List, rule)
	}

	tempRules.Release = release

	// sort the list by name to be able to use binary search later
	sort.Slice(tempRules.List, func(i int, j int) bool {
		return tempRules.List[i].Name < tempRules.List[j].Name
	})

	rules.Store(tempRules)

	return nil
}

func decomposeDomain(domain string) []string {
	var labels = strings.Split(domain, ".")

	var start = len(labels) - 1
	var end = 0

	if len(labels) > maxNbSubdomains {
		end = start - maxNbSubdomains
	}

	var subdomain string
	var subdomains []string

	for i := start; i >= end; i-- {
		subdomain = labels[i] + subdomain
		subdomains = append(subdomains, subdomain)
	}

	return subdomains
}
