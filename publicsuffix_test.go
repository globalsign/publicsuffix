package publicsuffix

import (
	"bytes"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// eTLDPlusOneTestCases come from
// https://github.com/publicsuffix/list/blob/master/tests/test_psl.txt
var eTLDPlusOneTestCases = []struct {
	domain, want string
}{
	// Empty input.
	{"", ""},
	// Unlisted TLD.
	{"example", ""},
	{"example.example", "example.example"},
	{"b.example.example", "example.example"},
	{"a.b.example.example", "example.example"},
	// TLD with only 1 rule.
	{"biz", ""},
	{"domain.biz", "domain.biz"},
	{"b.domain.biz", "domain.biz"},
	{"a.b.domain.biz", "domain.biz"},
	// TLD with some 2-level rules.
	{"com", ""},
	{"example.com", "example.com"},
	{"b.example.com", "example.com"},
	{"a.b.example.com", "example.com"},
	{"uk.com", ""},
	{"example.uk.com", "example.uk.com"},
	{"b.example.uk.com", "example.uk.com"},
	{"a.b.example.uk.com", "example.uk.com"},
	{"test.ac", "test.ac"},
	// TLD with only 1 (wildcard) rule.
	{"mm", ""},
	{"c.mm", ""},
	{"b.c.mm", "b.c.mm"},
	{"a.b.c.mm", "b.c.mm"},
	// More complex TLD.
	{"jp", ""},
	{"test.jp", "test.jp"},
	{"www.test.jp", "test.jp"},
	{"ac.jp", ""},
	{"test.ac.jp", "test.ac.jp"},
	{"www.test.ac.jp", "test.ac.jp"},
	{"kyoto.jp", ""},
	{"test.kyoto.jp", "test.kyoto.jp"},
	{"ide.kyoto.jp", ""},
	{"b.ide.kyoto.jp", "b.ide.kyoto.jp"},
	{"a.b.ide.kyoto.jp", "b.ide.kyoto.jp"},
	{"c.kobe.jp", ""},
	{"b.c.kobe.jp", "b.c.kobe.jp"},
	{"a.b.c.kobe.jp", "b.c.kobe.jp"},
	{"city.kobe.jp", "city.kobe.jp"},
	{"www.city.kobe.jp", "city.kobe.jp"},
	// TLD with a wildcard rule and exceptions.
	{"ck", ""},
	{"test.ck", ""},
	{"b.test.ck", "b.test.ck"},
	{"a.b.test.ck", "b.test.ck"},
	{"www.ck", "www.ck"},
	{"www.www.ck", "www.ck"},
	// US K12.
	{"us", ""},
	{"test.us", "test.us"},
	{"www.test.us", "test.us"},
	{"ak.us", ""},
	{"test.ak.us", "test.ak.us"},
	{"www.test.ak.us", "test.ak.us"},
	{"k12.ak.us", ""},
	{"test.k12.ak.us", "test.k12.ak.us"},
	{"www.test.k12.ak.us", "test.k12.ak.us"},
	// Punycoded IDN labels
	{"xn--85x722f.com.cn", "xn--85x722f.com.cn"},
	{"xn--85x722f.xn--55qx5d.cn", "xn--85x722f.xn--55qx5d.cn"},
	{"www.xn--85x722f.xn--55qx5d.cn", "xn--85x722f.xn--55qx5d.cn"},
	{"shishi.xn--55qx5d.cn", "shishi.xn--55qx5d.cn"},
	{"xn--55qx5d.cn", ""},
	{"xn--85x722f.xn--fiqs8s", "xn--85x722f.xn--fiqs8s"},
	{"www.xn--85x722f.xn--fiqs8s", "xn--85x722f.xn--fiqs8s"},
	{"shishi.xn--fiqs8s", "shishi.xn--fiqs8s"},
	{"xn--fiqs8s", ""},
}

func Test_EffectiveTLDPlusOne(t *testing.T) {
	//t.Parallel()
	for _, tc := range eTLDPlusOneTestCases {
		got, _ := EffectiveTLDPlusOne(tc.domain)
		if got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.domain, got, tc.want)
		}
	}
}

