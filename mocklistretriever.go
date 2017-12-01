package publicsuffix

import "io"

// mockListRetriever implements a mock of the ListRetriever for testing
type mockListRetriever struct {
	Release string
	RawList io.Reader
	Err     error
}

// GetLatestReleaseTag mocks the release retrieval
func (m mockListRetriever) GetLatestReleaseTag() (string, error) {
	return m.Release, m.Err
}

// GetList mocks the list retrieval
func (m mockListRetriever) GetList(release string) (io.Reader, error) {
	return m.RawList, m.Err
}
