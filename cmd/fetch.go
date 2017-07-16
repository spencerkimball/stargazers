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

package cmd

import (
	"errors"
	"log"

	"github.com/spencerkimball/stargazers/fetch"
	"github.com/spf13/cobra"
)

// FetchCmd recursively fetches stargazer github data.
var FetchCmd = &cobra.Command{
	Use:   "fetch --repo=:owner/:repo --token=:access_token",
	Short: "recursively fetch all stargazer github data",
	Long: `
Recursively fetch all stargazer github data starting with the list of
stargazers for the specified :owner/:repo and then descending into
each stargazer's followers, other starred repos, and subscribed
repos. Each subscribed repo is further queried for that stargazer's
contributions in terms of additions, deletions, and commits. All
fetched data is cached by URL.
`,
	Example: `  stargazers fetch --repo=cockroachdb/cockroach --token=f87456b1112dadb2d831a5792bf2ca9a6afca7bc`,
	RunE:    RunFetch,
}

// RunFetch recursively queries all relevant github data for
// the specified owner and repo.
func RunFetch(cmd *cobra.Command, args []string) error {
	if len(Repo) == 0 {
		return errors.New("repository not specified; use --repo=:owner/:repo")
	}
	token, err := getAccessToken()
	if err != nil {
		return err
	}
	log.Printf("fetching GitHub data for repository %s", Repo)
	fetchCtx := &fetch.Context{
		Repo:     Repo,
		Token:    token,
		CacheDir: CacheDir,
	}
	if err := fetch.QueryAll(fetchCtx); err != nil {
		log.Printf("failed to query stargazer data: %s", err)
		return nil
	}
	return nil
}
