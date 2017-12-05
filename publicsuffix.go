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

// RulesInfo contains the map of rules and the commit version that generated them
type RulesInfo struct {
	Map     map[string][]Rule
	Release string
}

// Rule contains the data related to a domain from the PSL
type Rule struct {
	DottedName string
	RuleType   RuleType
	ICANN      bool
}

type subdomain struct {
	name       string
	dottedName string
}

// RuleType encapsulates integer for enum
type RuleType int

const (
	normal RuleType = iota
	wildcard
	exception
)

// ICANNBegin marks the beginning of ICANN domains
const ICANNBegin = "BEGIN ICANN DOMAINS"

// ICANNEnd marks the beginning of ICANN domains
const ICANNEnd = "END ICANN DOMAINS"

var (
	// validSuffixRE is used to check that the entries in the public suffix
	// list are in canonical form (after Punycode encoding). Specifically,
	// capital letters are not allowed.
	validSuffixRE = regexp.MustCompile(`^[a-z0-9_\!\*\-\.]+$`)

	// rules caches the PSL from the last commit available
	// handles read/write concurrency
	rules atomic.Value

	githubListRetriever = GitHubListRetriever{}

	// subdomainPool pools subdomain arrays to avoid reallocation cost
	subdomainPool = sync.Pool{
		New: func() interface{} {
			// 5 should cover the average domain
			return make([]subdomain, 5)
		},
	}
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

func load() RulesInfo {
	return rules.Load().(RulesInfo)
}

// Write serialises and writes the Public Suffix List rules
func Write(w io.Writer) error {
	return json.NewEncoder(w).Encode(load())
}

// Read deserialises the reader and writes the data to the Public Suffix List rules
func Read(r io.Reader) error {
	var tempRulesInfo = RulesInfo{}
	if err := json.NewDecoder(r).Decode(&tempRulesInfo); err != nil {
		return err
	}
	rules.Store(tempRulesInfo)

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
		return fmt.Errorf("error while retrieving last commit information (%s): %s", latestTag, err.Error())
	}

	if latestTag == "" || load().Release == latestTag {
		return nil
	}

	var rawList io.Reader
	rawList, err = listRetriever.GetList(latestTag)
	if err != nil {
		return fmt.Errorf("error while retrieving Public Suffix List last release (%s): %s", latestTag, err.Error())
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

		match = true

		// Look for all the rules matching the concatenated name
		for _, rule := range rules {
			switch rule.RuleType {
			case wildcard:
				// first check if the rule is contained within the domain without the *.
				if !strings.HasSuffix(sub.dottedName, rule.DottedName[2:]) {
					match = false
					continue
				}

				var nbLevels = len(strings.Split(rule.DottedName, "."))
				var dot = len(domain) - 1

				if len(domain) < len(rule.DottedName) {
					// Handle corner case where the domain doesn't have a left side and a wildcard rule matches,
					// i.e ".ck" with rule "*.ck" must return .ck as per golang implementation
					if domain[0] == '.' && strings.Compare(domain, rule.DottedName[1:]) == 0 {
						return domain, rule.ICANN, match
					}

					match = false
					continue
				}

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

func populateList(r io.Reader, release string) error {
	var icann = false
	var scanner = bufio.NewScanner(r)
	var tempRulesMap = make(map[string][]Rule)
	var mapKey string

	for scanner.Scan() {
		var line = strings.TrimSpace(scanner.Text())

		if strings.Contains(line, ICANNBegin) {
			icann = true
			continue
		}

		if strings.Contains(line, ICANNEnd) {
			icann = false
			continue
		}

		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		var err error
		line, err = idna.ToASCII(line)
		if err != nil {
			return fmt.Errorf("error while converting to ASCII %s: %s", line, err.Error())
		}

		if !validSuffixRE.MatchString(line) {
			return fmt.Errorf("bad publicsuffix.org list data: %q", line)
		}

		var rule = Rule{ICANN: icann, DottedName: line}
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

	var tempRulesInfo = RulesInfo{Release: release, Map: tempRulesMap}

	rules.Store(tempRulesInfo)

	return nil
}

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
