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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ListRetriever is the interface for retrieving release information/content
type ListRetriever interface {
	GetLatestReleaseTag() (string, error)
	GetList(release string) (io.Reader, error)
}

// gitHubListRetriever implements the ListRetriever using github
type gitHubListRetriever struct {
	client *http.Client
}

// releaseInfo decodes the sha field from the commit information
type releaseInfo struct {
	SHA string `json:"sha"`
}

var (
	gitCommitURL    = "https://api.github.com/repos/publicsuffix/list/commits?path=public_suffix_list.dat"
	publicSuffixURL = "https://raw.githubusercontent.com/publicsuffix/list/%s/public_suffix_list.dat"
)

// NewGitHubListRetriever creates a new ListRetriever with a custom HTTP client.
func NewGitHubListRetriever(client *http.Client) ListRetriever {
	return gitHubListRetriever{
		client: client,
	}
}

// GetLatestReleaseTag retrieves the tag for the latest commit on Public Suffix List repo
func (gh gitHubListRetriever) GetLatestReleaseTag() (string, error) {
	var res, err = gh.client.Get(gitCommitURL)
	if err != nil {
		return "", fmt.Errorf("error while retrieving last release information from github: %s", err.Error())
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error GET %s: status %d", gitCommitURL, res.StatusCode)
	}

	var releaseInfo []releaseInfo
	if err = json.NewDecoder(res.Body).Decode(&releaseInfo); err != nil {
		return "", fmt.Errorf("error decoding release info: %s", err.Error())
	}

	if len(releaseInfo) == 0 || releaseInfo[0].SHA == "" {
		return "", errors.New("no release info found from github")
	}

	return releaseInfo[0].SHA, nil
}

// GetList retrieves the given release of the Public Suffix List from the github repository
func (gh gitHubListRetriever) GetList(release string) (io.Reader, error) {
	var url = fmt.Sprintf(publicSuffixURL, release)

	// Just in case a nil client was passed, use the default http client.
	client := http.DefaultClient
	if gh.client != nil {
		client = gh.client
	}

	var res, err = client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error while retrieving last revision of the PSL(%s): %s", release, err.Error())
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error GET %s: status %d", url, res.StatusCode)
	}

	var buf = &bytes.Buffer{}
	if _, err := io.Copy(buf, res.Body); err != nil {
		return nil, err
	}

	return buf, nil
}
