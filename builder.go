package chizuql

import (
	"fmt"
	"strings"
	"sync"
)

type queryType string

const (
	queryTypeSelect queryType = "SELECT"
	queryTypeInsert queryType = "INSERT"
	queryTypeUpdate queryType = "UPDATE"
	queryTypeDelete queryType = "DELETE"
	queryTypeRaw    queryType = "RAW"
)

// Dialect controls placeholder rendering and certain dialect-specific clauses.
type Dialect interface {
	placeholder(int) string
}

type dialectKind string

const (
	dialectMySQL    dialectKind = "mysql"
	dialectPostgres dialectKind = "postgres"
)

type sqlDialect struct {
	kind dialectKind
}

func (d sqlDialect) Kind() dialectKind {
	return d.kind
}

func (d sqlDialect) placeholder(i int) string {
	switch d.kind {
	case dialectPostgres:
		return fmt.Sprintf("$%d", i)
	default:
		return "?"
	}
}

type dialectInspector interface {
	Kind() dialectKind
}

func dialectKindOf(d Dialect) (dialectKind, bool) {
	if inspected, ok := d.(dialectInspector); ok {
		return inspected.Kind(), true
	}

	return "", false
}

var (
	// DialectMySQL renders placeholders as ?
	DialectMySQL Dialect = sqlDialect{kind: dialectMySQL}
	// DialectPostgres renders placeholders as $1, $2, ...
	DialectPostgres Dialect = sqlDialect{kind: dialectPostgres}

	defaultDialect   Dialect = DialectMySQL
	defaultDialectMu sync.RWMutex
)

// SetDefaultDialect replaces the package-wide default dialect used by newly created queries.
func SetDefaultDialect(d Dialect) {
	defaultDialectMu.Lock()
	defer defaultDialectMu.Unlock()

	defaultDialect = d
}

// DefaultDialect returns the package-wide default dialect.
func DefaultDialect() Dialect {
	defaultDialectMu.RLock()
	defer defaultDialectMu.RUnlock()

	return defaultDialect
}

// Query represents a composable SQL query built using the fluent API.
type Query struct {
	qType queryType

	dialect Dialect

	rawSQL  string
	rawArgs []any

	ctes []cte

	unions []unionClause

	selectColumns []Expression
	distinct      bool

	from  TableExpression
	joins []joinClause

	where     Predicate
	groupBy   []Expression
	having    Predicate
	orderBy   []Expression
	limit     *int
	offset    *int
	setLimit  *int
	setOffset *int

	insertTable         TableExpression
	insertCols          []string
	insertValues        [][]Expression
	onConflictTarget    []string
	onConflictSet       []SetClause
	onConflictDoNothing bool

	updateTable TableExpression
	setClauses  []SetClause

	deleteTable TableExpression

	returning []Expression
}

// New returns a fresh Query instance ready to be composed.
func New() *Query {
	return &Query{dialect: DefaultDialect()}
}

// RawQuery builds a query directly from the provided SQL fragment and arguments.
func RawQuery(sql string, args ...any) *Query {
	return &Query{qType: queryTypeRaw, rawSQL: sql, rawArgs: args}
}