var publicSuffixTestCases = []struct {
	domain, want string
}{
	// Empty string.
	{"", ""},

	// The .ao rules are:
	// ao
	// ed.ao
	// gv.ao
	// og.ao
	// co.ao
	// pb.ao
	// it.ao
	{"ao", "ao"},
	{"www.ao", "ao"},
	{"pb.ao", "pb.ao"},
	{"www.pb.ao", "pb.ao"},
	{"www.xxx.yyy.zzz.pb.ao", "pb.ao"},

	// The .ar rules are:
	// ar
	// com.ar
	// edu.ar
	// gob.ar
	// gov.ar
	// int.ar
	// mil.ar
	// net.ar
	// org.ar
	// tur.ar
	// blogspot.com.ar
	{"ar", "ar"},
	{"www.ar", "ar"},
	{"nic.ar", "ar"},
	{"www.nic.ar", "ar"},
	{"com.ar", "com.ar"},
	{"www.com.ar", "com.ar"},
	{"blogspot.com.ar", "blogspot.com.ar"},
	{"www.blogspot.com.ar", "blogspot.com.ar"},
	{"www.xxx.yyy.zzz.blogspot.com.ar", "blogspot.com.ar"},
	{"logspot.com.ar", "com.ar"},
	{"zlogspot.com.ar", "com.ar"},
	{"zblogspot.com.ar", "com.ar"},

	// The .arpa rules are:
	// arpa
	// e164.arpa
	// in-addr.arpa
	// ip6.arpa
	// iris.arpa
	// uri.arpa
	// urn.arpa
	{"arpa", "arpa"},
	{"www.arpa", "arpa"},
	{"urn.arpa", "urn.arpa"},
	{"www.urn.arpa", "urn.arpa"},
	{"www.xxx.yyy.zzz.urn.arpa", "urn.arpa"},

	// The relevant {kobe,kyoto}.jp rules are:
	// jp
	// *.kobe.jp
	// !city.kobe.jp
	// kyoto.jp
	// ide.kyoto.jp
	{"jp", "jp"},
	{"kobe.jp", "jp"},
	{"c.kobe.jp", "c.kobe.jp"},
	{"b.c.kobe.jp", "c.kobe.jp"},
	{"a.b.c.kobe.jp", "c.kobe.jp"},
	{"city.kobe.jp", "kobe.jp"},
	{"www.city.kobe.jp", "kobe.jp"},
	{"kyoto.jp", "kyoto.jp"},
	{"test.kyoto.jp", "kyoto.jp"},
	{"ide.kyoto.jp", "ide.kyoto.jp"},
	{"b.ide.kyoto.jp", "ide.kyoto.jp"},
	{"a.b.ide.kyoto.jp", "ide.kyoto.jp"},

	// The .tw rules are:
	// tw
	// edu.tw
	// gov.tw
	// mil.tw
	// com.tw
	// net.tw
	// org.tw
	// idv.tw
	// game.tw
	// ebiz.tw
	// club.tw
	// 網路.tw (xn--zf0ao64a.tw)
	// 組織.tw (xn--uc0atv.tw)
	// 商業.tw (xn--czrw28b.tw)
	// blogspot.tw
	{"tw", "tw"},
	{"aaa.tw", "tw"},
	{"www.aaa.tw", "tw"},
	{"xn--czrw28b.aaa.tw", "tw"},
	{"edu.tw", "edu.tw"},
	{"www.edu.tw", "edu.tw"},
	{"xn--czrw28b.edu.tw", "edu.tw"},
	{"xn--czrw28b.tw", "xn--czrw28b.tw"},
	{"www.xn--czrw28b.tw", "xn--czrw28b.tw"},
	{"xn--uc0atv.xn--czrw28b.tw", "xn--czrw28b.tw"},
	{"xn--kpry57d.tw", "tw"},

	// The .uk rules are:
	// uk
	// ac.uk
	// co.uk
	// gov.uk
	// ltd.uk
	// me.uk
	// net.uk
	// nhs.uk
	// org.uk
	// plc.uk
	// police.uk
	// *.sch.uk
	// blogspot.co.uk
	{"uk", "uk"},
	{"aaa.uk", "uk"},
	{"www.aaa.uk", "uk"},
	{"mod.uk", "uk"},
	{"www.mod.uk", "uk"},
	{"sch.uk", "uk"},
	{"mod.sch.uk", "mod.sch.uk"},
	{"www.sch.uk", "www.sch.uk"},
	{"blogspot.co.uk", "blogspot.co.uk"},
	{"blogspot.nic.uk", "uk"},
	{"blogspot.sch.uk", "blogspot.sch.uk"},

	// The .рф rules are
	// рф (xn--p1ai)
	{"xn--p1ai", "xn--p1ai"},
	{"aaa.xn--p1ai", "xn--p1ai"},
	{"www.xxx.yyy.xn--p1ai", "xn--p1ai"},

	// The .bd rules are:
	// *.bd
	{"bd", "bd"},
	{"www.bd", "www.bd"},
	{"zzz.bd", "zzz.bd"},
	{"www.zzz.bd", "zzz.bd"},
	{"www.xxx.yyy.zzz.bd", "zzz.bd"},

	// There are no .nosuchtld rules.
	{"nosuchtld", "nosuchtld"},
	{"foo.nosuchtld", "nosuchtld"},
	{"bar.foo.nosuchtld", "nosuchtld"},
	{"free.", ""},
	{"e.co", "co"},
	{"g.n", "n"},
	{"cl.a", "a"},
	{".m.m", "m"},
	{"b..n", "n"},
	{".ck", ".ck"},
	{"a.ck", "a.ck"},
	{"k.h", "h"},
}

