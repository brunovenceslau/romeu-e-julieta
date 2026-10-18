// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package pin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
)

// maxBody is the most a response may hold: every file kitpin reads is
// a few kilobytes, so a larger one is refused rather than read.
const maxBody = 4 << 20

// maxRedirects is the most redirects one request follows.
const maxRedirects = 5

// allowed returns an error unless u is https to a host of src.
func allowed(src Source, u *url.URL) error {
	if u.Scheme != "https" {
		return fmt.Errorf("%s is not https", u.Redacted())
	}
	if !slices.Contains(src.Hosts, u.Host) {
		return fmt.Errorf("%s is not on a host of this source %v", u.Redacted(), src.Hosts)
	}
	return nil
}

// fetch GETs path below the base URL of src with the given headers and
// returns the body and the headers of a 200 response; any other status
// is an error naming it. The request and each redirect must be https
// to a host of src.
func fetch(ctx context.Context, e Env, src Source, path string, headers map[string]string) ([]byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.Base+path, nil)
	if err != nil {
		return nil, nil, err
	}
	where := req.URL.Redacted()
	if err := allowed(src, req.URL); err != nil {
		return nil, nil, fmt.Errorf("GET refused: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := *e.Client
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) > maxRedirects {
			return fmt.Errorf("more than %d redirects", maxRedirects)
		}
		if err := allowed(src, next.URL); err != nil {
			return fmt.Errorf("redirect refused: %w", err)
		}
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err
		}
		return nil, nil, fmt.Errorf("GET %s: %w", where, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, nil, fmt.Errorf("GET %s: %w", where, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("GET %s: %s", where, resp.Status)
	}
	if len(body) > maxBody {
		return nil, nil, fmt.Errorf("GET %s: the response is over %d bytes", where, maxBody)
	}
	return body, resp.Header, nil
}
