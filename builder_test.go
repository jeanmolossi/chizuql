package chizuql

import (
	"reflect"
	"testing"
)

func assertBuild(t *testing.T, q *Query, wantSQL string, wantArgs []any) {
	t.Helper()

	gotSQL, gotArgs := q.Build()

	if gotSQL != wantSQL {
		t.Fatalf("unexpected SQL.\nwant: %s\n got: %s", wantSQL, gotSQL)
	}

	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("unexpected args.\nwant: %#v\n got: %#v", wantArgs, gotArgs)
	}
}

func TestSelectQuery(t *testing.T) {
	q := New().
		Select("id", ColAlias("name", "nome")).
		From(TableAlias("users", "u")).
		Where(
			Col("status").Eq("active"),
			Col("age").Gt(18),
		).
		OrderBy("created_at DESC").
		Limit(10).
		Offset(5)

	assertBuild(t, q,
		"SELECT id, name AS nome FROM users AS u WHERE (status = ? AND age > ?) ORDER BY created_at DESC LIMIT 10 OFFSET 5",
		[]any{"active", 18},
	)
}

func TestJoinGroupHaving(t *testing.T) {
	q := New().
		Select("u.id", ColAlias("COUNT(p.id)", "post_count")).
		From(TableAlias("users", "u")).
		LeftJoin(TableAlias("posts", "p"), Col("u.id").Eq(Col("p.user_id"))).
		GroupBy("u.id").
		Having(Col("COUNT(p.id)").Gt(0)).
		OrderBy("post_count DESC")

	assertBuild(t, q,
		"SELECT u.id, COUNT(p.id) AS post_count FROM users AS u LEFT JOIN posts AS p ON (u.id = p.user_id) GROUP BY u.id HAVING (COUNT(p.id) > ?) ORDER BY post_count DESC",
		[]any{0},
	)
}

func TestInsertReturning(t *testing.T) {
	q := New().
		InsertInto("users", "name", "email").
		Values("Jane", "jane@example.com").
		Values("John", "john@example.com").
		Returning("id")

	assertBuild(t, q,
		"INSERT INTO users (name, email) VALUES (?, ?), (?, ?) RETURNING id",
		[]any{"Jane", "jane@example.com", "John", "john@example.com"},
	)
}

func TestUpdateWithSubquery(t *testing.T) {
	jobID := 42

	q := New().
		Update(TableAlias("job.search", "s")).
		Set(Set("updated_at", Raw("now()"))).
		From(New().
			Select("r.job_id").
			From(TableAlias("job.reports", "r")).
			Where(Col("r.job_id").Eq(jobID)),
		).
		Where(
			Col("s.job_id").Eq(jobID),
			Col("s.job_id").Eq(Col("subq_1.job_id")),
		).
		Returning("s.job_id")

	assertBuild(t, q,
		"UPDATE job.search AS s SET updated_at = now() FROM (SELECT r.job_id FROM job.reports AS r WHERE (r.job_id = ?)) AS subq_1 WHERE (s.job_id = ? AND s.job_id = subq_1.job_id) RETURNING s.job_id",
		[]any{jobID, jobID},
	)
}

func TestDeleteWithCTE(t *testing.T) {
	cte := New().
		Select("id").
		From("sessions").
		Where(Col("expires_at").Lt(Raw("now()")))

	q := New().
		With("expired", cte).
		DeleteFrom("sessions").
		Where(Col("id").In(New().Select("id").From("expired")))

	assertBuild(t, q,
		"WITH expired AS (SELECT id FROM sessions WHERE (expires_at < now())) DELETE FROM sessions WHERE (id IN (SELECT id FROM expired))",
		nil,
	)
}

func TestMixedRecursiveCTEs(t *testing.T) {
	q := New().
		With("base", New().Select("id").From("users")).
		WithRecursive("tree", New().Select("id").From("nodes")).
		Select("id").
		From("base")

	assertBuild(t, q,
		"WITH RECURSIVE base AS (SELECT id FROM users), tree AS (SELECT id FROM nodes) SELECT id FROM base",
		nil,
	)
}

func TestRawQuery(t *testing.T) {
	q := RawQuery("SELECT now()")
	assertBuild(t, q, "SELECT now()", nil)
}

