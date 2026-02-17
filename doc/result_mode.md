# Result Mode

There are two main types of API calls required for an application to query Athena and retrieve the results of a query.

- Execute query
- Get the result of query execution (Rows of SELECT statement)

![Overview](https://user-images.githubusercontent.com/301822/100542326-90bc4580-328c-11eb-8102-692d4e414809.jpg)

The structure of Get Result depends on the Result mode.
Originally [go-athena](https://github.com/segmentio/go-athena) got the query result only by API access, but DL mode and GZIP DL mode are provided as follows.

- API mode (default)
- DL mode
- GZIP DL mode

However, DL mode and GZIP DL mode can be used only in the Select statement.

## API mode

To get the result, go to [GetQueryResults APIg](https://docs.aws.amazon.com/athena/latest/APIReference/API_GetQueryResults.html) and get the result.
This API access has a limit on the number of cases that can be returned in the response when returning the query result by API. If the query result exceeds [Maximum number (maximum 1000)](https://docs.aws.amazon.com/athena/latest/APIReference/API_GetQueryResults.html), all query results will be displayed by multiple API accesses. Get it.

![API Mode](https://user-images.githubusercontent.com/301822/100542359-bfd2b700-328c-11eb-8a5d-77c268d1c7fa.jpg)

## DL mode

Athena saves all query results as a csv file, so you can download and get it.
The csv file is uncompressed.
By downloading the file, you can get the query result with 1 API access regardless of the number of cases.

![DL Mode](https://user-images.githubusercontent.com/301822/100542394-f27caf80-328c-11eb-9800-2130e65eccbf.jpg)

- Note
  - It's used only in the Select statement.

## GZIP DL mode

The DL mode csv file was an uncompressed file download.
It is possible to compress the download file to gzip by using the CTAS table.

- CTAS table creation by query (GZIP specified)
- Downloading and decompressing CTAS table data
- Delete CTAS table

![GZIP DL Mode](https://user-images.githubusercontent.com/301822/100542438-27890200-328d-11eb-9f03-1688b29936c6.jpg)

- Note
  - It's used only in the Select statement.
  - Column Type is different compared to the other 2 modes.

|Result Mode|How to get column type|Column|Column|Column|
|---|---|---|---|---|
|API, DL|[ResultSet.ResultSetMetadata.ColumnInfo.Type](https://docs.aws.amazon.com/ja_jp/athena/latest/APIReference/API_GetQueryResults.html#API_GetQueryResults_ResponseSyntax)|varchar|integer|demical|
|GZIP DL|[TableMetadata.Columns.Type](https://docs.aws.amazon.com/ja_jp/athena/latest/APIReference/API_GetTableMetadata.html#API_GetTableMetadata_ResponseSyntax)|string|int|demical(numner, numner)|

## Response time for each mode

It is a comparison of the time taken from executing the query in the actual results to acquiring all the results.

<img width="788" alt="response_time" src="https://user-images.githubusercontent.com/301822/100542783-97988780-328f-11eb-9153-9623b4618c06.png">
<br/>

I think the following trends can be said.
- DL mode and API mode are effective for a small number of cases
- GZIP DL mode is very effective for a large number of cases

### Comparison with segmentio/go-athena

|package|response time (500000 rows)|response time (1000000 rows)|response time (5000000 rows)|
|--|--|--|--|
|segmentio/go-athena|1m 19.8935445s|2m 33.4132205s|12m 36.8392345s|
|speee/go-athena API mode|1m 18.9925315s|2m 26.475804292s|12m 25.008203542s|
|speee/go-athena DL mode|12.721702583s|20.719727417s|1m 23.689167708s|
|speee/go-athena GZIP mode|15.8399715s|17.661648209s|43.873663666s|

## Usages

### Setting in Configuration

```
# DL Mode
db, err := sql.Open("athena", "db=xxxx&output_location=s3://xxxxxxx&region=xxxxxx&result_mode=dl")

# GZIP DL Mode
db, err := sql.Open("athena", "db=xxxx&output_location=s3://xxxxxxx&region=xxxxxx&result_mode=gzip")
```

### Setting in Context

You can change the Result Mode for each SQL.
Settings in context override Configuration settings.

```
# API Mode
ctx = SetAPIMode(ctx)

# DL Mode
ctx = SetDLMode(ctx)

# GZIP DL Mode
ctx = SetGzipDLMode(ctx)
```
