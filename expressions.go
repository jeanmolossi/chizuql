package chizuql

import (
	"fmt"
	"strings"
	"sync"
)

func requireDialect(ctx *buildContext, expected dialectKind, feature string) {
	kind, ok := dialectKindOf(ctx.dialect)
	if !ok {
		panic(fmt.Sprintf("%s requer um dialeto reconhecido", feature))
	}

	if kind != expected {
		panic(fmt.Sprintf("%s é suportado apenas no dialeto %s", feature, expected))
	}
}

func escapeSingleQuotes(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

var (
	defaultTextSearchConfig   = "english"
	defaultTextSearchConfigMu sync.RWMutex
)

// SetDefaultTextSearchConfig replaces the package-wide default text search configuration used by TsVector builders.
func SetDefaultTextSearchConfig(config string) {
	defaultTextSearchConfigMu.Lock()
	defer defaultTextSearchConfigMu.Unlock()

	defaultTextSearchConfig = config
}

// DefaultTextSearchConfig returns the package-wide default text search configuration for TsVector builders.
func DefaultTextSearchConfig() string {
	defaultTextSearchConfigMu.RLock()
	defer defaultTextSearchConfigMu.RUnlock()

	return defaultTextSearchConfig
}

// Expression represents any fragment that can be embedded in SQL.
type Expression interface {
	build(*buildContext) string
}

// Predicate is a boolean expression used in WHERE/HAVING/ON clauses.
type Predicate interface {
	Expression
}

// Column references a column name and can be used to generate predicates.
type Column struct {
	name  string
	alias string
}

// Col creates a column expression.
func Col(name string) Column {
	return Column{name: name}
}

// ColAlias creates a column expression with alias for SELECT lists.
func ColAlias(name, alias string) Column {
	return Column{name: name, alias: alias}
}

func (c Column) build(ctx *buildContext) string {
	if c.alias != "" {
		return fmt.Sprintf("%s AS %s", c.name, c.alias)
	}

	return c.name
}

func (c Column) Eq(value any) Predicate {
	return comparison{left: c, op: "=", right: toValueExpression(value)}
}
func (c Column) Ne(value any) Predicate {
	return comparison{left: c, op: "<>", right: toValueExpression(value)}
}
func (c Column) Gt(value any) Predicate {
	return comparison{left: c, op: ">", right: toValueExpression(value)}
}
func (c Column) Gte(value any) Predicate {
	return comparison{left: c, op: ">=", right: toValueExpression(value)}
}
func (c Column) Lt(value any) Predicate {
	return comparison{left: c, op: "<", right: toValueExpression(value)}
}
func (c Column) Lte(value any) Predicate {
	return comparison{left: c, op: "<=", right: toValueExpression(value)}
}

// In builds an IN predicate. Values can be literals or a subquery.
func (c Column) In(values ...any) Predicate {
	if len(values) == 0 {
		panic("IN list cannot be empty")
	}

	if len(values) == 1 {
		if sub, ok := values[0].(*Query); ok {
			return comparison{left: c, op: "IN", right: subqueryExpr{query: sub}}
		}
	}

	exprs := toValueExpressions(values...)

	return inPredicate{left: c, list: exprs}
}

// Between builds a BETWEEN predicate.
func (c Column) Between(start, end any) Predicate {
	return betweenPredicate{left: c, start: toValueExpression(start), end: toValueExpression(end)}
}

// Like builds a LIKE predicate.
func (c Column) Like(value any) Predicate {
	return comparison{left: c, op: "LIKE", right: toValueExpression(value)}
}

// IsNull builds an IS NULL predicate.
func (c Column) IsNull() Predicate { return unaryPredicate{left: c, keyword: "IS NULL"} }

// IsNotNull builds an IS NOT NULL predicate.
func (c Column) IsNotNull() Predicate { return unaryPredicate{left: c, keyword: "IS NOT NULL"} }

// Value wraps a literal value as an Expression with placeholders.
type valueExpr struct {
	value any
}

// Value creates an expression that turns into a placeholder and stores the argument.
func Value(v any) Expression { return valueExpr{value: v} }

func (v valueExpr) build(ctx *buildContext) string {
	return ctx.nextPlaceholder(v.value)
}

// Raw builds an expression that is inserted as-is. Arguments are appended verbatim.
type rawExpr struct {
	sql  string
	args []any
}

// Raw creates a raw SQL expression. Use carefully.
func Raw(sql string, args ...any) Expression {
	return rawExpr{sql: sql, args: args}
}

func (r rawExpr) build(ctx *buildContext) string {
	ctx.args = append(ctx.args, r.args...)

	return r.sql
}

// subqueryExpr is used to embed subqueries into larger expressions.
type subqueryExpr struct {
	query *Query
}

func (s subqueryExpr) build(ctx *buildContext) string {
	return fmt.Sprintf("(%s)", s.query.render(ctx))
}

// comparison represents a binary comparison predicate.
type comparison struct {
	left  Expression
	op    string
	right Expression
}

func (c comparison) build(ctx *buildContext) string {
	return fmt.Sprintf("%s %s %s", c.left.build(ctx), c.op, c.right.build(ctx))
}

// inPredicate represents an IN predicate.
type inPredicate struct {
	left Expression
	list []Expression
}

func (i inPredicate) build(ctx *buildContext) string {
	if len(i.list) == 0 {
		panic("IN list cannot be empty")
	}

	parts := make([]string, 0, len(i.list))
	for _, item := range i.list {
		parts = append(parts, item.build(ctx))
	}

	return fmt.Sprintf("%s IN (%s)", i.left.build(ctx), strings.Join(parts, ", "))
}

// betweenPredicate represents a BETWEEN predicate.
type betweenPredicate struct {
	left  Expression
	start Expression
	end   Expression
}

func (b betweenPredicate) build(ctx *buildContext) string {
	return fmt.Sprintf("%s BETWEEN %s AND %s", b.left.build(ctx), b.start.build(ctx), b.end.build(ctx))
}

// unaryPredicate represents predicates without right operand.
type unaryPredicate struct {
	left    Expression
	keyword string
}

func (u unaryPredicate) build(ctx *buildContext) string {
	return fmt.Sprintf("%s %s", u.left.build(ctx), u.keyword)
}

// compoundPredicate combines predicates with a boolean operator.
type compoundPredicate struct {
	op    string
	parts []Predicate
}

func (c compoundPredicate) build(ctx *buildContext) string {
	if len(c.parts) == 0 {
		return ""
	}

	fragments := make([]string, 0, len(c.parts))

	for _, p := range c.parts {
		fragment := p.build(ctx)
		if fragment != "" {
			fragments = append(fragments, fragment)
		}
	}

	if len(fragments) == 0 {
		return ""
	}

	return fmt.Sprintf("(%s)", strings.Join(fragments, fmt.Sprintf(" %s ", c.op)))
}

// And joins predicates with AND.
func And(predicates ...Predicate) Predicate { return compoundPredicate{op: "AND", parts: predicates} }

// Or joins predicates with OR.
func Or(predicates ...Predicate) Predicate { return compoundPredicate{op: "OR", parts: predicates} }

// Not negates a predicate while preserving placeholder arguments.
func Not(predicate Predicate) Predicate { return notPredicate{pred: predicate} }

type notPredicate struct {
	pred Predicate
}

func (n notPredicate) build(ctx *buildContext) string {
	return fmt.Sprintf("NOT (%s)", n.pred.build(ctx))
}

// MatchBuilder creates MySQL MATCH ... AGAINST predicates.
type MatchBuilder struct {
	columns []string
}

// Match selects columns to use with MATCH AGAINST.
func Match(columns ...string) MatchBuilder { return MatchBuilder{columns: columns} }

// Against builds a MATCH ... AGAINST expression with optional mode (e.g., "BOOLEAN MODE").
func (m MatchBuilder) Against(query string, mode ...string) Predicate {
	selectedMode := ""
	if len(mode) > 0 {
		selectedMode = mode[0]
	}

	return matchAgainstExpr{columns: m.columns, mode: selectedMode, query: query}
}

// Score builds a MATCH ... AGAINST expression to be used in SELECT/ORDER BY for relevance ranking.
func (m MatchBuilder) Score(query string, mode ...string) matchScoreExpr {
	selectedMode := ""
	if len(mode) > 0 {
		selectedMode = mode[0]
	}

	return matchScoreExpr{clause: matchAgainstExpr{columns: m.columns, mode: selectedMode, query: query}}
}

type matchAgainstExpr struct {
	columns []string
	mode    string
	query   string
}

type matchScoreExpr struct {
	clause matchAgainstExpr
}

func (m matchAgainstExpr) build(ctx *buildContext) string {
	requireDialect(ctx, dialectMySQL, "MATCH ... AGAINST")

	pl := ctx.nextPlaceholder(m.query)
	part := fmt.Sprintf("MATCH(%s) AGAINST (%s)", strings.Join(m.columns, ", "), pl)

	if m.mode != "" {
		part = fmt.Sprintf("MATCH(%s) AGAINST (%s IN %s)", strings.Join(m.columns, ", "), pl, m.mode)
	}

	return part
}

func (m matchScoreExpr) build(ctx *buildContext) string { return m.clause.build(ctx) }

// Asc builds an ascending ORDER BY fragment for MATCH scores.
func (m matchScoreExpr) Asc() Expression { return orderedExpr{expr: m, order: "ASC"} }

// Desc builds a descending ORDER BY fragment for MATCH scores.
func (m matchScoreExpr) Desc() Expression { return orderedExpr{expr: m, order: "DESC"} }

// TsVectorBuilder creates PostgreSQL full-text search predicates.
type TsVectorBuilder struct {
	config  string
	columns []string
}

// TsVector builds a to_tsvector expression using CONCAT_WS semantics.
func TsVector(columns ...string) TsVectorBuilder {
	return TsVectorBuilder{columns: columns, config: DefaultTextSearchConfig()}
}

// WithConfig overrides the text search configuration.
func (t TsVectorBuilder) WithConfig(config string) TsVectorBuilder {
	t.config = config

	return t
}

// WithLanguage is an alias for WithConfig to highlight language switching on PostgreSQL FTS.
func (t TsVectorBuilder) WithLanguage(language string) TsVectorBuilder {
	return t.WithConfig(language)
}

// WebSearch builds a websearch_to_tsquery predicate.
func (t TsVectorBuilder) WebSearch(query string) Predicate {
	return tsQueryPredicate{builder: t, query: query, mode: "web"}
}

// PlainQuery builds a plainto_tsquery predicate.
func (t TsVectorBuilder) PlainQuery(query string) Predicate {
	return tsQueryPredicate{builder: t, query: query, mode: "plain"}
}

// RankWebSearch builds a ts_rank expression using websearch_to_tsquery for relevance scoring.
func (t TsVectorBuilder) RankWebSearch(query string, normalization ...int) tsRankExpr {
	return tsRankExpr{builder: t, query: query, mode: "web", normalization: pickNormalization(normalization)}
}

// RankPlainQuery builds a ts_rank expression using plainto_tsquery for relevance scoring.
func (t TsVectorBuilder) RankPlainQuery(query string, normalization ...int) tsRankExpr {
	return tsRankExpr{builder: t, query: query, mode: "plain", normalization: pickNormalization(normalization)}
}

type tsQueryPredicate struct {
	builder TsVectorBuilder
	query   string
	mode    string
}

func (t tsQueryPredicate) build(ctx *buildContext) string {
	vector, query := t.builder.buildTsQuery(ctx, t.query, t.mode)

	return fmt.Sprintf("%s @@ %s", vector, query)
}

func (t TsVectorBuilder) concatColumns() string {
	switch len(t.columns) {
	case 0:
		return "''"
	case 1:
		return t.columns[0]
	default:
		return fmt.Sprintf("CONCAT_WS(' ', %s)", strings.Join(t.columns, ", "))
	}
}

func (t TsVectorBuilder) buildTsQuery(ctx *buildContext, query string, mode string) (string, string) {
	requireDialect(ctx, dialectPostgres, "Full Text Search (tsvector)")

	placeholder := ctx.nextPlaceholder(query)
	config := escapeSingleQuotes(t.config)
	vector := fmt.Sprintf("to_tsvector('%s', %s)", config, t.concatColumns())

	switch mode {
	case "web":
		return vector, fmt.Sprintf("websearch_to_tsquery('%s', %s)", config, placeholder)
	default:
		return vector, fmt.Sprintf("plainto_tsquery('%s', %s)", config, placeholder)
	}
}

type tsRankExpr struct {
	builder       TsVectorBuilder
	query         string
	mode          string
	normalization *int
}

func (t tsRankExpr) build(ctx *buildContext) string {
	vector, query := t.builder.buildTsQuery(ctx, t.query, t.mode)

	if t.normalization != nil {
		return fmt.Sprintf("ts_rank(%s, %s, %d)", vector, query, *t.normalization)
	}

	return fmt.Sprintf("ts_rank(%s, %s)", vector, query)
}

// Asc builds an ascending ORDER BY fragment for ts_rank scores.
func (t tsRankExpr) Asc() Expression { return orderedExpr{expr: t, order: "ASC"} }

// Desc builds a descending ORDER BY fragment for ts_rank scores.
func (t tsRankExpr) Desc() Expression { return orderedExpr{expr: t, order: "DESC"} }

func pickNormalization(values []int) *int {
	if len(values) == 0 {
		return nil
	}

	return &values[0]
}

// JSONExtract builds a dialect-aware JSON/JSONB extractor using parameterized paths.
func JSONExtract(column string, path any) Expression {
	return jsonExtractExpr{column: column, path: toValueExpression(path)}
}

// JSONExtractText unwraps JSON/JSONB values into text while keeping paths parameterized.
func JSONExtractText(column string, path any) Expression {
	return jsonExtractExpr{column: column, path: toValueExpression(path), unwrap: true}
}

type jsonExtractExpr struct {
	column string
	path   Expression
	unwrap bool
}

func (j jsonExtractExpr) build(ctx *buildContext) string {
	kind, ok := dialectKindOf(ctx.dialect)
	if !ok {
		panic("Extração de JSON requer um dialeto reconhecido")
	}

	path := j.path.build(ctx)

	var expr string

	switch kind {
	case dialectMySQL:
		expr = fmt.Sprintf("JSON_EXTRACT(%s, %s)", j.column, path)
	case dialectPostgres:
		expr = fmt.Sprintf("jsonb_path_query_first(to_jsonb(%s), (%s)::jsonpath)", j.column, path)
	default:
		panic("Extração de JSON não suportada para este dialeto")
	}

	if j.unwrap {
		switch kind {
		case dialectMySQL:
			return fmt.Sprintf("JSON_UNQUOTE(%s)", expr)
		case dialectPostgres:
			return fmt.Sprintf("(%s)::text", expr)
		}
	}

	return expr
}

// JSONContains builds a containment predicate for JSON/JSONB values.
func JSONContains(column string, value any) Predicate {
	return jsonContainsPredicate{column: column, value: toValueExpression(value)}
}

type jsonContainsPredicate struct {
	column string
	value  Expression
}

func (j jsonContainsPredicate) build(ctx *buildContext) string {
	kind, ok := dialectKindOf(ctx.dialect)
	if !ok {
		panic("JSON_CONTAINS requer um dialeto reconhecido")
	}

	value := j.value.build(ctx)

	switch kind {
	case dialectMySQL:
		return fmt.Sprintf("JSON_CONTAINS(%s, %s)", j.column, value)
	case dialectPostgres:
		return fmt.Sprintf("to_jsonb(%s) @> (%s)::jsonb", j.column, value)
	default:
		panic("JSON_CONTAINS não suportado para este dialeto")
	}
}

type orderedExpr struct {
	expr  Expression
	order string
}

func (o orderedExpr) build(ctx *buildContext) string {
	return fmt.Sprintf("%s %s", o.expr.build(ctx), o.order)
}
