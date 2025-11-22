package chizuql

import (
	"fmt"
	"sort"
	"strings"
)

type queryType string

const (
	queryTypeSelect queryType = "SELECT"
	queryTypeInsert queryType = "INSERT"
	queryTypeUpdate queryType = "UPDATE"
	queryTypeDelete queryType = "DELETE"
	queryTypeRaw    queryType = "RAW"
)

// Query represents a composable SQL query built using the fluent API.
type Query struct {
	qType queryType

	rawSQL  string
	rawArgs []any

	ctes []cte

	selectColumns []Expression
	distinct      bool

	from  TableExpression
	joins []joinClause

	where   Predicate
	groupBy []Expression
	having  Predicate
	orderBy []Expression
	limit   *int
	offset  *int

	insertTable  TableExpression
	insertCols   []string
	insertValues [][]Expression

	updateTable TableExpression
	setClauses  []SetClause

	deleteTable TableExpression

	returning []Expression
}

// New returns a fresh Query instance ready to be composed.
func New() *Query {
	return &Query{}
}

// RawQuery builds a query directly from the provided SQL fragment and arguments.
func RawQuery(sql string, args ...any) *Query {
	return &Query{qType: queryTypeRaw, rawSQL: sql, rawArgs: args}
}

// Select starts a SELECT query.
func (q *Query) Select(columns ...any) *Query {
	q.qType = queryTypeSelect
	q.selectColumns = append(q.selectColumns, toSQLExpressions(columns...)...)
	return q
}

// Distinct marks the SELECT query as DISTINCT.
func (q *Query) Distinct() *Query {
	q.distinct = true
	return q
}

// InsertInto starts an INSERT query.
func (q *Query) InsertInto(table any, columns ...string) *Query {
	q.qType = queryTypeInsert
	q.insertTable = toTableExpression(table)
	q.insertCols = append(q.insertCols, columns...)
	return q
}

// Values appends a values list for an INSERT query.
func (q *Query) Values(values ...any) *Query {
	row := toValueExpressions(values...)
	q.insertValues = append(q.insertValues, row)
	return q
}

// Update starts an UPDATE query.
func (q *Query) Update(table any) *Query {
	q.qType = queryTypeUpdate
	q.updateTable = toTableExpression(table)
	return q
}

// DeleteFrom starts a DELETE query.
func (q *Query) DeleteFrom(table any) *Query {
	q.qType = queryTypeDelete
	q.deleteTable = toTableExpression(table)
	return q
}

// Set adds SET clauses for UPDATE queries.
func (q *Query) Set(clauses ...SetClause) *Query {
	q.setClauses = append(q.setClauses, clauses...)
	return q
}

// From sets the FROM clause.
func (q *Query) From(table any) *Query {
	q.from = toTableExpression(table)
	return q
}

// Join adds an INNER JOIN clause.
func (q *Query) Join(table any, on ...Predicate) *Query {
	return q.join("JOIN", table, on...)
}

// LeftJoin adds a LEFT JOIN clause.
func (q *Query) LeftJoin(table any, on ...Predicate) *Query {
	return q.join("LEFT JOIN", table, on...)
}

// RightJoin adds a RIGHT JOIN clause.
func (q *Query) RightJoin(table any, on ...Predicate) *Query {
	return q.join("RIGHT JOIN", table, on...)
}

// FullJoin adds a FULL JOIN clause.
func (q *Query) FullJoin(table any, on ...Predicate) *Query {
	return q.join("FULL JOIN", table, on...)
}

func (q *Query) join(kind string, table any, on ...Predicate) *Query {
	clause := joinClause{kind: kind, table: toTableExpression(table)}
	if len(on) > 0 {
		clause.on = And(on...)
	}
	q.joins = append(q.joins, clause)
	return q
}

// Where appends predicates to the WHERE clause combined with AND.
func (q *Query) Where(predicates ...Predicate) *Query {
	if len(predicates) == 0 {
		return q
	}

	if q.where == nil {
		q.where = And(predicates...)
		return q
	}

	q.where = And(q.where, And(predicates...))
	return q
}