func TestFullTextSearchBuilders(t *testing.T) {
	mysql := New().
		Select("id").
		From("posts").
		Where(Match("title", "body").Against("golang", "BOOLEAN MODE"))

	assertBuild(t, mysql,
		"SELECT id FROM posts WHERE (MATCH(title, body) AGAINST (? IN BOOLEAN MODE))",
		[]any{"golang"},
	)

	pg := New().
		WithDialect(DialectPostgres).
		Select("id").
		From("posts").
		Where(TsVector("title", "body").PlainQuery("safe go"))

	assertBuild(t, pg,
		"SELECT id FROM posts WHERE (to_tsvector('english', CONCAT_WS(' ', title, body)) @@ plainto_tsquery('english', $1))",
		[]any{"safe go"},
	)
}

func TestFullTextRankingHelpers(t *testing.T) {
	mysql := New().
		Select(
			Match("title", "body").Score("golang", "BOOLEAN MODE"),
			"id",
		).
		From("posts").
		Where(Match("title", "body").Against("golang", "BOOLEAN MODE")).
		OrderBy(Match("title", "body").Score("golang", "BOOLEAN MODE").Desc())

	assertBuild(t, mysql,
		"SELECT MATCH(title, body) AGAINST (? IN BOOLEAN MODE), id FROM posts WHERE (MATCH(title, body) AGAINST (? IN BOOLEAN MODE)) ORDER BY MATCH(title, body) AGAINST (? IN BOOLEAN MODE) DESC",
		[]any{"golang", "golang", "golang"},
	)

	pg := New().
		WithDialect(DialectPostgres).
		Select(
			TsVector("title", "body").RankWebSearch("safe go"),
			"id",
		).
		From("posts").
		Where(TsVector("title", "body").WebSearch("safe go")).
		OrderBy(TsVector("title", "body").RankWebSearch("safe go", 16).Desc())

	assertBuild(t, pg,
		"SELECT ts_rank(to_tsvector('english', CONCAT_WS(' ', title, body)), websearch_to_tsquery('english', $1)), id FROM posts WHERE (to_tsvector('english', CONCAT_WS(' ', title, body)) @@ websearch_to_tsquery('english', $2)) ORDER BY ts_rank(to_tsvector('english', CONCAT_WS(' ', title, body)), websearch_to_tsquery('english', $3), 16) DESC",
		[]any{"safe go", "safe go", "safe go"},
	)
}

func TestPostgresFullTextLanguage(t *testing.T) {
	pg := New().
		WithDialect(DialectPostgres).
		Select("id").
		From("posts").
		Where(TsVector("title").WithLanguage("portuguese").WebSearch("busca segura"))

	assertBuild(t, pg,
		"SELECT id FROM posts WHERE (to_tsvector('portuguese', title) @@ websearch_to_tsquery('portuguese', $1))",
		[]any{"busca segura"},
	)

	escaped := New().
		WithDialect(DialectPostgres).
		Select("id").
		From("posts").
		Where(TsVector("title").WithLanguage("portuguese'safe").PlainQuery("segura"))

	assertBuild(t, escaped,
		"SELECT id FROM posts WHERE (to_tsvector('portuguese''safe', title) @@ plainto_tsquery('portuguese''safe', $1))",
		[]any{"segura"},
	)
}

func TestDefaultTextSearchConfig(t *testing.T) {
	previous := DefaultTextSearchConfig()

	SetDefaultTextSearchConfig("simple")
	t.Cleanup(func() { SetDefaultTextSearchConfig(previous) })

	pg := New().
		WithDialect(DialectPostgres).
		Select("id").
		From("posts").
		Where(TsVector("title", "body").PlainQuery("seguro"))

	assertBuild(t, pg,
		"SELECT id FROM posts WHERE (to_tsvector('simple', CONCAT_WS(' ', title, body)) @@ plainto_tsquery('simple', $1))",
		[]any{"seguro"},
	)
}

func TestDialectSpecificFullTextSearchPanics(t *testing.T) {
	assertPanicsWith(t, func() {
		New().Select(TsVector("title").PlainQuery("oops")).From("posts").Build()
	}, "Full Text Search (tsvector) é suportado apenas no dialeto postgres")

	assertPanicsWith(t, func() {
		New().WithDialect(DialectPostgres).Select(Match("title").Against("oops")).From("posts").Build()
	}, "MATCH ... AGAINST é suportado apenas no dialeto mysql")
}

