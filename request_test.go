package httpc_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/unvurn/httpc"
)

func TestNewRequest(t *testing.T) {
	req := httpc.NewRequest()
	assert.NotNil(t, req)
}

type httpbinResponse struct {
	Args    map[string]string `json:"args"`
	Headers map[string]string `json:"headers"`
	Origin  string            `json:"origin"`
	URL     string            `json:"url"`
}

const httpbinEndpoint = "https://httpbin.org"

func TestHttpbin_Get(t *testing.T) {
	u, _ := url.JoinPath(httpbinEndpoint, "get")
	resp, err := httpc.NewRequest().Get(context.Background(), u)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	r := httpbinResponse{}
	err = json.Unmarshal(resp, &r)
	assert.NoError(t, err)
}

func TestHttpbin_Get_NotFound(t *testing.T) {
	u, _ := url.JoinPath(httpbinEndpoint, "status/404")
	resp, err := httpc.NewRequest().Get(context.Background(), u)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestNewRequest_WithResponder(t *testing.T) {
	req := httpc.NewRequest().WithResponder(func(response *http.Response, r httpc.ResponderFunc[[]byte]) ([]byte, error) {
		panic("implement me")
	})
	assert.NotNil(t, req)
}