// Having appends predicates to the HAVING clause combined with AND.
func (q *Query) Having(predicates ...Predicate) *Query {
	if len(predicates) == 0 {
		return q
	}

	if q.having == nil {
		q.having = And(predicates...)
		return q
	}

	q.having = And(q.having, And(predicates...))
	return q
}

// GroupBy adds GROUP BY expressions.
func (q *Query) GroupBy(expressions ...any) *Query {
	q.groupBy = append(q.groupBy, toSQLExpressions(expressions...)...)
	return q
}

// OrderBy appends ORDER BY expressions.
func (q *Query) OrderBy(expressions ...any) *Query {
	q.orderBy = append(q.orderBy, toSQLExpressions(expressions...)...)
	return q
}

// Limit sets a LIMIT clause.
func (q *Query) Limit(limit int) *Query {
	q.limit = &limit
	return q
}

// Offset sets an OFFSET clause.
func (q *Query) Offset(offset int) *Query {
	q.offset = &offset
	return q
}

// Returning adds RETURNING expressions for INSERT/UPDATE/DELETE queries.
func (q *Query) Returning(expressions ...any) *Query {
	q.returning = append(q.returning, toSQLExpressions(expressions...)...)
	return q
}

// With adds a common table expression (CTE).
func (q *Query) With(name string, subquery *Query, columns ...string) *Query {
	q.ctes = append(q.ctes, cte{name: name, query: subquery, columns: columns})
	return q
}

// WithRecursive adds a recursive CTE.
func (q *Query) WithRecursive(name string, subquery *Query, columns ...string) *Query {
	q.ctes = append(q.ctes, cte{name: name, query: subquery, columns: columns, recursive: true})
	return q
}

// Build renders the SQL string and the ordered arguments slice.
func (q *Query) Build() (string, []any) {
	ctx := &buildContext{}

	if q.qType == queryTypeRaw {
		ctx.args = append(ctx.args, q.rawArgs...)
		return q.rawSQL, ctx.args
	}

	sql := strings.Builder{}
	q.writeCTEs(&sql, ctx)

	switch q.qType {
	case queryTypeSelect:
		q.buildSelect(&sql, ctx)
	case queryTypeInsert:
		q.buildInsert(&sql, ctx)
	case queryTypeUpdate:
		q.buildUpdate(&sql, ctx)
	case queryTypeDelete:
		q.buildDelete(&sql, ctx)
	default:
		panic("query type not set")
	}

	return strings.TrimSpace(sql.String()), ctx.args
}

func (q *Query) writeCTEs(sql *strings.Builder, ctx *buildContext) {
	if len(q.ctes) == 0 {
		return
	}

	sql.WriteString("WITH ")

	parts := make([]string, 0, len(q.ctes))
	for _, c := range q.ctes {
		parts = append(parts, c.build(ctx))
	}

	sql.WriteString(strings.Join(parts, ", "))
	sql.WriteString(" ")
}

func (q *Query) buildSelect(sql *strings.Builder, ctx *buildContext) {
	sql.WriteString("SELECT ")
	if q.distinct {
		sql.WriteString("DISTINCT ")
	}

	columns := "*"
	if len(q.selectColumns) > 0 {
		colParts := make([]string, 0, len(q.selectColumns))
		for _, c := range q.selectColumns {
			colParts = append(colParts, c.build(ctx))
		}
		columns = strings.Join(colParts, ", ")
	}
	sql.WriteString(columns)

	if q.from != nil {
		sql.WriteString(" FROM ")
		sql.WriteString(q.from.build(ctx))
	}

	for _, j := range q.joins {
		sql.WriteString(" ")
		sql.WriteString(j.build(ctx))
	}

	q.buildPredicates(sql, ctx, "WHERE", q.where)

	if len(q.groupBy) > 0 {
		parts := make([]string, 0, len(q.groupBy))
		for _, g := range q.groupBy {
			parts = append(parts, g.build(ctx))
		}
		sql.WriteString(" GROUP BY ")
		sql.WriteString(strings.Join(parts, ", "))
	}

	q.buildPredicates(sql, ctx, "HAVING", q.having)

	if len(q.orderBy) > 0 {
		parts := make([]string, 0, len(q.orderBy))
		for _, o := range q.orderBy {
			parts = append(parts, o.build(ctx))
		}
		sql.WriteString(" ORDER BY ")
		sql.WriteString(strings.Join(parts, ", "))
	}

	if q.limit != nil {
		sql.WriteString(fmt.Sprintf(" LIMIT %d", *q.limit))
	}

	if q.offset != nil {
		sql.WriteString(fmt.Sprintf(" OFFSET %d", *q.offset))
	}

	q.writeReturning(sql, ctx)
}