func TestPostgresPlaceholdersAndSubqueries(t *testing.T) {
	q := New().
		WithDialect(DialectPostgres).
		Select("id").
		From("users").
		Where(
			Col("id").In(New().WithDialect(DialectPostgres).Select("user_id").From("likes").Where(Col("kind").Eq("gopher"))),
			Col("status").Eq("active"),
		)

	assertBuild(t, q,
		"SELECT id FROM users WHERE (id IN (SELECT user_id FROM likes WHERE (kind = $1)) AND status = $2)",
		[]any{"gopher", "active"},
	)
}

func TestWithOrdinalityOnFunctionTable(t *testing.T) {
	q := New().
		WithDialect(DialectPostgres).
		Select("tags.tag_value", "tags.ord").
		From(TableAlias("users", "u")).
		Join(
			WithOrdinality(FuncTable("unnest", Col("u.tags")), "tags", "tag_value", "ord"),
			Col("tags.tag_value").Eq("go"),
		)

	assertBuild(t, q,
		"SELECT tags.tag_value, tags.ord FROM users AS u JOIN unnest(u.tags) WITH ORDINALITY AS tags (tag_value, ord) ON (tags.tag_value = $1)",
		[]any{"go"},
	)
}

func TestWindowFunctions(t *testing.T) {
	spec := Window().
		PartitionBy(Col("department_id")).
		OrderBy(Col("salary").Desc()).
		RowsBetween(UnboundedPreceding(), CurrentRow())

	q := New().
		Select(
			Func("row_number").Over(spec),
			Func("sum", Col("salary")).Over(spec),
			"employee_id",
		).
		From("employees")

	assertBuild(t, q,
		"SELECT row_number() OVER (PARTITION BY department_id ORDER BY salary DESC ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW), sum(salary) OVER (PARTITION BY department_id ORDER BY salary DESC ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW), employee_id FROM employees",
		nil,
	)
}

func TestAutomaticSubqueryAliases(t *testing.T) {
	q := New().
		Select("subq_1.user_id").
		From(New().Select("user_id").From("votes")).
		Where(Col("subq_1.user_id").Gt(10))

	assertBuild(t, q,
		"SELECT subq_1.user_id FROM (SELECT user_id FROM votes) AS subq_1 WHERE (subq_1.user_id > ?)",
		[]any{10},
	)

	joined := New().
		Select("subq_1.id", "subq_2.total").
		From(New().Select("id").From("posts")).
		Join(
			New().
				Select("post_id", Raw("COUNT(*) AS total")).
				From("comments").
				GroupBy("post_id"),
			Col("subq_1.id").Eq(Col("subq_2.post_id")),
		)

	assertBuild(t, joined,
		"SELECT subq_1.id, subq_2.total FROM (SELECT id FROM posts) AS subq_1 JOIN (SELECT post_id, COUNT(*) AS total FROM comments GROUP BY post_id) AS subq_2 ON (subq_1.id = subq_2.post_id)",
		nil,
	)
}

func TestUnionAllWithOrdering(t *testing.T) {
	base := New().
		Select("id", "title").
		From("posts")

	archived := New().
		Select("id", "title").
		From("archived_posts")

	union := base.UnionAll(archived).OrderBy("id DESC").Limit(5)

	assertBuild(t, union,
		"SELECT id, title FROM posts UNION ALL (SELECT id, title FROM archived_posts) ORDER BY id DESC LIMIT 5",
		nil,
	)
}

func TestUnionAllKeepsPaginationPerOperand(t *testing.T) {
	recent := New().
		Select("id", "title").
		From("posts").
		Limit(5)

	archived := New().
		Select("id", "title").
		From("archived_posts").
		Limit(5)

	union := recent.UnionAll(archived).OrderBy("id DESC")

	assertBuild(t, union,
		"SELECT id, title FROM posts LIMIT 5 UNION ALL (SELECT id, title FROM archived_posts LIMIT 5) ORDER BY id DESC",
		nil,
	)
}

