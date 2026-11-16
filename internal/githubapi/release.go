package githubapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Releases asks for a public repository's releases without a token
// (nothing of the user's is sent), each question with its last answer's
// ETag: asked again unchanged, GitHub answers 304, which costs nothing.
// Without a token GitHub allows an address 60 requests an hour, and an
// office or a VPN is one address: refused for that, the same question
// goes to the repository's pages on github.com (release_web.go), which
// that limit doesn't count.
type Releases struct {
	Repo      string            // "owner/name"
	Base      string            // REST's base; "" for api.github.com
	Web       string            // the pages' base; "" for https://github.com
	Transport http.RoundTripper // nil: http.DefaultTransport

	now    func() time.Time // nil: time.Now
	mu     sync.Mutex
	latest asked[string]
	tags   asked[[]string]
}

// freshKey marks a context whose questions want GitHub's answer as it
// stands now ([Fresh]).
type freshKey struct{}

// Fresh marks ctx's questions to Releases as ones that want GitHub's answer
// as it stands now, not a cached one: someone asked for it (Check for
// updates). Right after a release is published GitHub's caches, the
// releases feed's above all, can still answer without it for minutes, and
// a 304 to the last answer's ETag can be such an answer; a fresh question
// sends no ETag and a query no cache has seen. Each costs one of the
// address's 60 an hour, so only a person's asking is fresh.
func Fresh(ctx context.Context) context.Context { return context.WithValue(ctx, freshKey{}, true) }

// wantsFresh says whether ctx was marked [Fresh].
func wantsFresh(ctx context.Context) bool { return ctx.Value(freshKey{}) != nil }

// uncached is u with a query no cache has seen ("_=<now>") when ctx wants
// a fresh answer, else u.
func uncached(ctx context.Context, u string, now time.Time) string {
	if !wantsFresh(ctx) {
		return u
	}
	sep := "?"
	if strings.Contains(u, "?") {
		sep = "&"
	}
	return u + sep + "_=" + strconv.FormatInt(now.UnixNano(), 10)
}

// asked is what GitHub last gave to one question, with its ETag.
type asked[T any] struct {
	etag string
	got  T
}

// Latest is the tag ("v0.1.17") of the repository's latest release
// (GitHub's: the newest that is neither a draft nor a pre-release);
// ErrNotFound when it has none.
func (r *Releases) Latest(ctx context.Context) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var body struct {
		Tag string `json:"tag_name"`
	}
	fresh, err := r.get(ctx, "/releases/latest", &r.latest.etag, &body)
	if networkLimited(err) {
		return viaPages(ctx, err, r.webLatest)
	}
	if err != nil {
		return "", err
	}
	if fresh {
		r.latest.got = body.Tag
	}
	return r.latest.got, nil
}

// Tags is the tags of the repository's newest releases (GitHub's first
// ten, the newest first), pre-releases among them, drafts not: what a
// beta version is looked for in.
func (r *Releases) Tags(ctx context.Context) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var body []struct {
		Tag   string `json:"tag_name"`
		Draft bool   `json:"draft"`
	}
	fresh, err := r.get(ctx, "/releases?per_page=10", &r.tags.etag, &body)
	if networkLimited(err) {
		return viaPages(ctx, err, r.webTags)
	}
	if err != nil {
		return nil, err
	}
	if fresh {
		r.tags.got = nil
		for _, b := range body {
			if !b.Draft {
				r.tags.got = append(r.tags.got, b.Tag)
			}
		}
	}
	return slices.Clone(r.tags.got), nil
}

// get asks GitHub for path under the repository, with etag when it has
// one, and reads a new answer into out, keeping its ETag; fresh is false
// when GitHub says the last answer stands (304).
func (r *Releases) get(ctx context.Context, path string, etag *string, out any) (fresh bool, err error) {
	owner, name, err := SplitRepo(r.Repo)
	if err != nil {
		return false, err
	}
	base := r.Base
	if base == "" {
		base, _ = endpoints("github.com")
	}
	u := uncached(ctx, base+"/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name)+path, r.clock())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-Github-Api-Version", Version)
	if *etag != "" && !wantsFresh(ctx) {
		req.Header.Set("If-None-Match", *etag)
	}
	resp, err := (&http.Client{Transport: r.Transport, Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		if ue, ok := errors.AsType[*url.Error](err); ok {
			err = ue.Err // its message has the URL
		}
		return false, fmt.Errorf("can't reach GitHub: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only
	switch {
	case resp.StatusCode == http.StatusNotModified && *etag != "":
		return false, nil
	case resp.StatusCode == http.StatusNotFound:
		return false, ErrNotFound
	case resp.StatusCode != http.StatusOK:
		if e := rateRefusal(resp, r.clock()); e != nil {
			return false, e
		}
		return false, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	// Ten releases with their notes and files' records are a few hundred kB.
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out); err != nil {
		return false, fmt.Errorf("reading GitHub's answer: %w", err)
	}
	*etag = resp.Header.Get("ETag")
	return true, nil
}