func (q *Query) buildInsert(sql *strings.Builder, ctx *buildContext) {
	sql.WriteString("INSERT INTO ")
	sql.WriteString(q.insertTable.build(ctx))

	if len(q.insertCols) > 0 {
		sql.WriteString(" (")
		sql.WriteString(strings.Join(q.insertCols, ", "))
		sql.WriteString(")")
	}

	valueRows := make([]string, 0, len(q.insertValues))
	for _, row := range q.insertValues {
		parts := make([]string, 0, len(row))
		for _, v := range row {
			parts = append(parts, v.build(ctx))
		}
		valueRows = append(valueRows, fmt.Sprintf("(%s)", strings.Join(parts, ", ")))
	}

	if len(valueRows) > 0 {
		sql.WriteString(" VALUES ")
		sql.WriteString(strings.Join(valueRows, ", "))
	}

	q.buildPredicates(sql, ctx, "WHERE", q.where)
	q.writeReturning(sql, ctx)
}

func (q *Query) buildUpdate(sql *strings.Builder, ctx *buildContext) {
	sql.WriteString("UPDATE ")
	sql.WriteString(q.updateTable.build(ctx))

	if len(q.setClauses) == 0 {
		return
	}

	setParts := make([]string, 0, len(q.setClauses))
	for _, s := range q.setClauses {
		setParts = append(setParts, s.build(ctx))
	}

	sql.WriteString(" SET ")
	sql.WriteString(strings.Join(setParts, ", "))

	if q.from != nil {
		sql.WriteString(" FROM ")
		sql.WriteString(q.from.build(ctx))
	}

	for _, j := range q.joins {
		sql.WriteString(" ")
		sql.WriteString(j.build(ctx))
	}

	q.buildPredicates(sql, ctx, "WHERE", q.where)
	q.writeReturning(sql, ctx)
}

func (q *Query) buildDelete(sql *strings.Builder, ctx *buildContext) {
	sql.WriteString("DELETE FROM ")
	sql.WriteString(q.deleteTable.build(ctx))

	q.buildPredicates(sql, ctx, "WHERE", q.where)
	q.writeReturning(sql, ctx)
}

func (q *Query) buildPredicates(sql *strings.Builder, ctx *buildContext, keyword string, pred Predicate) {
	if pred == nil {
		return
	}
	clause := pred.build(ctx)
	if clause == "" {
		return
	}
	sql.WriteString(" ")
	sql.WriteString(keyword)
	sql.WriteString(" ")
	sql.WriteString(clause)
}

func (q *Query) writeReturning(sql *strings.Builder, ctx *buildContext) {
	if len(q.returning) == 0 {
		return
	}

	parts := make([]string, 0, len(q.returning))
	for _, r := range q.returning {
		parts = append(parts, r.build(ctx))
	}

	sql.WriteString(" RETURNING ")
	sql.WriteString(strings.Join(parts, ", "))
}

// SetClause represents a column-value assignment used in UPDATE statements.
type SetClause struct {
	column string
	value  Expression
}

// Set defines a column assignment for UPDATE queries.
func Set(column string, value any) SetClause {
	return SetClause{column: column, value: toValueExpression(value)}
}

