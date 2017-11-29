package publicsuffix

import (
	"fmt"
	"net/http/cookiejar"
)

type list struct{}

// List implements the cookiejar.PublicSuffixList interface by calling the
// PublicSuffix function.
var List cookiejar.PublicSuffixList = list{}

func (list) PublicSuffix(domain string) string {
	var ps, _ = PublicSuffix(domain)
	return ps
}

func (list) String() string {
	var rules = load()
	return fmt.Sprintf("publicsuffix.org's public_suffix_list.dat, git revision: %s", rules.Release)
}
