package chizuql

import (
	"context"
	"errors"
	"fmt"
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

func TestBuildHooks(t *testing.T) {
	t.Cleanup(func() { SetGlobalBuildHooks() })

	beforeCalls := 0
	afterCalls := 0

	var (
		recordedSQL    string
		recordedArgs   []any
		recordedReport BuildReport
	)

	recordingHook := BuildHookFuncs{
		Before: func(ctx context.Context, q *Query) error {
			beforeCalls++

			return nil
		},
		After: func(ctx context.Context, result BuildResult) error {
			afterCalls++
			recordedSQL = result.SQL
			recordedArgs = result.Args
			recordedReport = result.Report

			return nil
		},
	}

	RegisterBuildHooks(BuildHookFuncs{
		Before: func(context.Context, *Query) error { return errors.New("before fail") },
		After:  func(context.Context, BuildResult) error { return errors.New("after fail") },
	})

	q := New().
		Select("id").
		From("users").
		Where(Col("id").Eq(42)).
		WithHooks(recordingHook)

	gotSQL, gotArgs := q.Build()

	if gotSQL != "SELECT id FROM users WHERE (id = ?)" {
		t.Fatalf("unexpected SQL: %s", gotSQL)
	}

	if !reflect.DeepEqual(gotArgs, []any{42}) {
		t.Fatalf("unexpected args: %#v", gotArgs)
	}

	if beforeCalls != 1 {
		t.Fatalf("expected before hook once, got %d", beforeCalls)
	}

	if afterCalls != 1 {
		t.Fatalf("expected after hook once, got %d", afterCalls)
	}

	if recordedSQL != gotSQL {
		t.Fatalf("after hook SQL mismatch: %s", recordedSQL)
	}

	if !reflect.DeepEqual(recordedArgs, gotArgs) {
		t.Fatalf("after hook args mismatch: %#v", recordedArgs)
	}

	if recordedReport.ArgsCount != len(gotArgs) {
		t.Fatalf("expected ArgsCount=%d, got %d", len(gotArgs), recordedReport.ArgsCount)
	}

	if recordedReport.DialectKind != dialectMySQL {
		t.Fatalf("expected dialect %s, got %s", dialectMySQL, recordedReport.DialectKind)
	}

	if recordedReport.RenderDuration <= 0 {
		t.Fatalf("expected positive render duration, got %s", recordedReport.RenderDuration)
	}
}

func TestBuildContextWithTelemetryPropagation(t *testing.T) {
	t.Cleanup(func() { SetGlobalBuildHooks() })

	type ctxKey string

	ctx := context.WithValue(context.Background(), ctxKey("traceparent"), "abc123")

	q := New().
		Select("id").
		From("users").
		Where(Col("id").Eq(99))

	var (
		capturedContextValue any
		report               BuildReport
	)

	RegisterBuildHooks(BuildHookFuncs{
		Before: func(ctx context.Context, _ *Query) error {
			capturedContextValue = ctx.Value(ctxKey("traceparent"))

			return nil
		},
		After: func(ctx context.Context, result BuildResult) error {
			if capturedContextValue == nil {
				capturedContextValue = ctx.Value(ctxKey("traceparent"))
			}

			report = result.Report

			return nil
		},
	})

	gotSQL, gotArgs, err := q.BuildContext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotSQL != "SELECT id FROM users WHERE (id = ?)" {
		t.Fatalf("unexpected SQL: %s", gotSQL)
	}

	if !reflect.DeepEqual(gotArgs, []any{99}) {
		t.Fatalf("unexpected args: %#v", gotArgs)
	}

	if capturedContextValue != "abc123" {
		t.Fatalf("expected context propagation, got %v", capturedContextValue)
	}

	if report.ArgsCount != 1 {
		t.Fatalf("expected ArgsCount=1, got %d", report.ArgsCount)
	}

	if report.DialectKind != dialectMySQL {
		t.Fatalf("expected dialect %s, got %s", dialectMySQL, report.DialectKind)
	}

	if report.RenderDuration <= 0 {
		t.Fatalf("expected positive render duration, got %s", report.RenderDuration)
	}
}

func TestBuildContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := New().Select("id").From("users").BuildContext(ctx)
	if err == nil {
		t.Fatalf("expected cancellation error")
	}
}

