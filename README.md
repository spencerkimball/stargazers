# Stargazers
Analyze GitHub stars

Run it using:

go run main.go --repo=:owner/:repo --token=:token

You can also run the data-fetching or analyses phases a la carte via:

go run main.go fetch --repo=:owner/:repo --token=:token

go run main.go analyze --repo=:owner/:repo

Data fetched from GitHub is cached locally as the original HTTP responses. A more minimal amount of state is saved after the fetch phase is complete and is loaded by the analyze phase, which is considerably more efficient. To clear out all of the cached HTTP responses use:

go run main.go clear

