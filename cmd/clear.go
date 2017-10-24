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

// ClearCmd clears cached GitHub API responses.
var ClearCmd = &cobra.Command{
	Use:   "clear --repo=:owner/:repo",
	Short: "clear cached GitHub API responses",
	Long: `
Clears all GitHub API responses which have been cached in the repo-specific
--cache subdirectory.
`,
	Example: `  stargazers clear --repo=cockroachdb/cockroach`,
	RunE:    RunClear,
}

// RunClear clears all cached GitHub API responses for the specified repo.
func RunClear(cmd *cobra.Command, args []string) error {
	if len(Repo) == 0 {
		return errors.New("repository not specified; use --repo=:owner/:repo")
	}
	log.Printf("clearing GitHub API response cache for repository %s", Repo)
	fetchCtx := &fetch.Context{
		Repo:     Repo,
		CacheDir: CacheDir,
	}
	if err := fetch.Clear(fetchCtx); err != nil {
		log.Printf("failed to clear cached responses: %s", err)
		return nil
	}
	return nil
}
