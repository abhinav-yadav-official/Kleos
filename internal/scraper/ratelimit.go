package scraper

import (
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

// SourceLimits returns the default per-source rate limit (requests per second
// and burst). Values follow plan §8: 1 req/2s default, 1 req/5s for fragile
// scrapers.
func SourceLimits(source string) (rate.Limit, int) {
	switch source {
	case "linkedin", "indeed", "naukri":
		return rate.Every(5 * time.Second), 1
	default:
		return rate.Every(2 * time.Second), 1
	}
}

// LimitedClient returns a new *http.Client whose transport waits on the given
// limiter before each request. A zero/nil limiter returns the input client as-is.
func LimitedClient(base *http.Client, lim *rate.Limiter) *http.Client {
	if lim == nil {
		return base
	}
	if base == nil {
		base = &http.Client{Timeout: 30 * time.Second}
	}
	clone := *base
	rt := base.Transport
	if rt == nil {
		rt = http.DefaultTransport
	}
	clone.Transport = &limitedTransport{base: rt, lim: lim}
	return &clone
}

type limitedTransport struct {
	base http.RoundTripper
	lim  *rate.Limiter
}

func (t *limitedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.lim.Wait(req.Context()); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(req)
}

// NewLimitedHTTPClient builds a limited client using the per-source defaults.
func NewLimitedHTTPClient(source string) *http.Client {
	limit, burst := SourceLimits(source)
	return LimitedClient(&http.Client{Timeout: 30 * time.Second}, rate.NewLimiter(limit, burst))
}
