// Copyright 2016 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.
//
// Author: Spencer Kimball (spencer.kimball@gmail.com)

package fetch

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

// A rateLimitError is returned when the requestor's rate limit has
// been exceeded.
type rateLimitError struct {
	resetUnix int64 // Unix seconds at which rate limit is reset
}

// Error implements the error interface.
func (rle *rateLimitError) Error() string {
	reset := time.Unix(rle.resetUnix, 0 /* nanos */).Local()
	return fmt.Sprintf("rate limit for GitHub API access using this user token "+
		"has been exceeded; resets at %s (in %s)", reset, rle.expiration())
}

// expiration returns the duration until the rate limit regime expires,
// including 1 second of padding to account of clock offset.
func (rle *rateLimitError) expiration() time.Duration {
	return time.Unix(rle.resetUnix, 0).Add(1 * time.Second).Sub(time.Now())
}

// An httpError specifies a non-200 http response code.
type httpError struct {
	req  *http.Request
	resp *http.Response
}

// Error implements the error interface.
func (e *httpError) Error() string {
	return fmt.Sprintf("failed to fetch (req: %s): %s", e.req, e.resp)
}

// linkRE provides parsing of the "Link" HTTP header directive.
var linkRE = regexp.MustCompile(`^<(.*)>; rel="next", <(.*)>; rel="last".*`)

// fetchURL fetches the specified URL. The cache (specified in
// c.CacheDir) is consulted first and if not found, the specified URL
// is fetched using the HTTP client. The refresh bool indicates
// whether the last page of results should be refreshed if it's found
// in the response cache. Returns the next URL if the result is paged
// or an error on failure.
func fetchURL(c *Context, url string, value interface{}, refresh bool) (string, error) {
	// Create request and add mandatory user agent and accept encoding headers.
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("User-Agent", "Cockroach Labs Stargazers App")
	req.Header.Add("Accept-Encoding", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("token %s", c.Token))
	if len(c.acceptHeader) > 0 {
		req.Header.Add("Accept", c.acceptHeader)
	}

	// Check the response cache first.
	cached := true // assume true
	var next string
	var resp *http.Response
	resp, err = getCache(c, req)
	if err != nil {
		return "", errors.New(fmt.Sprintf("getCache URL=%q: %s", url, err))
	}

	// We loop until we have a next URL or we've gotten a direct result
	// by fetching from the server; the last result might change between
	// runs, so it must be rechecked.
	for {
		// If not found, fetch the URL from the GitHub API server.
		if resp == nil {
			cached = false
			// Maximum 20 retries.
			for i := uint(0); i < 10; i++ {
				resp, err = doFetch(c, url, req)
				if err == nil {
					break
				}
				switch t := err.(type) {
				case *rateLimitError:
					// Sleep until the expiration of the rate limit regime (+ 1s for clock offsets).
					log.Printf("%s", t)
					time.Sleep(t.expiration())
				case *httpError:
					// For now, regard HTTP errors as permanent.
					log.Printf("unable to fetch %q: %s", url, err)
					return "", nil
				default:
					// Retry with exponential backoff on random connection and networking errors.
					log.Printf("%s", t)
					backoff := int64((1 << i)) * 50000000 // nanoseconds, starting at 50ms
					if backoff > 1000000000 {
						backoff = 1000000000
					}
					time.Sleep(time.Duration(backoff))
				}
			}
		}
		if resp == nil {
			log.Printf("unable to fetch %q", url)
			return "", nil
		}

		// Parse the next link, if available.
		if link := resp.Header.Get("Link"); len(link) > 0 {
			urls := linkRE.FindStringSubmatch(link)
			if urls != nil {
				next = urls[1]
			}
		}
		// If there is no next page and we used cached; clear resp for
		// explicit re-fetch.
		if len(next) > 0 || !cached || !refresh {
			break
		}
		resp.Body.Close() // Don't forget to close the body
		resp = nil
	}

	// Parse the body from JSON string into the supplied go struct.
	var body []byte
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		// In the event corruption of the cached entry was encountered, clear
		// the entry and try again.
		log.Printf("cache entry %q corrupted; removing and refetching", url)
		clearEntry(c, url)
		return fetchURL(c, url, value, refresh)
	}
	if err = json.Unmarshal(body, value); err != nil {
		return "", errors.New(fmt.Sprintf("unmarshal URL=%q: %s", url, err))
	}
	return next, nil
}

// doFetch performs the GET https request and stores the result in the
// cache on success. A rateLimitError is returned in the event that
// the access token has exceeded its hourly limit.
func doFetch(c *Context, url string, req *http.Request) (*http.Response, error) {
	log.Printf("fetching %q...", url)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case 200:
		// Success!
		if err := putCache(c, req, resp); err == nil {
			return resp, nil
		}
	case 202: // Accepted
		// This is a weird one, but it's been returned by GitHub before.
		err = errors.New("202 (Accepted) HTTP response; backoff and retry")
	case 403: // Forbidden...handle case of rate limit exception
		if limitRem := resp.Header.Get("X-rateLimit-Remaining"); len(limitRem) > 0 {
			var remaining, resetUnix int
			if remaining, err = strconv.Atoi(limitRem); err == nil && remaining == 0 {
				if limitReset := resp.Header.Get("X-rateLimit-Reset"); len(limitReset) > 0 {
					if resetUnix, err = strconv.Atoi(limitReset); err == nil {
						err = &rateLimitError{resetUnix: int64(resetUnix)}
						resp.Body.Close()
						return nil, err
					}
				}
			}
		}
		fallthrough
	default:
		err = &httpError{req, resp}
	}
	resp.Body.Close()
	return nil, err
}
