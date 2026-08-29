// Package fleet stands in for a platform team's internal fleet API client.
// main constructs one Client and hands it to whichever commands need it as
// a constructor parameter, so it reaches handlers by closure rather than
// through a package global; see deploy.Command.
package fleet

import (
	"context"
	"time"
)

// Client talks to the fleet API. This stub has no network dependency: its
// methods just wait a moment and honor ctx, standing in for the real
// round trip.
type Client struct {
	Region string
}

// NewClient returns a Client scoped to the given region.
func NewClient(region string) *Client {
	return &Client{Region: region}
}

// Services returns the names of the services running in the client's
// region. The real client would ask the fleet API; completion calls this,
// which is why it takes no context: a completion callback answers in-line
// or not at all.
func (c *Client) Services() []string {
	return []string{"api", "billing", "web", "worker"}
}

// Deploy rolls out version of service, honoring ctx cancellation the way a
// real HTTP call would via its request context.
func (c *Client) Deploy(ctx context.Context, service, version string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(50 * time.Millisecond): // stand-in for a real rollout
	}
	return nil
}
