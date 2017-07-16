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
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// TODO(spencer): this would all benefit from using a GitHub API
//   based in Go. It's all just very ad-hoc at the moment, but wasn't
//   meant to be anything other than a quick and dirty analysis.

const (
	githubAPI     = "https://api.github.com/"
	maxStarred    = 300 // Max starred repos to query per stargazer
	maxSubscribed = 300 // Max subscribed repos to query per stargazer

	// To consider a subscribed repo for a stargazer's contributions,
	// it must match at least one of these thresholds...
	minStargazers = 25
	minForks      = 10
	minOpenIssues = 10
)

// Context holds config information used to query GitHub.
type Context struct {
	Repo     string // Repository (:owner/:repo)
	Token    string // Access token
	CacheDir string // Cache directory

	acceptHeader string // Optional Accept: header value
}

type User struct {
	Login            string `json:"login"`
	ID               int    `json:"id"`
	AvatarURL        string `json:"avatar_url"`
	GravatarID       string `json:"gravatar_id"`
	URL              string `json:"url"`
	HtmlURL          string `json:"html_url"`
	FollowersURL     string `json:"followers_url"`
	FollowingURL     string `json:"following_url"`
	StarredURL       string `json:"starred_url"`
	SubscriptionsURL string `json:"subscriptions_url"`
	Type             string `json:"type"`
	SiteAdmin        bool   `json:"site_admin"`
	Name             string `json:"name"`
	Company          string `json:"company"`
	Blog             string `json:"blog"`
	Location         string `json:"location"`
	Email            string `json:"email"`
	Hireable         bool   `json:"hireable"`
	Bio              string `json:"bio"`
	PublicRepos      int    `json:"public_repos"`
	PublicGists      int    `json:"public_gists"`
	Followers        int    `json:"followers"`
	Following        int    `json:"following"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`

	//GistsURL          string `json:"gists_url"`
	//OrganizationsURL  string `json:"organizations_url"`
	//ReposURL          string `json:"repos_url"`
	//EventsURL         string `json:"events_url"`
	//ReceivedEventsURL string `json:"received_events_url"`
}

type Week struct {
	Timestamp int `json:"w"`
	Additions int `json:"a"`
	Deletions int `json:"d"`
	Commits   int `json:"c"`
}

type Contribution struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Commits   int    `json:"commits"`
}

func makeContribution(c *Contributor) *Contribution {
	contrib := &Contribution{
		ID:    c.Author.ID,
		Login: c.Author.Login,
	}
	for _, w := range c.Weeks {
		contrib.Commits += w.Commits
		contrib.Additions += w.Additions
		contrib.Deletions += w.Deletions
	}
	return contrib
}

type Contributor struct {
	Author User   `json:"author"`
	Total  int    `json:"total"`
	Weeks  []Week `json:"weeks"`
}

