# Introduction of speee/go-athena

Recently, we released [speee/go-athena](https://github.com/speee/go-athena), and we would like to introduce it here.

## What is go-athena
`go-athena` is a Golang [database/sql](https://golang.org/pkg/database/sql/) package driver for AWS Athena.

You can use it as follows:

```go
import (
    "database/sql"
    _ "github.com/speee/go-athena"
)

func main() {
  db, _ := sql.Open("athena", "db=default&output_location=s3://results")
  rows, _ := db.Query("SELECT url, code from cloudfront")

  for rows.Next() {
    var url string
    var code int
    rows.Scan(&url, &code)
  }
}
```

This library was originally maintained [here](https://github.com/segmentio/go-athena), but unfortunately, it looks like it has not been maintained actively, then we started maintaining it instead.

## New features

We added the following new features, including importing some of the issues that were originally up:

- Add Docker environment
- Add [reviewdog](https://github.com/reviewdog/reviewdog) configuration for linting
- Remove Result Header when executing DDL
- Use workgroup
- Update tests (make each package up to date)
- Introduce Result Mode (Query result acquisition mode)

**Especially, we think the Result Mode is a cool feature, and we summarized the feature [here](./result_mode.md). Please read it!**

## About the future

We can keep maintaining this library and review / merge your PRs :)
And if the original creator would hope so, we would also be happy to merge our changes into the original repository.

`go-athena` is an awesome package for Golang developers because it's very easy to use Athena via `database/sql` interface.
`speee/go-athena` has new features as well as solving the previous issues, so we hope that the original creator will also like it.
