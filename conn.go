package athena

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	uuid "github.com/satori/go.uuid"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/athena/types"
)

// Query type patterns
var (
	ddlQueryPattern    = regexp.MustCompile(`(?i)^(ALTER|CREATE|DESCRIBE|DROP|MSCK|SHOW)`)
	selectQueryPattern = regexp.MustCompile(`(?i)^SELECT`)
	ctasQueryPattern   = regexp.MustCompile(`(?i)^CREATE.+AS\s+SELECT`)
)

// queryType represents the type of SQL query
type queryType int

const (
	queryTypeUnknown queryType = iota
	queryTypeDDL
	queryTypeSelect
	queryTypeCTAS
)

// getQueryType determines the type of the query
func getQueryType(query string) queryType {
	switch {
	case ddlQueryPattern.MatchString(query):
		return queryTypeDDL
	case ctasQueryPattern.MatchString(query):
		return queryTypeCTAS
	case selectQueryPattern.MatchString(query):
		return queryTypeSelect
	default:
		return queryTypeUnknown
	}
}

// isDDLQuery determines if the query is a DDL statement
func isDDLQuery(query string) bool {
	return getQueryType(query) == queryTypeDDL
}

// isSelectQuery determines if the query is a SELECT statement
func isSelectQuery(query string) bool {
	return getQueryType(query) == queryTypeSelect
}

// isCTASQuery determines if the query is a CREATE TABLE AS SELECT statement
func isCTASQuery(query string) bool {
	return getQueryType(query) == queryTypeCTAS
}

type conn struct {
	athena         *athena.Client
	db             string
	OutputLocation string
	workgroup      string

	pollFrequency time.Duration

	resultMode ResultMode
	config     aws.Config
	timeout    uint
	catalog    string
}

func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if len(args) > 0 {
		panic("Athena doesn't support prepared statements. Format your own arguments.")
	}

	rows, err := c.runQuery(ctx, query)
	return rows, err
}

func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if len(args) > 0 {
		panic("Athena doesn't support prepared statements. Format your own arguments.")
	}

	_, err := c.runQuery(ctx, query)
	return nil, err
}

func (c *conn) runQuery(ctx context.Context, query string) (driver.Rows, error) {
	// result mode
	isSelect := isSelectQuery(query)
	resultMode := c.resultMode
	if rmode, ok := getResultMode(ctx); ok {
		if !isValidResultMode(rmode) {
			return nil, ErrInvalidResultMode
		}
		resultMode = rmode
	}
	if !isSelect {
		resultMode = ResultModeAPI
	}

	// timeout
	timeout := c.timeout
	if to, ok := getTimeout(ctx); ok {
		timeout = to
	}

	// catalog
	catalog := c.catalog
	if cat, ok := getCatalog(ctx); ok {
		catalog = cat
	}

	// output location (with empty value)
	if checkOutputLocation(resultMode, c.OutputLocation) {
		var err error
		c.OutputLocation, err = getOutputLocation(c.athena, c.workgroup)
		if err != nil {
			return nil, err
		}
	}

	// mode ctas
	var ctasTable string
	var afterDownload func() error
	if isCreatingCTASTable(isSelect, resultMode) {
		// Create AS Select
		ctasTable = fmt.Sprintf("tmp_ctas_%v", strings.Replace(uuid.NewV4().String(), "-", "", -1))
		query = fmt.Sprintf("CREATE TABLE %s WITH (format='TEXTFILE') AS %s", ctasTable, query)
		afterDownload = c.dropCTASTable(ctx, ctasTable)
	}

	queryID, err := c.startQuery(ctx, query)
	if err != nil {
		return nil, err
	}

	if err := c.waitOnQuery(ctx, queryID); err != nil {
		return nil, err
	}

	return newRows(rowsConfig{
		Athena:         c.athena,
		QueryID:        queryID,
		SkipHeader:     !isDDLQuery(query),
		ResultMode:     resultMode,
		Config:         c.config,
		OutputLocation: c.OutputLocation,
		Timeout:        timeout,
		AfterDownload:  afterDownload,
		CTASTable:      ctasTable,
		DB:             c.db,
		Catalog:        catalog,
	})
}

func (c *conn) dropCTASTable(ctx context.Context, table string) func() error {
	return func() error {
		query := fmt.Sprintf("DROP TABLE %s", table)

		queryID, err := c.startQuery(ctx, query)
		if err != nil {
			return err
		}

		return c.waitOnQuery(ctx, queryID)
	}
}

