package aws

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

type mockTransport struct{}

func newMockTransport() http.RoundTripper {
	return &mockTransport{}
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Mock http.Response
	response := &http.Response{
		Header:     make(http.Header),
		Request:    req,
		StatusCode: http.StatusOK,
	}
	response.Header.Set("Content-Type", "application/json")

	fakeResponseBody := `{"kernelId" : null,"region":"ap-northeast-1"}`
	response.Body = ioutil.NopCloser(strings.NewReader(fakeResponseBody))
	return response, nil
}

func TestGetRegion(t *testing.T) {
	mockClient := &http.Client{}
	mockClient.Transport = newMockTransport()

	got, err := getIdentity(mockClient)
	const want = "ap-northeast-1"
	if err != nil {
		t.Fatal(err)
	}
	if got.Region != want {
		t.Fatalf("getRegion got: %s, want: %s", got.Region, want)
	}
}
