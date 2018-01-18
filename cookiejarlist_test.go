/*
Copyright 2018 GMO GlobalSign Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package publicsuffix

import (
	"fmt"
	"testing"
)

func TestCookieJarList_PublicSuffix(t *testing.T) {
	for _, tc := range publicSuffixTestCases {
		got := CookieJarList.PublicSuffix(tc.domain)
		if got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.domain, got, tc.want)
		}
	}
}

func TestCookieJarList_String(t *testing.T) {
	var expected = fmt.Sprintf("publicsuffix.org's public_suffix_list.dat, git revision: %s", initialRelease)
	if release := CookieJarList.String(); release != expected {
		t.Fatalf("got: %s, want %s", release, expected)
	}
}