func TestAfterHookReceivesOriginalArgsSlice(t *testing.T) {
	t.Cleanup(func() { SetGlobalBuildHooks() })

	var hookPtr string

	RegisterBuildHooks(BuildHookFuncs{
		After: func(_ context.Context, result BuildResult) error {
			if len(result.Args) > 0 {
				hookPtr = fmt.Sprintf("%p", &result.Args[0])
			}

			return nil
		},
	})

	sql, args := New().Select("id").From("users").Where(Col("id").Eq(7)).Build()

	if sql == "" {
		t.Fatalf("expected SQL")
	}

	if len(args) == 0 {
		t.Fatalf("expected args")
	}

	callPtr := fmt.Sprintf("%p", &args[0])

	if hookPtr != callPtr {
		t.Fatalf("hook received copied args slice: hook=%s build=%s", hookPtr, callPtr)
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

func TestWhereWithOr(t *testing.T) {
	skipIDs := []int{3, 4}

	q := New().
		Select("doc_id", "doc_date").
		From("doc_update_queue").
		Where(
			Or(
				And(
					Col("doc_date").Gt("2025-01-01"),
					Col("doc_id").NotIn(CastAsAny(skipIDs)...),
				),
				Col("doc_id").In(101, 102),
				Col("doc_id").In(New().Select("doc_id").From("urgent_docs")),
				Col("priority").Gt(5),
			),
		).
		OrderBy("doc_id ASC")

	assertBuild(t, q,
		"SELECT doc_id, doc_date FROM doc_update_queue WHERE ((doc_date > ? AND doc_id NOT IN (?, ?)) OR doc_id IN (?, ?) OR doc_id IN (SELECT doc_id FROM urgent_docs) OR priority > ?) ORDER BY doc_id ASC",
		[]any{"2025-01-01", 3, 4, 101, 102, 5},
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

func TestRowLevelLocks(t *testing.T) {
	lockMySQL := New().
		Select("id").
		From("users").
		OrderBy("id").
		Limit(1).
		LockInShareMode()

	assertBuild(t, lockMySQL, "SELECT id FROM users ORDER BY id LIMIT 1 LOCK IN SHARE MODE", nil)

	lockPg := New().
		WithDialect(DialectPostgres).
		Select("id").
		From("users").
		ForUpdate()

	assertBuild(t, lockPg, "SELECT id FROM users FOR UPDATE", nil)
}

func TestOptimizerHints(t *testing.T) {
	q := New().
		Select("id").
		From("users").
		OptimizerHints(
			OptimizerHint("MAX_EXECUTION_TIME(500)"),
			MySQLHint("INDEX(users idx_users_status)"),
			PostgresHint("SeqScan(users) OFF"),
		)

	assertBuild(t, q,
		"SELECT /*+ MAX_EXECUTION_TIME(500) INDEX(users idx_users_status) */ id FROM users",
		nil,
	)

	pg := New().
		WithDialect(DialectPostgres).
		Select("id").
		From("users").
		OptimizerHints(
			OptimizerHint("Parallel(2)"),
			PostgresHint("SeqScan(users) OFF"),
			MySQLHint("NO_ICP(users)"),
		)

	assertBuild(t, pg,
		"SELECT /*+ Parallel(2) SeqScan(users) OFF */ id FROM users",
		nil,
	)
}

func TestRowLevelLockPanics(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic when using lock on non-select")
		}
	}()

	New().Update("users").Set(Set("name", "x")).ForUpdate()
}

func TestBetweenPredicates(t *testing.T) {
	q := New().
		Select("id").
		From("events").
		Where(
			Col("occurred_at").Between("2024-01-01", "2024-02-01"),
			Col("status").NotBetween("archived", "z"),
		)

	assertBuild(t, q,
		"SELECT id FROM events WHERE (occurred_at BETWEEN ? AND ? AND status NOT BETWEEN ? AND ?)",
		[]any{"2024-01-01", "2024-02-01", "archived", "z"},
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

func TestMySQLReturningOmitted(t *testing.T) {
	q := New().
		WithDialect(DialectMySQL).
		WithMySQLReturningMode(MySQLReturningOmit).
		Update("users").
		Set(Set("name", "Ana")).
		Where(Col("id").Eq(1)).
		Returning("id")

	assertBuild(t, q,
		"UPDATE users SET name = ? WHERE (id = ?)",
		[]any{"Ana", 1},
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

func TestGroupingSetsRollupAndCube(t *testing.T) {
	sales := New().
		Select("region", "channel", Raw("SUM(amount) AS total")).
		From("sales").
		GroupBy(
			GroupingSets(
				GroupSet("region"),
				GroupSet("channel"),
				GroupSet(),
			),
			Rollup("region", "channel"),
		).
		Having(Col("total").Gt(1000)).
		OrderBy("total DESC")

	assertBuild(t, sales,
		"SELECT region, channel, SUM(amount) AS total FROM sales GROUP BY GROUPING SETS ((region), (channel), ()), ROLLUP (region, channel) HAVING (total > ?) ORDER BY total DESC",
		[]any{1000},
	)

	cube := New().
		Select("category", "region", Raw("SUM(amount) AS sum_amount")).
		From("sales").
		GroupBy(Cube("category", "region"))

	assertBuild(t, cube,
		"SELECT category, region, SUM(amount) AS sum_amount FROM sales GROUP BY CUBE (category, region)",
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

func TestKeysetPaginationHelpers(t *testing.T) {
	q := New().
		Select("id", "created_at").
		From("posts").
		OrderBy(Col("id").Asc(), Col("created_at").Desc()).
		KeysetAfter(10, "2024-01-01 00:00:00").
		Limit(20)

	assertBuild(t, q,
		"SELECT id, created_at FROM posts WHERE ((id > ?) OR (id = ? AND created_at < ?)) ORDER BY id ASC, created_at DESC LIMIT 20",
		[]any{10, 10, "2024-01-01 00:00:00"},
	)

	prev := New().
		Select("score", "id").
		From("rankings").
		OrderBy(Col("score").Desc(), Col("id").Asc()).
		KeysetBefore(95.5, 50)

	assertBuild(t, prev,
		"SELECT score, id FROM rankings WHERE ((score > ?) OR (score = ? AND id < ?)) ORDER BY score DESC, id ASC",
		[]any{95.5, 95.5, 50},
	)
}

func TestKeysetPaginationWithRawOrderings(t *testing.T) {
	next := New().
		Select("id", "created_at").
		From("posts").
		OrderBy("id DESC", "created_at ASC").
		KeysetAfter(100, "2024-12-31 23:59:59").
		Limit(10)

	assertBuild(t, next,
		"SELECT id, created_at FROM posts WHERE ((id < ?) OR (id = ? AND created_at > ?)) ORDER BY id DESC, created_at ASC LIMIT 10",
		[]any{100, 100, "2024-12-31 23:59:59"},
	)

	prev := New().
		Select("id").
		From("posts").
		OrderBy("id DESC").
		KeysetBefore(42)

	assertBuild(t, prev,
		"SELECT id FROM posts WHERE ((id > ?)) ORDER BY id DESC",
		[]any{42},
	)
}

func TestKeysetPaginationPanics(t *testing.T) {
	assertPanicsWith(t, func() {
		New().Select("id").From("items").KeysetAfter(1)
	}, "KeysetAfter requer ORDER BY configurado")

	assertPanicsWith(t, func() {
		KeysetAfter([]Expression{Col("id").Asc()}, 1, 2)
	}, "a quantidade de valores de cursor deve corresponder ao ORDER BY configurado")
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

func TestInsertIgnoreMySQL(t *testing.T) {
	q := New().
		InsertInto("users", "email").
		Values("a@example.com").
		InsertIgnore()

	assertBuild(t, q,
		"INSERT IGNORE INTO users (email) VALUES (?)",
		[]any{"a@example.com"},
	)
}

func TestInsertIgnorePostgres(t *testing.T) {
	q := New().
		WithDialect(DialectPostgres).
		InsertInto("users", "email").
		Values("a@example.com").
		InsertIgnore().
		Returning("id")

	assertBuild(t, q,
		"INSERT INTO users (email) VALUES ($1) ON CONFLICT DO NOTHING RETURNING id",
		[]any{"a@example.com"},
	)
}

func TestInsertIgnoreSQLite(t *testing.T) {
	q := New().
		WithDialect(DialectSQLite).
		InsertInto("users", "email").
		Values("a@example.com").
		InsertIgnore()

	assertBuild(t, q,
		"INSERT INTO users (email) VALUES (?) ON CONFLICT DO NOTHING",
		[]any{"a@example.com"},
	)
}

func TestInsertIgnoreWithExplicitConflictPanics(t *testing.T) {
	assertPanicsWith(t, func() {
		New().
			InsertInto("users", "email").
			InsertIgnore().
			OnConflictDoUpdate([]string{"email"}, Set("name", Raw("EXCLUDED.name"))).
			Build()
	}, "INSERT IGNORE não pode ser combinado com handlers explícitos de conflito")
}

func TestInsertIgnoreWithoutTableReturnsError(t *testing.T) {
	q := New().
		InsertIgnore().
		Values("a@example.com")

	_, _, err := q.BuildContext(context.Background())
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}

	const want = "InsertInto must be called before InsertIgnore/Build for INSERT queries"

	if err.Error() != want {
		t.Fatalf("unexpected error.\nwant: %s\n got: %v", want, err)
	}
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

func TestNotInWithValues(t *testing.T) {
	ids := []int{1, 2, 3}

	q := New().
		Select("id").
		From("users").
		Where(Col("id").NotIn(CastAsAny(ids)...))

	assertBuild(t, q,
		"SELECT id FROM users WHERE (id NOT IN (?, ?, ?))",
		[]any{1, 2, 3},
	)
}

func TestNotInWithSubquery(t *testing.T) {
	q := New().
		Select("id").
		From("users").
		Where(
			Col("id").NotIn(New().Select("user_id").From("logs")),
			Col("status").Eq("active"),
		)

	assertBuild(t, q,
		"SELECT id FROM users WHERE (id NOT IN (SELECT user_id FROM logs) AND status = ?)",
		[]any{"active"},
	)
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
