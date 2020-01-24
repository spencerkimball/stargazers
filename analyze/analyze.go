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

package analyze

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/netdata/stargazers/fetch"
)

const (
	// nMostCorrelated is the number of correlated starred or subscribed
	// repos to include in the csv output.
	nMostCorrelated = 50
)

type Stargazers []*fetch.Stargazer

func (slice Stargazers) Len() int {
	return len(slice)
}

func (slice Stargazers) Less(i, j int) bool {
	return slice[i].StarredAt < slice[j].StarredAt
}

func (slice Stargazers) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

type RepoCount struct {
	name  string
	count int
}

type RepoCounts []*RepoCount

func (slice RepoCounts) Len() int {
	return len(slice)
}

func (slice RepoCounts) Less(i, j int) bool {
	return slice[i].count > slice[j].count /* descending order */
}

func (slice RepoCounts) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

// RunAll runs all analyses.
func RunAll(c *fetch.Context, sg []*fetch.Stargazer, rs map[string]*fetch.Repo) error {
	if err := RunCumulativeStars(c, sg); err != nil {
		return err
	}
	if err := RunAttributesByTime(c, sg, rs); err != nil {
		return err
	}
	return nil
}

// RunCumulativeStars creates a table of date and cumulative
// star count for the provided stargazers.
func RunCumulativeStars(c *fetch.Context, sg []*fetch.Stargazer) error {
	log.Printf("running cumulative stars analysis")

	// Open file and prepare.
	f, err := createFile(c, "cumulative_stars.csv")
	if err != nil {
		return fmt.Errorf("failed to create file: %s", err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"Date", "New", "Cumulative"}); err != nil {
		return fmt.Errorf("failed to write to CSV: %s", err)
	}

	// Sort the stargazers.
	slice := Stargazers(sg)
	sort.Sort(slice)

	// Now accumulate by days.
	lastDay := int64(0)
	total := 0
	count := 0
	for _, s := range slice {
		t, err := time.Parse(time.RFC3339, s.StarredAt)
		if err != nil {
			return err
		}
		day := t.Unix() / int64(60*60*24)
		if day != lastDay {
			if count > 0 {
				t := time.Unix(lastDay*60*60*24, 0)
				if err := w.Write([]string{t.Format("01/02/2006"), strconv.Itoa(count), strconv.Itoa(total)}); err != nil {
					return fmt.Errorf("failed to write to CSV: %s", err)
				}
			}
			lastDay = day
			count = 1
		} else {
			count++
		}
		total++
	}
	if count > 0 {
		t := time.Unix(lastDay*60*60*24, 0)
		if err := w.Write([]string{t.Format("01/02/2006"), strconv.Itoa(count), strconv.Itoa(total)}); err != nil {
			return fmt.Errorf("failed to write to CSV: %s", err)
		}
	}
	w.Flush()
	log.Printf("wrote cumulative stars analysis to %s", f.Name())

	return nil
}

// RunFollowers computes the size of follower networks, as well as
// the count of shared followers.
func RunFollowers(c *fetch.Context, sg []*fetch.Stargazer) error {
	log.Printf("running followers analysis")

	// Open file and prepare.
	f, err := createFile(c, "followers.csv")
	if err != nil {
		return fmt.Errorf("failed to create file: %s", err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"Email", "Name", "Login", "URL", "Avatar URL", "Company", "Location", "Followers", "Shared Followers"}); err != nil {
		return fmt.Errorf("failed to write to CSV: %s", err)
	}

	shared := map[string]int{}
	for _, s := range sg {
		for _, f := range s.Followers {
			shared[f.Login]++
		}
	}

	// For each stargazer, output followers, and shared followers.
	// Now accumulate by days.
	for _, s := range sg {
		sharedCount := 0
		for _, f := range s.Followers {
			if c := shared[f.Login]; c > 1 {
				sharedCount++
			}
		}
		url := fmt.Sprintf("https://github.com/%s", s.Login)
		if err := w.Write([]string{s.Email, s.Name, s.Login, url, s.AvatarURL, s.Company, s.Location, strconv.Itoa(s.User.Followers), strconv.Itoa(sharedCount)}); err != nil {
			return fmt.Errorf("failed to write to CSV: %s", err)
		}
	}
	w.Flush()
	log.Printf("wrote followers analysis to %s", f.Name())

	return nil
}

func createFile(c *fetch.Context, baseName string) (*os.File, error) {
	filename := filepath.Join(c.CacheDir, c.Repo, baseName)
	f, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	return f, nil
}
