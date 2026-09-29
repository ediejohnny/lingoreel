package vlc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	endpointPath     = "/requests/lingoreel.json"
	requestTimeout   = 2 * time.Second
	maxResponseBytes = 64 << 10
)

// status is the answer of the control endpoint.
type status struct {
	Status string  `json:"status"`
	Time   int64   `json:"time"`
	Length int64   `json:"length"`
	Rate   float64 `json:"rate"`
}

// client talks to the control endpoint served by VLC on the loopback interface.
type client struct {
	base     string
	password string
	http     *http.Client
}

func newClient(port int, password string) *client {
	return &client{
		base:     fmt.Sprintf("http://127.0.0.1:%d%s", port, endpointPath),
		password: password,
		http:     &http.Client{Timeout: requestTimeout},
	}
}

// call sends an optional command and returns the player status.
func (c *client) call(ctx context.Context, params url.Values) (status, error) {
	target := c.base
	if len(params) > 0 {
		target += "?" + params.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return status{}, err
	}
	request.SetBasicAuth("", c.password)
	response, err := c.http.Do(request)
	if err != nil {
		return status{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return status{}, fmt.Errorf("control endpoint answered %s", response.Status)
	}
	var result status
	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes)).Decode(&result); err != nil {
		return status{}, fmt.Errorf("decoding VLC status: %w", err)
	}
	return result, nil
}

func command(name string, pairs ...string) url.Values {
	values := url.Values{"command": {name}}
	for i := 0; i+1 < len(pairs); i += 2 {
		values.Set(pairs[i], pairs[i+1])
	}
	return values
}