// WithDialect sets the SQL dialect for placeholder and conflict rendering.
func (q *Query) WithDialect(d Dialect) *Query {
	q.dialect = d

	return q
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

// OnConflictDoNothing adds a conflict handler that skips inserts when conflicts arise.
func (q *Query) OnConflictDoNothing(targetColumns ...string) *Query {
	q.onConflictTarget = targetColumns
	q.onConflictDoNothing = true

	return q
}

// OnConflictDoUpdate adds a conflict handler that performs an update when conflicts arise.
func (q *Query) OnConflictDoUpdate(targetColumns []string, setClauses ...SetClause) *Query {
	q.onConflictTarget = targetColumns
	q.onConflictSet = setClauses
	q.onConflictDoNothing = false

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
	if len(q.unions) > 0 {
		q.setLimit = &limit
	} else {
		q.limit = &limit
	}

	return q
}

// Offset sets an OFFSET clause.
func (q *Query) Offset(offset int) *Query {
	if len(q.unions) > 0 {
		q.setOffset = &offset
	} else {
		q.offset = &offset
	}

	return q
}

// Returning adds RETURNING expressions for INSERT/UPDATE/DELETE queries.
func (q *Query) Returning(expressions ...any) *Query {
	switch q.qType {
	case queryTypeSelect:
		panic("RETURNING is not supported on SELECT queries")
	case queryTypeInsert, queryTypeUpdate, queryTypeDelete:
	case queryTypeRaw, "":
		panic("RETURNING requer uma query INSERT, UPDATE ou DELETE")
	}

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

// Union appends UNION operations with other SELECT queries.
func (q *Query) Union(queries ...*Query) *Query { return q.union(false, queries...) }

// UnionAll appends UNION ALL operations with other SELECT queries.
func (q *Query) UnionAll(queries ...*Query) *Query { return q.union(true, queries...) }

func (q *Query) union(all bool, queries ...*Query) *Query {
	if q.qType != queryTypeSelect {
		if q.qType == "" {
			panic("UNION requer uma consulta SELECT inicial")
		}

		panic("UNION pode ser usado apenas em consultas SELECT")
	}

	for _, other := range queries {
		if other == nil {
			panic("UNION requer queries não nulas")
		}

		if other.qType != queryTypeSelect {
			panic("UNION aceita apenas queries SELECT como operando")
		}

		q.unions = append(q.unions, unionClause{query: other, all: all})
	}

	return q
}

// Build renders the SQL string and the ordered arguments slice.
func (q *Query) Build() (string, []any) {
	dialect := q.dialect
	if dialect == nil {
		dialect = DefaultDialect()
	}

	ctx := &buildContext{dialect: dialect}
	sql := strings.TrimSpace(q.render(ctx))

	return sql, ctx.args
}

func (q *Query) render(ctx *buildContext) string {
	if q.qType == queryTypeRaw {
		ctx.args = append(ctx.args, q.rawArgs...)

		return q.rawSQL
	}

	sql := strings.Builder{}
	q.writeCTEs(&sql, ctx)

	switch q.qType {
	case queryTypeSelect:
		if len(q.unions) > 0 {
			q.buildSetSelect(&sql, ctx)
		} else {
			q.buildSelect(&sql, ctx, true)
		}
	case queryTypeInsert:
		q.buildInsert(&sql, ctx)
	case queryTypeUpdate:
		q.buildUpdate(&sql, ctx)
	case queryTypeDelete:
		q.buildDelete(&sql, ctx)
	default:
		panic("query type not set")
	}

	return sql.String()
}

func (q *Query) writeCTEs(sql *strings.Builder, ctx *buildContext) {
	if len(q.ctes) == 0 {
		return
	}

	sql.WriteString("WITH ")

	hasRecursive := false

	for _, c := range q.ctes {
		if c.recursive {
			hasRecursive = true

			break
		}
	}

	if hasRecursive {
		sql.WriteString("RECURSIVE ")
	}

	parts := make([]string, 0, len(q.ctes))
	for _, c := range q.ctes {
		parts = append(parts, c.build(ctx))
	}

	sql.WriteString(strings.Join(parts, ", "))
	sql.WriteString(" ")
}

func (q *Query) buildSelect(sql *strings.Builder, ctx *buildContext, includeOrdering bool) {
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

	if includeOrdering {
		q.appendOrdering(sql, ctx)
	}

	q.appendPagination(sql, q.limit, q.offset)
}

func (q *Query) buildSetSelect(sql *strings.Builder, ctx *buildContext) {
	q.buildSelect(sql, ctx, false)

	for _, u := range q.unions {
		sql.WriteString(" ")

		if u.all {
			sql.WriteString("UNION ALL ")
		} else {
			sql.WriteString("UNION ")
		}

		sql.WriteString(u.query.renderSetOperand(ctx))
	}

	q.appendOrdering(sql, ctx)

	q.appendPagination(sql, q.setLimit, q.setOffset)
}

func (q *Query) appendOrdering(sql *strings.Builder, ctx *buildContext) {
	if len(q.orderBy) > 0 {
		parts := make([]string, 0, len(q.orderBy))
		for _, o := range q.orderBy {
			parts = append(parts, o.build(ctx))
		}

		sql.WriteString(" ORDER BY ")
		sql.WriteString(strings.Join(parts, ", "))
	}
}

func (q *Query) renderSetOperand(ctx *buildContext) string {
	if q == nil {
		panic("UNION requer queries não nulas")
	}

	if q.qType != queryTypeSelect {
		panic("UNION aceita apenas queries SELECT como operando")
	}

	sb := strings.Builder{}
	sb.WriteString("(")

	if len(q.ctes) > 0 {
		q.writeCTEs(&sb, ctx)
	}

	q.buildSelect(&sb, ctx, false)
	sb.WriteString(")")

	return sb.String()
}

func (q *Query) appendPagination(sql *strings.Builder, limit *int, offset *int) {
	if limit != nil {
		fmt.Fprintf(sql, " LIMIT %d", *limit)
	}

	if offset != nil {
		fmt.Fprintf(sql, " OFFSET %d", *offset)
	}
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

	q.writeOnConflict(sql, ctx)
	q.writeReturning(sql, ctx)
}

func (q *Query) buildUpdate(sql *strings.Builder, ctx *buildContext) {
	sql.WriteString("UPDATE ")
	sql.WriteString(q.updateTable.build(ctx))

	if len(q.setClauses) == 0 {
		panic("UPDATE requires at least one SET clause")
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

func (q *Query) writeOnConflict(sql *strings.Builder, ctx *buildContext) {
	if len(q.onConflictSet) == 0 && !q.onConflictDoNothing {
		return
	}

	if d, ok := ctx.dialect.(sqlDialect); ok && d.kind == dialectMySQL {
		if len(q.onConflictSet) == 0 {
			return
		}

		setParts := make([]string, 0, len(q.onConflictSet))
		for _, s := range q.onConflictSet {
			setParts = append(setParts, s.build(ctx))
		}

		sql.WriteString(" ON DUPLICATE KEY UPDATE ")
		sql.WriteString(strings.Join(setParts, ", "))

		return
	}

	sql.WriteString(" ON CONFLICT")

	if len(q.onConflictTarget) > 0 {
		sql.WriteString(" (")
		sql.WriteString(strings.Join(q.onConflictTarget, ", "))
		sql.WriteString(")")
	}

	if q.onConflictDoNothing {
		sql.WriteString(" DO NOTHING")

		return
	}

	setParts := make([]string, 0, len(q.onConflictSet))
	for _, s := range q.onConflictSet {
		setParts = append(setParts, s.build(ctx))
	}

	sql.WriteString(" DO UPDATE SET ")
	sql.WriteString(strings.Join(setParts, ", "))
}

// SetClause represents a column-value assignment used in UPDATE statements.
type SetClause struct {
	column string
	value  Expression
}

type unionClause struct {
	query *Query
	all   bool
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
	args             []any
	dialect          Dialect
	placeholderIndex int
	subqueryAlias    int
	subqueryAliases  map[*Query]string
}

// nextPlaceholder appends the provided argument and returns the placeholder symbol.
func (ctx *buildContext) nextPlaceholder(arg any) string {
	ctx.placeholderIndex++
	pl := ctx.dialect.placeholder(ctx.placeholderIndex)
	ctx.args = append(ctx.args, arg)

	return pl
}

func (ctx *buildContext) nextSubqueryAlias(q *Query) string {
	if ctx.subqueryAliases == nil {
		ctx.subqueryAliases = make(map[*Query]string)
	}

	if alias, ok := ctx.subqueryAliases[q]; ok {
		return alias
	}

	ctx.subqueryAlias++
	alias := fmt.Sprintf("subq_%d", ctx.subqueryAlias)
	ctx.subqueryAliases[q] = alias

	return alias
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

	sb.WriteString(c.name)

	if len(c.columns) > 0 {
		sb.WriteString(" (")
		sb.WriteString(strings.Join(c.columns, ", "))
		sb.WriteString(")")
	}

	sb.WriteString(" AS (")
	sb.WriteString(c.query.render(ctx))
	sb.WriteString(")")

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
	alias := t.alias

	if t.sub != nil {
		sb.WriteString("(")
		sb.WriteString(t.sub.render(ctx))
		sb.WriteString(")")

		if alias == "" {
			alias = ctx.nextSubqueryAlias(t.sub)
		}
	} else {
		sb.WriteString(t.name)
	}

	if alias != "" {
		sb.WriteString(" AS ")
		sb.WriteString(alias)
	}

	return sb.String()
}

func toTableExpression(value any) TableExpression {
	switch v := value.(type) {
	case TableExpression:
		return v
	case *Query:
		return FromSubquery(v, "")
	case Query:
		return FromSubquery(&v, "")
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
