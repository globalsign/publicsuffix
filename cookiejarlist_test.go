package publicsuffix

import (
	"fmt"
	"testing"
)

func TestCookieJarList_PublicSuffix(t *testing.T) {
	for _, tc := range publicSuffixTestCases {
		got := List.PublicSuffix(tc.domain)
		if got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.domain, got, tc.want)
		}
	}
}

func TestCookieJarList_String(t *testing.T) {
	var expected = fmt.Sprintf("publicsuffix.org's public_suffix_list.dat, git revision: %s", initialRelease)
	if release := List.String(); release != expected {
		t.Fatalf("got: %s, want %s", release, expected)
	}
}
