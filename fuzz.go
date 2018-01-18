// +build gofuzz

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

	psl "golang.org/x/net/publicsuffix"
)

func Fuzz(in []byte) int {
	var domain = string(in)

	var got, _ = PublicSuffix(domain)
	var want, _ = psl.PublicSuffix(domain)
	if want != got {
		panic(fmt.Sprintf("output mismatch: got %q, want %q (%v)\n", got, want, domain))
	}

	var wantErr error
	want, wantErr = psl.EffectiveTLDPlusOne(domain)

	var err error
	got, err = EffectiveTLDPlusOne(domain)
	if want != got {
		panic(fmt.Sprintf("output mismatch: TLD got %q, want %q (%v)\n", got, want, domain))
	}

	// Compare if an error exists, not the value of it
	if (err == nil) != (wantErr == nil) {
		panic(fmt.Sprintf("error mismatch: got err %q, want %q (%v)\n", err, wantErr, domain))
	}

	if err != nil {
		return -1
	}

	return 1
}