// startQuery starts an Athena query and returns its ID.
func (c *conn) startQuery(ctx context.Context, query string) (string, error) {
	// resolve catalog from context, fallback to connection-level catalog
	catalog := c.catalog
	if cat, ok := getCatalog(ctx); ok {
		catalog = cat
	}

	var catalogPtr *string
	if catalog != "" {
		catalogPtr = aws.String(catalog)
	}

	resp, err := c.athena.StartQueryExecution(ctx, &athena.StartQueryExecutionInput{
		QueryString: aws.String(query),
		QueryExecutionContext: &types.QueryExecutionContext{
			Database: aws.String(c.db),
			Catalog:  catalogPtr,
		},
		ResultConfiguration: &types.ResultConfiguration{
			OutputLocation: aws.String(c.OutputLocation),
		},
		WorkGroup: aws.String(c.workgroup),
	})
	if err != nil {
		return "", err
	}

	return *resp.QueryExecutionId, nil
}

// waitOnQuery blocks until a query finishes, returning an error if it failed.
func (c *conn) waitOnQuery(ctx context.Context, queryID string) error {
	for {
		statusResp, err := c.athena.GetQueryExecution(ctx, &athena.GetQueryExecutionInput{
			QueryExecutionId: aws.String(queryID),
		})
		if err != nil {
			return err
		}

		switch statusResp.QueryExecution.Status.State {
		case types.QueryExecutionStateCancelled:
			return context.Canceled
		case types.QueryExecutionStateFailed:
			reason := *statusResp.QueryExecution.Status.StateChangeReason
			return errors.New(reason)
		case types.QueryExecutionStateSucceeded:
			return nil
		case types.QueryExecutionStateQueued:
		case types.QueryExecutionStateRunning:
		}

		select {
		case <-ctx.Done():
			c.athena.StopQueryExecution(ctx, &athena.StopQueryExecutionInput{
				QueryExecutionId: aws.String(queryID),
			})

			return ctx.Err()
		case <-time.After(c.pollFrequency):
			continue
		}
	}
}

func (c *conn) Prepare(query string) (driver.Stmt, error) {
	return c.prepareContext(context.Background(), query)
}

func (c *conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	stmt, err := c.prepareContext(ctx, query)

	select {
	default:
	case <-ctx.Done():
		stmt.Close()
		return nil, ctx.Err()
	}

	return stmt, err
}

func (c *conn) prepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	// resultMode
	isSelect := isSelectQuery(query)
	resultMode := c.resultMode
	if rmode, ok := getResultMode(ctx); ok {
		resultMode = rmode
	}
	if !isSelect {
		resultMode = ResultModeAPI
	}

	// ctas
	var ctasTable string
	var afterDownload func() error
	if isCreatingCTASTable(isSelect, resultMode) {
		// Create AS Select
		ctasTable = fmt.Sprintf("tmp_ctas_%v", strings.Replace(uuid.NewV4().String(), "-", "", -1))
		query = fmt.Sprintf("CREATE TABLE %s WITH (format='TEXTFILE') AS %s", ctasTable, query)
		afterDownload = c.dropCTASTable(ctx, ctasTable)
	}

	numInput := len(strings.Split(query, "?")) - 1

	// prepare
	prepareKey := fmt.Sprintf("tmp_prepare_%v", strings.Replace(uuid.NewV4().String(), "-", "", -1))
	newQuery := fmt.Sprintf("PREPARE %s FROM %s", prepareKey, query)

	queryID, err := c.startQuery(ctx, newQuery)
	if err != nil {
		return nil, err
	}

	if err := c.waitOnQuery(ctx, queryID); err != nil {
		return nil, err
	}

	return &stmtAthena{
		prepareKey:    prepareKey,
		numInput:      numInput,
		ctasTable:     ctasTable,
		afterDownload: afterDownload,
		conn:          c,
		resultMode:    resultMode,
	}, nil
}

func (c *conn) Begin() (driver.Tx, error) {
	panic("Athena doesn't support transactions")
}

func (c *conn) Close() error {
	return nil
}

var _ driver.QueryerContext = (*conn)(nil)
var _ driver.ExecerContext = (*conn)(nil)

// HACK(tejasmanohar): database/sql calls Prepare() if your driver doesn't implement
// Queryer. Regardless, db.Query/Exec* calls Query/Exec-Context so I've filed a bug--
// https://github.com/golang/go/issues/22980.
func (c *conn) Query(query string, args []driver.Value) (driver.Rows, error) {
	panic("Query() is noop")
}

func (c *conn) Exec(query string, args []driver.Value) (driver.Result, error) {
	panic("Exec() is noop")
}

var _ driver.Queryer = (*conn)(nil)
var _ driver.Execer = (*conn)(nil)

func isCreatingCTASTable(isSelect bool, resultMode ResultMode) bool {
	return isSelect && resultMode == ResultModeGzipDL
}

// isValidResultMode checks if the given result mode is valid
func isValidResultMode(mode ResultMode) bool {
	switch mode {
	case ResultModeAPI, ResultModeDL, ResultModeGzipDL:
		return true
	default:
		return false
	}
}
