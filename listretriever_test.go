package publicsuffix

import (
	"net/http"
	"testing"
)

func TestNewGitHubListRetriever(t *testing.T) {
	var client *http.Client
	lr := NewGitHubListRetriever(client)
	if glr, ok := lr.(gitHubListRetriever); !ok || glr.client != client {
		t.Fatalf("didn't get expected github list retriever, got %+#v", lr)
	}
}
