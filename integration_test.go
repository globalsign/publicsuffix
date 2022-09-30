//go:build integration

package publicsuffix

import (
	"net/http"
	"testing"
)

func TestNewGitHubListRetriever(t *testing.T) {
	var client *http.Client = http.DefaultClient
	lr := NewGitHubListRetriever(client)
	if glr, ok := lr.(gitHubListRetriever); !ok || glr.client != client {
		t.Fatalf("didn't get expected github list retriever, got %+#v", lr)
	}

	tag, err := lr.GetLatestReleaseTag()
	if err != nil {
		t.Fatalf("GetLatestReleaseTag() got err %v, want nil", err)
	}

	_, err = lr.GetList(tag)
	if err != nil {
		t.Fatalf("GetList(tag) got err %v, want nil", err)
	}
}

func TestUpdate(t *testing.T) {
	err := Update()
	if err != nil {
		t.Fatalf("Got err when updating %v", err)
	}
}