func TestOnConflictMySQL(t *testing.T) {
	q := New().
		InsertInto("users", "email", "name").
		Values("a@example.com", "Jane").
		OnConflictDoUpdate(nil, Set("name", Raw("VALUES(name)")))

	assertBuild(t, q,
		"INSERT INTO users (email, name) VALUES (?, ?) ON DUPLICATE KEY UPDATE name = VALUES(name)",
		[]any{"a@example.com", "Jane"},
	)
}

func TestOnConflictPostgres(t *testing.T) {
	q := New().
		WithDialect(DialectPostgres).
		InsertInto("users", "email", "name").
		Values("a@example.com", "Jane").
		OnConflictDoUpdate([]string{"email"}, Set("name", Raw("EXCLUDED.name")), Set("updated_at", Raw("now()"))).
		Returning("id")

	assertBuild(t, q,
		"INSERT INTO users (email, name) VALUES ($1, $2) ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name, updated_at = now() RETURNING id",
		[]any{"a@example.com", "Jane"},
	)
}

func TestOnConflictDoNothingPostgres(t *testing.T) {
	q := New().
		WithDialect(DialectPostgres).
		InsertInto("events", "id", "payload").
		Values(1, "data").
		OnConflictDoNothing("id")

	assertBuild(t, q,
		"INSERT INTO events (id, payload) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING",
		[]any{1, "data"},
	)
}

func TestJSONHelpers(t *testing.T) {
	mysql := New().
		Select(
			JSONExtract("metadata", "$.title"),
			JSONExtractText("metadata", "$.author"),
		).
		From("articles").
		Where(JSONContains("metadata", `{"published":true}`))

	assertBuild(t, mysql,
		"SELECT JSON_EXTRACT(metadata, ?), JSON_UNQUOTE(JSON_EXTRACT(metadata, ?)) FROM articles WHERE (JSON_CONTAINS(metadata, ?))",
		[]any{"$.title", "$.author", "{\"published\":true}"},
	)

	pg := New().
		WithDialect(DialectPostgres).
		Select(
			JSONExtract("metadata", "$.title"),
			JSONExtractText("metadata", "$.author"),
		).
		From("articles").
		Where(JSONContains("metadata", `{"published":true}`))

	assertBuild(t, pg,
		"SELECT jsonb_path_query_first(to_jsonb(metadata), ($1)::jsonpath), (jsonb_path_query_first(to_jsonb(metadata), ($2)::jsonpath))::text FROM articles WHERE (to_jsonb(metadata) @> ($3)::jsonb)",
		[]any{"$.title", "$.author", "{\"published\":true}"},
	)
}

func TestDefaultDialectSwitching(t *testing.T) {
	prev := DefaultDialect()

	SetDefaultDialect(DialectPostgres)
	t.Cleanup(func() {
		SetDefaultDialect(prev)
	})

	q := New().
		Select("id").
		From("users").
		Where(Col("id").Eq(1))

	assertBuild(t, q,
		"SELECT id FROM users WHERE (id = $1)",
		[]any{1},
	)

	qMySQL := New().
		WithDialect(DialectMySQL).
		Select("id").
		From("users").
		Where(Col("id").Eq(2))

	assertBuild(t, qMySQL,
		"SELECT id FROM users WHERE (id = ?)",
		[]any{2},
	)
}

func TestReturningPanicsOnSelect(t *testing.T) {
	assertPanicsWith(t, func() {
		New().Select("id").Returning("id")
	}, "RETURNING is not supported on SELECT queries")
}

func TestUpdateWithoutSetPanics(t *testing.T) {
	assertPanicsWith(t, func() {
		New().Update("users").Build()
	}, "UPDATE requires at least one SET clause")
}

func TestEmptyInListPanics(t *testing.T) {
	assertPanicsWith(t, func() {
		Col("id").In()
	}, "IN list cannot be empty")

	assertPanicsWith(t, func() {
		inPredicate{left: Col("id"), list: nil}.build(&buildContext{dialect: DialectMySQL})
	}, "IN list cannot be empty")
}

func assertPanicsWith(t *testing.T, fn func(), msg string) {
	t.Helper()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic with message %q", msg)
		}

		got, ok := r.(string)
		if !ok {
			t.Fatalf("panic is not a string: %#v", r)

			return
		}

		if got != msg {
			t.Fatalf("unexpected panic message. want %q got %q", msg, got)
		}
	}()

	fn()
}
