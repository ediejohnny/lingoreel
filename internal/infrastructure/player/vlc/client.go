package vlc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"time"
)

const (
	endpointPath     = "/requests/lingoreel.json"
	requestTimeout   = 2 * time.Second
	maxResponseBytes = 64 << 10
	// tokenParameter carries the session token the endpoint demands.
	tokenParameter = "token"
)

// Statuses the endpoint reports; statusDenied means the token did not match.
const (
	statusPlaying = "playing"
	statusPaused  = "paused"
	statusStopped = "stopped"
	statusDenied  = "denied"
)

// errForeignEndpoint reports an answer that is not our endpoint's: another
// program listens on the port, or a VLC started for another session.
var errForeignEndpoint = errors.New("another program answered on the VLC control port")

// status is the answer of the control endpoint.
type status struct {
	Status string  `json:"status"`
	Time   int64   `json:"time"`
	Length int64   `json:"length"`
	Rate   float64 `json:"rate"`
	// Volume is on VLC's scale (256 is 100%); negative before the sound
	// output exists.
	Volume float64 `json:"volume"`
	// Audio is the playing audio track's position among the audio tracks,
	// -1 for none.
	Audio int `json:"audio"`
}

// client talks to the control endpoint served by VLC on the loopback interface.
// The password is visible on VLC's command line; the token only lives in the
// endpoint file, so reading the command line is not enough to drive VLC.
type client struct {
	base     string
	password string
	token    string
	http     *http.Client
}

func newClient(port int, password, token string) *client {
	return &client{
		base:     fmt.Sprintf("http://127.0.0.1:%d%s", port, endpointPath),
		password: password,
		token:    token,
		http:     &http.Client{Timeout: requestTimeout},
	}
}

// call sends an optional command and returns the player status.
func (c *client) call(ctx context.Context, params url.Values) (status, error) {
	query := url.Values{}
	maps.Copy(query, params)
	query.Set(tokenParameter, c.token)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"?"+query.Encode(), nil)
	if err != nil {
		return status{}, fmt.Errorf("building the VLC request: %w", err)
	}
	request.SetBasicAuth("", c.password)
	response, err := c.http.Do(request)
	if err != nil {
		return status{}, fmt.Errorf("calling VLC: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return status{}, fmt.Errorf("%w: it answered %s", errForeignEndpoint, response.Status)
	}
	var result status
	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes)).Decode(&result); err != nil {
		return status{}, fmt.Errorf("%w: unreadable answer: %v", errForeignEndpoint, err)
	}
	switch result.Status {
	case statusPlaying, statusPaused, statusStopped:
		return result, nil
	case statusDenied:
		return status{}, fmt.Errorf("%w: the session token was refused", errForeignEndpoint)
	}
	return status{}, fmt.Errorf("%w: unknown status %q", errForeignEndpoint, result.Status)
}

func command(name string, pairs ...string) url.Values {
	values := url.Values{"command": {name}}
	for i := 0; i+1 < len(pairs); i += 2 {
		values.Set(pairs[i], pairs[i+1])
	}
	return values
}
