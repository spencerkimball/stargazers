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

package main

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/spencerkimball/stargazers/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// pflagValue wraps flag.Value and implements the extra methods of the
// pflag.Value interface.
type pflagValue struct {
	flag.Value
}

func (v pflagValue) Type() string {
	t := reflect.TypeOf(v.Value).Elem()
	return t.Kind().String()
}

func (v pflagValue) IsBoolFlag() bool {
	t := reflect.TypeOf(v.Value).Elem()
	return t.Kind() == reflect.Bool
}

func normalizeStdFlagName(s string) string {
	return strings.Replace(s, "_", "-", -1)
}

var stargazersCmd = &cobra.Command{
	Use:   "stargazers :owner/:repo --token=:access_token",
	Short: "illuminate your GitHub community by delving into your repo's stars",
	Long: `
GitHub allows visitors to star a repo to bookmark it for later
perusal. Stars represent a casual interest in a repo, and when enough
of them accumulate, it's natural to wonder what's driving interest.
Stargazers attempts to get a handle on who these users are by finding
out what else they've starred, which other repositories they've
contributed to, and who's following them on GitHub.

Basic starting point:

1. List all stargazers
2. Fetch user info for each stargazer
3. For each stargazer, get list of starred repos & subscriptions
4. For each stargazer subscription, query the repo statistics to
   get additions / deletions & commit counts for that stargazer
5. Run analyses on stargazer data
`,
	Example: `  stargazers cockroachdb/cockroach --token=f87456b1112dadb2d831a5792bf2ca9a6afca7bc`,
	RunE:    runStargazers,
}

func runStargazers(c *cobra.Command, args []string) error {
	if err := cmd.RunFetch(cmd.FetchCmd, args); err != nil {
		return err
	}
	if err := cmd.RunAnalyze(cmd.AnalyzeCmd, args); err != nil {
		return err
	}
	return nil
}

var genDocCmd = &cobra.Command{
	Use:   "gendoc",
	Short: "generate markdown documentation",
	Long: `
Generate markdown documentation
`,
	Example: `  stargazers gendoc`,
	RunE:    runGenDoc,
}

func runGenDoc(c *cobra.Command, args []string) error {
	return doc.GenMarkdown(stargazersCmd, os.Stdout)
}

func init() {
	stargazersCmd.AddCommand(
		cmd.AnalyzeCmd,
		cmd.ClearCmd,
		cmd.FetchCmd,
		genDocCmd,
	)
	// Map any flags registered in the standard "flag" package into the
	// top-level command.
	pf := stargazersCmd.PersistentFlags()
	flag.VisitAll(func(f *flag.Flag) {
		pf.Var(pflagValue{f.Value}, normalizeStdFlagName(f.Name), f.Usage)
	})
	// Add persistent flags to the top-level command.
	stargazersCmd.PersistentFlags().StringVarP(&cmd.Repo, "repo", "r", "", cmd.RepoDesc)
	stargazersCmd.PersistentFlags().StringVarP(&cmd.AccessToken, "token", "t", "", cmd.AccessTokenDesc)
	stargazersCmd.PersistentFlags().StringVarP(&cmd.CacheDir, "cache", "c", "./stargazer_cache", cmd.CacheDirDesc)
}

// Run ...
func Run(args []string) error {
	stargazersCmd.SetArgs(args)
	return stargazersCmd.Execute()
}

func main() {
	if err := Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "failed running command %q: %v", os.Args[1:], err)
		os.Exit(1)
	}
}
