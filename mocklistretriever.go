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