func Test_PublicSuffix(t *testing.T) {
	for _, tc := range publicSuffixTestCases {
		got, _ := PublicSuffix(tc.domain)
		if got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.domain, got, tc.want)
		}
	}
}

func Test_SearchList(t *testing.T) {
	var tests = []struct {
		domain   string
		expected string
		icann    bool
		found    bool
	}{
		{"nosuchtld", "nosuchtld", false, false},
		{"www.bd", "www.bd", true, true},
		{"xn--p1ai", "xn--p1ai", true, true},
		{"example.globalsign.fake", "fake", false, false},
		{"cl.a", "a", false, false},
		{".m.m", "m", false, false},
		{"b..n", "n", false, false},
		{"b.n", "n", false, false},
		{"np", "np", false, false}, // rule *.np
		{"ad", "ad", true, true},
		{"00.za", "za", false, false},
		{"transurl.be", "be", true, true},
		{"0emm.com", "com", true, true},
		{"i.ng", "i.ng", true, true},
		{".mm", ".mm", true, true},
	}

	for _, tt := range tests {
		var tt = tt
		t.Run(tt.domain, func(t *testing.T) {
			got, icann, found := searchList(tt.domain)
			if got != tt.expected {
				t.Errorf("%q: got %q, want %q", tt.domain, got, tt.expected)
			}
			if icann != tt.icann {
				t.Errorf("%q: got %v, want %v", tt.domain, icann, tt.icann)
			}
			if found != tt.found {
				t.Errorf("%q: got %v, want %v", tt.domain, found, tt.found)
			}
		})
	}
}

func Test_IsInPublicSuffixList(t *testing.T) {
	var tests = []struct {
		domain string
		found  bool
	}{
		{"nosuchtld", false},
		{"www.bd", true},
		{"xn--p1ai", true},
		{"example.globalsign.fake", false},
		{"cl.a", false},
		{".m.m", false},
		{"b..n", false},
	}

	for _, tt := range tests {
		var tt = tt
		t.Run(tt.domain, func(t *testing.T) {
			if found := IsInPublicSuffixList(tt.domain); found != tt.found {
				t.Errorf("%q: got %v want %v", tt.domain, found, tt.found)
			}
		})
	}

}

func Test_Release(t *testing.T) {
	var testRelease = "release_test"
	var listRetriever = mockListRetriever{Release: testRelease, RawList: &bytes.Buffer{}}

	UpdateWithListRetriever(listRetriever)

	if release := Release(); release != testRelease {
		t.Fatalf("got: %s want: %s", release, testRelease)
	}

}

