// +build gofuzz

package publicsuffix

import (
	"fmt"

	psl "golang.org/x/net/publicsuffix"
)

func Fuzz(in []byte) int {
	var domain = string(in)

	var got, icann = PublicSuffix(domain)
	var want, wantIcann = psl.PublicSuffix(domain)
	if want != got {
		panic(fmt.Sprintf("output mismatch: got %q, want %q\n", got, want))
	}

	if icann != wantIcann {
		panic(fmt.Sprintf("output mismatch: ICANN got %v, want %v\n", icann, wantIcann))
	}

	var wantErr error
	want, wantErr = psl.EffectiveTLDPlusOne(domain)

	var err error
	got, err = EffectiveTLDPlusOne(domain)
	if want != got {

		panic(fmt.Sprintf("output mismatch: TLD got %q, want %q\n", got, want))
	}

	// Compare if an error exists, not the value of it
	if (err == nil) != (wantErr == nil) {
		panic(fmt.Sprintf("error mismatch: got err %q, want %q\n", err, wantErr))
	}

	if err != nil {
		return -1
	}

	return 1
}
