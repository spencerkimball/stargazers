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

	"github.com/spencerkimball/stargazers/analyze"
	"github.com/spencerkimball/stargazers/fetch"
	"github.com/spf13/cobra"
)

// AnalyzeCmd analyzes previously fetched GitHub stargazer data.
var AnalyzeCmd = &cobra.Command{
	Use:   "analyze --repo=:owner/:repo",
	Short: "analyze previously fetched and saved GitHub stargazer data",
	Long: `

Analyzes the previously fetched and saved GitHub stargazer data. The
following analyses are run:

    - Cumulative stars (week timestamp and star count)
    - Correlated repos (count of occurrences of other starred & subscribed repos)
    - Correlation histogram (50 bins of occurrence counts)
    - Stargazer report (name, email(?), date starred, correlation score,
      correlated repos, raw activity, raw activity repos, correlated activity,
      correlated activity repos
`,
	Example: `  stargazers analyze --repo=cockroachdb/cockroach`,
	RunE:    RunAnalyze,
}

// RunAnalyze fetches saved stargazer info for the specified repo and
// runs the analysis reports.
func RunAnalyze(cmd *cobra.Command, args []string) error {
	if len(Repo) == 0 {
		return errors.New("repository not specified; use --repo=:owner/:repo")
	}
	log.Printf("fetching saved GitHub stargazer data for repository %s", Repo)
	fetchCtx := &fetch.Context{
		Repo:     Repo,
		CacheDir: CacheDir,
	}
	sg, rs, err := fetch.LoadState(fetchCtx)
	if err != nil {
		log.Printf("failed to load saved stargazer data: %s", err)
		return nil
	}
	log.Printf("analyzing GitHub data for repository %s", Repo)
	if err := analyze.RunAll(fetchCtx, sg, rs); err != nil {
		log.Printf("failed to query stargazer data: %s", err)
		return nil
	}
	return nil
}
