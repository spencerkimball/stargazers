## stargazers

illuminate your GitHub community by delving into your repo's stars

### Synopsis



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


```
stargazers :owner/:repo --token=:access_token
```

### Examples

```
  stargazers cockroachdb/cockroach --token=f87456b1112dadb2d831a5792bf2ca9a6afca7bc
```

### Options

```
      --alsologtostderr    logs at or above this threshold go to stderr (default NONE)
  -c, --cache string       directory for storing cached GitHub API responses (default "./stargazer_cache")
      --log-backtrace-at   when logging hits line file:N, emit a stack trace (default :0)
      --log-dir            if non-empty, write log files in this directory (default /var/folders/83/r_nmcwd969g5qc0b7my9wl900000gn/T/)
      --logtostderr        log to standard error instead of files (default true)
      --no-color           disable standard error log colorization
  -r, --repo string        GitHub owner and repository, formatted as :owner/:repo
  -t, --token string       GitHub access token for authorized rate limits
      --verbosity          log level for V logs
      --vmodule            comma-separated list of pattern=N settings for file-filtered logging
```