type Repo struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	FullName        string `json:"full_name"`
	Private         bool   `json:"private"`
	HtmlURL         string `json:"html_url"`
	Fork            bool   `json:"fork"`
	URL             string `json:"url"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	PushedAt        string `json:"pushed_at"`
	Homepage        string `json:"homepage"`
	Size            int    `json:"size"`
	StargazersCount int    `json:"stargazers_count"`
	WatchersCount   int    `json:"watchers_count"`
	Language        string `json:"language"`
	HasIssues       bool   `json:"has_issues"`
	HasDownloads    bool   `json:"has_downloads"`
	HasWiki         bool   `json:"has_wiki"`
	HasPages        bool   `json:"has_pages"`
	ForksCount      int    `json:"forks_count"`
	Forks           int    `json:"forks"`
	OpenIssues      int    `json:"open_issues"`
	Watchers        int    `json:"watchers"`
	DefaultBranch   string `json:"default_branch"`

	//Owner           User   `json:"owner"`
	//Description     string `json:"description"`
	//GitURL          string `json:"git_url"`
	//SshHURL         string `json:"ssh_url"`
	//CloneURL        string `json:"clone_url"`
	//SvnURL          string `json:"svn_url"`
	//MirrorURL       string `json:"mirror_url"`

	// Contributions map from user login to contributor statistics.
	Statistics map[string]*Contribution `json:"statistics"`
}

// meetsThresholds returns whether the repo meets any of the minimal
// thresholds to qualify for contributor statistics querying.
func (r *Repo) meetsThresholds() bool {
	return r.StargazersCount > minStargazers || r.ForksCount > minForks || r.OpenIssues > minOpenIssues
}

// TotalCommits returns the total commits as well as additions
// and deletions.
func (r *Repo) TotalCommits() (int, int, int) {
	c, a, d := 0, 0, 0
	for _, contrib := range r.Statistics {
		c += contrib.Commits
		a += contrib.Additions
		d += contrib.Deletions
	}
	return c, a, d
}

// Stargazer holds all information and further query URLs for a stargazer.
type Stargazer struct {
	User      `json:"user"`
	StarredAt string `json:"starred_at"`

	Followers  []*User  `json:"follower_list"`
	Starred    []string `json:"starred"`    // Slice of repos by full name
	Subscribed []string `json:"subscribed"` // Slice of repos by full name

	// Contributions to subscribed repos (by repo FullName).
	Contributions map[string]*Contribution `json:"contributions"`
}

// Age returns the age (time from current time to created at
// timestamp) of this stargazer in seconds.
func (s *Stargazer) Age() int64 {
	curDay := time.Now().Unix()
	createT, err := time.Parse(time.RFC3339, s.CreatedAt)
	if err != nil {
		log.Printf("failed to parse created at timestamp (%s): %s", s.CreatedAt, err)
		return 0
	}
	return curDay - createT.Unix()
}

// TotalCommits returns the total commits as well as additions and
// deletions, ranged over all tracked contributions.
func (s *Stargazer) TotalCommits() (int, int, int) {
	c, a, d := 0, 0, 0
	for _, contrib := range s.Contributions {
		c += contrib.Commits
		a += contrib.Additions
		d += contrib.Deletions
	}
	return c, a, d
}

// QueryAll recursively descends into GitHub API endpoints, starting
// with the list of stargazers for the repo.
func QueryAll(c *Context) error {
	// Query all stargazers for the repo.
	sg, err := QueryStargazers(c)
	if err != nil {
		return err
	}
	// Query stargazer user info for all stargazers.
	if err = QueryUserInfo(c, sg); err != nil {
		return err
	}
	// Query followers for all stargazers.
	if err = QueryFollowers(c, sg); err != nil {
		return err
	}

	// Unique map of repos by repo full name.
	rs := map[string]*Repo{}

	// Query starred repos for all stargazers.
	if err = QueryStarred(c, sg, rs); err != nil {
		return err
	}
	// Query subscribed repos for all stargazers.
	if err = QuerySubscribed(c, sg, rs); err != nil {
		return err
	}
	// Query contributions to subscribed repos for all stargazers.
	if err = QueryContributions(c, sg, rs); err != nil {
		return err
	}
	return SaveState(c, sg, rs)
}

// QueryStargazers queries the repo's stargazers API endpoint.
// Returns the complete slice of stargazers.
func QueryStargazers(c *Context) ([]*Stargazer, error) {
	cCopy := *c
	cCopy.acceptHeader = "application/vnd.github.v3.star+json"
	log.Printf("querying stargazers of repository %s", c.Repo)
	url := fmt.Sprintf("%srepos/%s/stargazers", githubAPI, c.Repo)
	stargazers := []*Stargazer{}
	var err error
	fmt.Printf("*** 0 stargazers")
	for len(url) > 0 {
		fetched := []*Stargazer{}
		url, err = fetchURL(&cCopy, url, &fetched, true /* refresh last page of results */)
		if err != nil {
			return nil, err
		}
		stargazers = append(stargazers, fetched...)
		fmt.Printf("\r*** %s stargazers", format(len(stargazers)))
	}
	fmt.Printf("\n")
	return stargazers, nil
}

// QueryUserInfo queries user info for each stargazer.
func QueryUserInfo(c *Context, sg []*Stargazer) error {
	log.Printf("querying user info for each of %s stargazers...", format(len(sg)))
	fmt.Printf("*** user info for 0 stargazers")
	for i, s := range sg {
		if _, err := fetchURL(c, s.URL, &s.User, false); err != nil {
			return err
		}
		fmt.Printf("\r*** user info for %s stargazers", format(i+1))
	}
	fmt.Printf("\n")
	return nil
}

// QueryFollowers queries each stargazers list of followers.
func QueryFollowers(c *Context, sg []*Stargazer) error {
	log.Printf("querying followers for each of %s stargazers...", format(len(sg)))
	total := 0
	fmt.Printf("*** 0 followers for 0 stargazers")
	uniqueFollowers := map[int]struct{}{}
	for i, s := range sg {
		var err error
		url := fmt.Sprintf("%s", s.FollowersURL)
		for len(url) > 0 {
			fetched := []*User{}
			url, err = fetchURL(c, url, &fetched, false /* don't refresh followers */)
			if err != nil {
				return err
			}
			for _, u := range fetched {
				uniqueFollowers[u.ID] = struct{}{}
			}
			s.Followers = append(s.Followers, fetched...)
			total += len(fetched)
			fmt.Printf("\r*** %s followers (%s unique) for %s stargazers",
				format(total), format(len(uniqueFollowers)), format(i+1))
		}
	}
	fmt.Printf("\n")
	return nil
}

// QueryStarred queries all starred repos for each stargazer.
func QueryStarred(c *Context, sg []*Stargazer, rs map[string]*Repo) error {
	log.Printf("querying starred repos for each of %s stargazers...", format(len(sg)))
	starred := 0
	fmt.Printf("*** 0 starred repos for 0 stargazers")
	uniqueStarred := map[int]struct{}{}
	for i, s := range sg {
		var err error
		url := s.StarredURL
		url = strings.Replace(url, "{/owner}{/repo}", "", 1)
		for len(url) > 0 && len(s.Starred) < maxStarred {
			fetched := []*Repo{}
			url, err = fetchURL(c, url, &fetched, false /* don't refresh starred repos */)
			if err != nil {
				return err
			}
			for _, r := range fetched {
				if _, ok := rs[r.FullName]; !ok {
					rs[r.FullName] = r
				}
				uniqueStarred[r.ID] = struct{}{}
				s.Starred = append(s.Starred, r.FullName)
			}
			starred += len(fetched)
			fmt.Printf("\r*** %s starred repos (%s unique) for %s stargazers",
				format(starred), format(len(uniqueStarred)), format(i+1))
		}
	}
	fmt.Printf("\n")
	return nil
}

// QuerySubscribed queries all subscribed repos for each stargazer.
func QuerySubscribed(c *Context, sg []*Stargazer, rs map[string]*Repo) error {
	log.Printf("querying subscribed repos for each of %s stargazers...", format(len(sg)))
	subscribed := 0
	fmt.Printf("*** 0 subscribed repos for 0 stargazers")
	uniqueSubscribed := map[int]struct{}{}
	for i, s := range sg {
		var err error
		url := s.SubscriptionsURL
		for len(url) > 0 && len(s.Subscribed) < maxSubscribed {
			fetched := []*Repo{}
			url, err = fetchURL(c, url, &fetched, false /* don't refresh subscribed repos */)
			if err != nil {
				return err
			}
			for _, r := range fetched {
				if _, ok := rs[r.FullName]; !ok {
					rs[r.FullName] = r
				}
				uniqueSubscribed[r.ID] = struct{}{}
				s.Subscribed = append(s.Subscribed, r.FullName)
			}
			subscribed += len(fetched)
			fmt.Printf("\r*** %s subscribed repos (%s unique) for %s stargazers",
				format(subscribed), format(len(uniqueSubscribed)), format(i+1))
		}
	}
	fmt.Printf("\n")
	return nil
}

// QueryContributions queries all contributions to subscribed repos
// for each stargazer.
func QueryContributions(c *Context, sg []*Stargazer, rs map[string]*Repo) error {
	log.Printf("querying contributions to subscribed repos for each of %s stargazers...", format(len(sg)))
	authors := map[string]struct{}{}
	for _, s := range sg {
		authors[s.Login] = struct{}{}
	}
	commits := 0
	subscribed := 0
	qualifying := 0
	uniqueRepos := map[int]struct{}{}
	fmt.Printf("*** 0 commits from 0 repos (0 qual, 0 total) for 0 stargazers")
	for i, s := range sg {
		for _, rName := range s.Subscribed {
			r, ok := rs[rName]
			if !ok {
				log.Fatalf("missing %s repo", rName)
			}
			subscribed++
			if !r.meetsThresholds() {
				continue
			}
			if _, ok := uniqueRepos[r.ID]; !ok {
				uniqueRepos[r.ID] = struct{}{}
			}
			qualifying++
			if r.Statistics == nil {
				if err := QueryStatistics(c, r, authors); err != nil {
					return err
				}
			}
			if contrib, ok := r.Statistics[s.Login]; ok {
				commits += int(contrib.Commits)
				if s.Contributions == nil {
					s.Contributions = map[string]*Contribution{}
				}
				s.Contributions[r.FullName] = contrib
			}
			fmt.Printf("\r*** %s commits from %s repos (%s qual, %s total) for %s stargazers",
				format(commits), format(len(uniqueRepos)), format(qualifying), format(subscribed), format(i+1))
		}
	}
	fmt.Printf("\n")
	return nil
}

// QueryStatistics queries contributor stats for the specified repo.
func QueryStatistics(c *Context, r *Repo, authors map[string]struct{}) error {
	r.Statistics = map[string]*Contribution{}
	var err error
	url := fmt.Sprintf("%srepos/%s/stats/contributors", githubAPI, r.FullName)
	for len(url) > 0 {
		fetched := []*Contributor{}
		url, err = fetchURL(c, url, &fetched, false /* don't refresh */)
		if err != nil {
			return err
		}
		for _, c := range fetched {
			if _, ok := authors[c.Author.Login]; ok {
				r.Statistics[c.Author.Login] = makeContribution(c)
			}
		}
	}
	return nil
}

// SaveState writes all queried stargazer and repo data.
func SaveState(c *Context, sg []*Stargazer, rs map[string]*Repo) error {
	log.Printf("saving state")
	filename := filepath.Join(c.CacheDir, c.Repo, "saved_state")
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	log.Printf("encoding stargazers data")
	if err := enc.Encode(sg); err != nil {
		return errors.New(fmt.Sprintf("failed to encode stargazer data: %s", err))
	}
	log.Printf("encoding repository data")
	if err := enc.Encode(rs); err != nil {
		return errors.New(fmt.Sprintf("failed to encode repo data: %s", err))
	}
	return nil
}

// LoadState reads previously saved queried stargazer and repo data.
func LoadState(c *Context) ([]*Stargazer, map[string]*Repo, error) {
	log.Printf("loading state")
	filename := filepath.Join(c.CacheDir, c.Repo, "saved_state")
	f, err := os.Open(filename)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	sg := []*Stargazer{}
	log.Printf("decoding stargazers data")
	if err := dec.Decode(&sg); err != nil {
		return nil, nil, errors.New(fmt.Sprintf("failed to decode stargazer data: %s", err))
	}
	rs := map[string]*Repo{}
	log.Printf("decoding repository data")
	if err := dec.Decode(&rs); err != nil {
		return nil, nil, errors.New(fmt.Sprintf("failed to decode repo data: %s", err))
	}
	return sg, rs, nil
}

func format(n int) string {
	in := strconv.FormatInt(int64(n), 10)
	out := make([]byte, len(in)+(len(in)-2+int(in[0]/'0'))/3)
	if in[0] == '-' {
		in, out[0] = in[1:], '-'
	}

	for i, j, k := len(in)-1, len(out)-1, 0; ; i, j = i-1, j-1 {
		out[j] = in[i]
		if i == 0 {
			return string(out)
		}
		if k++; k == 3 {
			j, k = j-1, 0
			out[j] = ','
		}
	}
}