func Test_Update(t *testing.T) {
	var initialRelease = load().Release
	var tests = []struct {
		name            string
		mockRetriever   mockListRetriever
		expectedRelease string
	}{
		{
			"No update required, latest release",
			mockListRetriever{Release: initialRelease},
			initialRelease,
		},
		{
			"Update required",
			mockListRetriever{Release: "test", RawList: &bytes.Buffer{}},
			"test",
		},
		{
			"Empty release, don't update",
			mockListRetriever{Release: "", RawList: &bytes.Buffer{}},
			"test",
		},
	}

	for _, tt := range tests {
		var tt = tt
		t.Run(tt.name, func(t *testing.T) {
			if err := UpdateWithListRetriever(tt.mockRetriever); err != nil {
				t.Fatalf("unexpected error: %s", err.Error())
			}
			var rules = load()
			if rules.Release != tt.expectedRelease {
				t.Fatalf("want: %s got: %s", tt.expectedRelease, rules.Release)
			}
		})
	}
}

func Test_Write(t *testing.T) {
	var input bytes.Buffer
	input.WriteString(`//
		ac
		com.ac
		//`)

	var mockRetriever = mockListRetriever{RawList: &input, Release: "write_test"}

	if err := UpdateWithListRetriever(mockRetriever); err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}

	var bytes bytes.Buffer
	if err := Write(&bytes); err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}

	var expected = `{"Map":{"ac":[{"DottedName":"ac","RuleType":0,"ICANN":false}],"comac":[{"DottedName":"com.ac","RuleType":0,"ICANN":false}]},"Release":"write_test"}
`
	if strings.Compare(bytes.String(), expected) != 0 {
		t.Fatalf("got: %#v, want: %#v", bytes.String(), expected)
	}

}

func Test_Read(t *testing.T) {
	// empty the rules - size 0
	var mockRetriever = mockListRetriever{RawList: &bytes.Buffer{}, Release: "read_test"}
	if err := UpdateWithListRetriever(mockRetriever); err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	var rules = load()
	if len(rules.Map) != 0 {
		t.Fatalf("got: %d want: %d", len(rules.Map), 0)
	}

	var expectedNbRules = 2
	var bytes bytes.Buffer
	bytes.WriteString(`{"Map":{"ac":[{"DottedName":"ac","RuleType":0,"ICANN":false}],"comac":[{"DottedName":"com.ac","RuleType":0,"ICANN":false}]},"Release":"write_test"}`)

	if err := Read(&bytes); err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	rules = load()
	if len(rules.Map) != expectedNbRules {
		t.Fatalf("got: %d want: %d", len(rules.Map), expectedNbRules)
	}
}

func Test_PopulateList(t *testing.T) {
	var testRelease = "populate_test"

	t.Run("OK", func(t *testing.T) {
		var input bytes.Buffer
		input.WriteString(`// Instructions on pulling and using this list can be found at https://publicsuffix.org/list/.

// ===BEGIN ICANN DOMAINS===

// ac : https://en.wikipedia.org/wiki/.ac
ac
com.ac
edu.ac
gov.ac
net.ac
mil.ac
org.ac
`)
		var nbRules = 7

		if err := populateList(&input, testRelease); err != nil {
			t.Fatalf("unexpected error: %s", err.Error())
		}
		var rules = load()
		if rules.Release != testRelease {
			t.Fatalf("got: %s, want: %s", rules.Release, testRelease)
		}

		if len(rules.Map) != nbRules {
			t.Fatalf("got: %d, want: %d", len(rules.Map), nbRules)
		}

		if !rules.Map["ac"][0].ICANN {
			t.Fatalf("icann should be true, got: %v", rules.Map["ac"][0].ICANN)
		}
	})

	t.Run("Error bad suffix list data", func(t *testing.T) {
		var input bytes.Buffer
		input.WriteString(`//
			COM
			`)
		var expectedErr = errors.New("bad publicsuffix.org list data: \"COM\"")

		var err = populateList(&input, testRelease)

		if !reflect.DeepEqual(err, expectedErr) {
			t.Fatalf("got: %v, want: %v", err, expectedErr)
		}
	})

	t.Run("Rule type checks", func(t *testing.T) {
		var tests = []struct {
			input    string
			expected RuleType
		}{
			{
				`//
				!ac
				//`,
				exception,
			},
			{
				`//
				*.ac
				//`,
				wildcard,
			},
			{
				`//
				ac
				//`,
				normal,
			},
		}
		for _, test := range tests {
			var input bytes.Buffer
			input.WriteString(test.input)

			if err := populateList(&input, testRelease); err != nil {
				t.Fatalf("unexpected error: %s", err.Error())
			}

			var rules = load()
			if rules.Map["ac"][0].RuleType != test.expected {
				t.Fatalf("got: %v, want: %v", rules.Map["ac"][0].RuleType, test.expected)
			}
		}
	})
}

