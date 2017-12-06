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
	GetList(string) (io.Reader, error)
}

// GitHubListRetriever implements the ListRetriever using github
type GitHubListRetriever struct{}

// ReleaseInfo decodes the sha field from the commit information
type ReleaseInfo struct {
	SHA string `json:"sha"`
}

var (
	gitCommitURL    = "https://api.github.com/repos/publicsuffix/list/commits?path=public_suffix_list.dat"
	publicSuffixURL = "https://raw.githubusercontent.com/publicsuffix/list/%s/public_suffix_list.dat"
)

// GetLatestReleaseTag retrieves the tag for the latest commit on Public Suffix List repo
func (gh GitHubListRetriever) GetLatestReleaseTag() (string, error) {
	var res, err = http.Get(gitCommitURL)
	if err != nil {
		return "", fmt.Errorf("error while retrieving last release information from github: %s", err.Error())
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error GET %s: status %d", gitCommitURL, res.StatusCode)
	}

	var releaseInfo []ReleaseInfo
	if err = json.NewDecoder(res.Body).Decode(&releaseInfo); err != nil {
		return "", fmt.Errorf("error decoding release info: %s", err.Error())
	}

	if len(releaseInfo) == 0 ||
		(len(releaseInfo) > 0 && releaseInfo[0].SHA == "") {
		return "", errors.New("no release info found from github")
	}

	return releaseInfo[0].SHA, nil
}

// GetList retrieves the given release of the Public Suffix List from the github repository
func (gh GitHubListRetriever) GetList(release string) (io.Reader, error) {
	var url = fmt.Sprintf(publicSuffixURL, release)

	var res, err = http.Get(url)
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
