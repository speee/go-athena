[![](https://godoc.org/github.com/speee/go-athena?status.svg)](https://godoc.org/github.com/speee/go-athena)

**This is forked from [segmentio/go-athena](https://github.com/segmentio/go-athena) and we described what changes we added [here](./doc/introduction.md).**

# go-athena

go-athena is a simple Golang [database/sql] driver for [Amazon Athena](https://aws.amazon.com/athena/).

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

It provides a higher-level, idiomatic wrapper over the
[AWS Go SDK](https://docs.aws.amazon.com/sdk-for-go/api/service/athena/),
comparable to the [Athena JDBC driver](http://docs.aws.amazon.com/athena/latest/ug/athena-jdbc-driver.html)
AWS provides for Java users.

For example,

- Instead of manually parsing types from strings, you can use [database/sql.Rows.Scan()](https://golang.org/pkg/database/sql/#Rows.Scan)
- Instead of reaching for semaphores, you can use [database/sql.DB.SetMaxOpenConns](https://golang.org/pkg/database/sql/#DB.SetMaxOpenConns)
- And, so on...


## Caveats

[database/sql] exposes lots of methods that aren't supported in Athena.
For example, Athena doesn't support transactions so `Begin()` is irrelevant.
If a method must be supplied to satisfy a standard library interface but is unsupported,
the driver will **panic** indicating so. If there are new offerings in Athena and/or
helpful additions, feel free to PR.

## Result Mode

go-athena has the following modes to get the result of the query.

- API (default)
- DL
- GZIP DL

Note

- DL and GZIP DL Mode are used only in the Select statement.
  - Other statements automatically use API mode under DL or GZIP DL Mode.
- Detailed explanation is described [here](doc/result_mode.md).
- [Usages of Result Mode](doc/result_mode.md#usages).

## Performance

Response time for fetching 1,000,000 records

|package|response time|
|--|--|
|segmentio/go-athena|2m 33.4132205s|
|speee/go-athena API mode|2m 26.475804292s|
|speee/go-athena DL mode|20.719727417s|
|speee/go-athena GZIP mode|17.661648209s|

Detailed explanation is described [here](doc/result_mode.md#response-time-for-each-mode).

## Prepared Statements

You can use [Athena Prepared Statements](https://docs.aws.amazon.com/athena/latest/ug/querying-with-prepared-statements.html).
Click [here](doc/prepared_statements.md) for details on how to use.

## Testing

Athena doesn't have a local version and revolves around S3 so our tests are
integration tests against AWS itself. Thus, our tests require AWS credentials.
The simplest way to provide them is via `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`
environment variables, but you can use anything supported by the
[Default Credential Provider Chain].

The tests support a few environment variables:
- `ATHENA_DATABASE` can be used to override the default database "go_athena_tests"
- `S3_BUCKET` can be used to override the default S3 bucket of "go-athena-tests"
- `ATHENA_REGION` or `AWS_DEFAULT_REGION` can be used to override the default region of "us-east-1"
- `ATHENA_WORK_GROUP` can be used to override the default workgroup of "primary"


[database/sql]: https://golang.org/pkg/database/sql/
[Default Credential Provider Chain]: http://docs.aws.amazon.com/sdk-for-java/v1/developer-guide/credentials.html#credentials-default
