package chizuql

import (
	"fmt"
	"strings"
)

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
	return comparison{left: c, op: "=", right: toExpression(value)}
}
func (c Column) Ne(value any) Predicate {
	return comparison{left: c, op: "<>", right: toExpression(value)}
}
func (c Column) Gt(value any) Predicate {
	return comparison{left: c, op: ">", right: toExpression(value)}
}
func (c Column) Gte(value any) Predicate {
	return comparison{left: c, op: ">=", right: toExpression(value)}
}
func (c Column) Lt(value any) Predicate {
	return comparison{left: c, op: "<", right: toExpression(value)}
}
func (c Column) Lte(value any) Predicate {
	return comparison{left: c, op: "<=", right: toExpression(value)}
}

// In builds an IN predicate. Values can be literals or a subquery.
func (c Column) In(values ...any) Predicate {
	if len(values) == 1 {
		if sub, ok := values[0].(*Query); ok {
			return comparison{left: c, op: "IN", right: subqueryExpr{query: sub}}
		}
	}

	exprs := toExpressions(values...)
	return inPredicate{left: c, list: exprs}
}

// Between builds a BETWEEN predicate.
func (c Column) Between(start, end any) Predicate {
	return betweenPredicate{left: c, start: toExpression(start), end: toExpression(end)}
}

// Like builds a LIKE predicate.
func (c Column) Like(value any) Predicate {
	return comparison{left: c, op: "LIKE", right: toExpression(value)}
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
	sql, args := s.query.Build()
	ctx.args = append(ctx.args, args...)
	return fmt.Sprintf("(%s)", sql)
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
	part := fmt.Sprintf("MATCH(%s) AGAINST (?)", strings.Join(m.columns, ", "))
	if len(mode) > 0 && mode[0] != "" {
		part = fmt.Sprintf("MATCH(%s) AGAINST (? IN %s)", strings.Join(m.columns, ", "), mode[0])
	}
	return rawExpr{sql: part, args: []any{query}}
}

// TsVectorBuilder creates PostgreSQL full-text search predicates.
type TsVectorBuilder struct {
	config  string
	columns []string
}

// TsVector builds a to_tsvector expression using CONCAT_WS semantics.
func TsVector(columns ...string) TsVectorBuilder {
	return TsVectorBuilder{columns: columns, config: "english"}
}

// WithConfig overrides the text search configuration.
func (t TsVectorBuilder) WithConfig(config string) TsVectorBuilder { t.config = config; return t }

// WebSearch builds a websearch_to_tsquery predicate.
func (t TsVectorBuilder) WebSearch(query string) Predicate {
	return rawExpr{
		sql:  fmt.Sprintf("to_tsvector('%s', %s) @@ websearch_to_tsquery('%s', ?)", t.config, t.concatColumns(), t.config),
		args: []any{query},
	}
}

// PlainQuery builds a plainto_tsquery predicate.
func (t TsVectorBuilder) PlainQuery(query string) Predicate {
	return rawExpr{
		sql:  fmt.Sprintf("to_tsvector('%s', %s) @@ plainto_tsquery('%s', ?)", t.config, t.concatColumns(), t.config),
		args: []any{query},
	}
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