func Test_DecomposeDomain(t *testing.T) {
	var tests = []struct {
		input    string
		expected []subdomain
	}{
		{"example.co.uk", []subdomain{{"examplecouk", "example.co.uk"}, {"couk", "co.uk"}, {"uk", "uk"}}},
		{"b.ide.kyoto.jp", []subdomain{{"bidekyotojp", "b.ide.kyoto.jp"}, {"idekyotojp", "ide.kyoto.jp"}, {"kyotojp", "kyoto.jp"}, {"jp", "jp"}}},
		{"bd", []subdomain{{"bd", "bd"}}},
		{"aaa.xn--p1ai", []subdomain{{"aaaxn--p1ai", "aaa.xn--p1ai"}, {"xn--p1ai", "xn--p1ai"}}},
	}

	var subdomains = subdomainPool.Get().([]subdomain)[:0]
	defer subdomainPool.Put(subdomains)

	for _, tt := range tests {
		var tt = tt
		t.Run(tt.input, func(t *testing.T) {
			var results = decomposeDomain(tt.input, subdomains)

			if !reflect.DeepEqual(results, tt.expected) {
				t.Fatalf("got: %v want: %v", results, tt.expected)
			}
		})
	}
}

func Test_Concurrency(t *testing.T) {
	var listRetriever = mockListRetriever{Release: "0", RawList: &bytes.Buffer{}}
	UpdateWithListRetriever(listRetriever)

	// start 100 workers
	const readWorkers = 100
	const domain = "example.co.uk"

	var wg = &sync.WaitGroup{}
	var sem = make(chan struct{})

	for i := 0; i < readWorkers; i++ {
		wg.Add(1)
		go readWorker(sem, wg, domain)
	}

	const writeWorkers = 50

	for i := 0; i < writeWorkers; i++ {
		wg.Add(1)
		go writeWorker(sem, wg, i)
	}

	close(sem)
	wg.Wait()
}

func readWorker(sem chan struct{}, wg *sync.WaitGroup, domain string) {
	<-sem
	for i := 0; i < 10; i++ {
		PublicSuffix(domain)
	}
	wg.Done()
}

func writeWorker(sem chan struct{}, wg *sync.WaitGroup, i int) {
	var listRetriever = mockListRetriever{Release: strconv.Itoa(i + 1), RawList: &bytes.Buffer{}}
	<-sem
	for i := 0; i < 10; i++ {
		UpdateWithListRetriever(listRetriever)
	}
	wg.Done()
}

func benchmarkPublicSuffix(domain string, b *testing.B) {
	for n := 0; n < b.N; n++ {
		PublicSuffix(domain)
	}
}

func BenchmarkPublicSuffix1(b *testing.B) { benchmarkPublicSuffix("example.ac.il", b) }
func BenchmarkPublicSuffix2(b *testing.B) { benchmarkPublicSuffix("www.example.blogspot.com", b) }
func BenchmarkPublicSuffix3(b *testing.B) { benchmarkPublicSuffix("parliament.uk", b) }
func BenchmarkPublicSuffix4(b *testing.B) { benchmarkPublicSuffix("www.example.test", b) }
func BenchmarkPublicSuffix5(b *testing.B) { benchmarkPublicSuffix("bar.foo.nosuchtld", b) }        // not present in the rules
func BenchmarkPublicSuffix6(b *testing.B) { benchmarkPublicSuffix("example.sch.uk", b) }           // wildcard rule
func BenchmarkPublicSuffix7(b *testing.B) { benchmarkPublicSuffix("example.city.kawasaki.jp", b) } // exception rule