func (s SetClause) build(ctx *buildContext) string {
	return fmt.Sprintf("%s = %s", s.column, s.value.build(ctx))
}

// buildContext is used internally to collect placeholders and arguments.
type buildContext struct {
	args []any
}

// nextPlaceholder appends the provided argument and returns the placeholder symbol.
func (ctx *buildContext) nextPlaceholder(arg any) string {
	ctx.args = append(ctx.args, arg)
	return "?"
}

// cte represents a common table expression definition.
type cte struct {
	name      string
	query     *Query
	columns   []string
	recursive bool
}

func (c cte) build(ctx *buildContext) string {
	sb := strings.Builder{}

	if c.recursive {
		sb.WriteString("RECURSIVE ")
	}

	sb.WriteString(c.name)
	if len(c.columns) > 0 {
		sb.WriteString(" (")
		sb.WriteString(strings.Join(c.columns, ", "))
		sb.WriteString(")")
	}

	sb.WriteString(" AS (")
	sql, args := c.query.Build()
	sb.WriteString(sql)
	sb.WriteString(")")

	ctx.args = append(ctx.args, args...)
	return sb.String()
}

// joinClause represents a SQL JOIN clause.
type joinClause struct {
	kind  string
	table TableExpression
	on    Predicate
}

func (j joinClause) build(ctx *buildContext) string {
	sb := strings.Builder{}
	sb.WriteString(j.kind)
	sb.WriteString(" ")
	sb.WriteString(j.table.build(ctx))
	if j.on != nil {
		sb.WriteString(" ON ")
		sb.WriteString(j.on.build(ctx))
	}
	return sb.String()
}

// TableExpression represents a FROM or JOIN target.
type TableExpression interface {
	build(*buildContext) string
}

// TableRef references a table, optionally with alias or derived subquery.
type TableRef struct {
	name  string
	alias string
	sub   *Query
}

// TableAlias returns a table reference with an alias.
func TableAlias(name, alias string) TableRef {
	return TableRef{name: name, alias: alias}
}

// FromSubquery wraps a subquery as a table expression.
func FromSubquery(q *Query, alias string) TableRef {
	return TableRef{sub: q, alias: alias}
}

func (t TableRef) build(ctx *buildContext) string {
	sb := strings.Builder{}

	if t.sub != nil {
		sql, args := t.sub.Build()
		sb.WriteString("(")
		sb.WriteString(sql)
		sb.WriteString(")")
		ctx.args = append(ctx.args, args...)
	} else {
		sb.WriteString(t.name)
	}

	if t.alias != "" {
		sb.WriteString(" AS ")
		sb.WriteString(t.alias)
	}

	return sb.String()
}

func toTableExpression(value any) TableExpression {
	switch v := value.(type) {
	case TableExpression:
		return v
	case *Query:
		return FromSubquery(v, "")
	case string:
		return TableRef{name: v}
	default:
		panic(fmt.Sprintf("unsupported table expression: %T", value))
	}
}

// toExpression converts common Go types into an Expression.
func toValueExpression(value any) Expression {
	switch v := value.(type) {
	case Expression:
		return v
	case *Query:
		return subqueryExpr{query: v}
	case Query:
		return subqueryExpr{query: &v}
	default:
		return Value(v)
	}
}

func toValueExpressions(values ...any) []Expression {
	out := make([]Expression, 0, len(values))
	for _, v := range values {
		out = append(out, toValueExpression(v))
	}
	return out
}

func toSQLExpression(value any) Expression {
	switch v := value.(type) {
	case string:
		return rawExpr{sql: v}
	default:
		return toValueExpression(v)
	}
}

func toSQLExpressions(values ...any) []Expression {
	out := make([]Expression, 0, len(values))
	for _, v := range values {
		out = append(out, toSQLExpression(v))
	}
	return out
}

// SortArgs sorts arguments to make tests deterministic. Useful for text search helpers.
func sortArgs(args []string) []string {
	clone := append([]string(nil), args...)
	sort.Strings(clone)
	return clone
}
