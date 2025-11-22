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
			Col("s.job_id").Eq(Col("r.job_id")),
		).
		Returning("s.job_id")

	assertBuild(t, q,
		"UPDATE job.search AS s SET updated_at = now() FROM (SELECT r.job_id FROM job.reports AS r WHERE (r.job_id = ?)) WHERE (s.job_id = ? AND s.job_id = r.job_id) RETURNING s.job_id",
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
		Select("id").
		From("posts").
		Where(TsVector("title", "body").PlainQuery("safe go"))

	assertBuild(t, pg,
		"SELECT id FROM posts WHERE (to_tsvector('english', CONCAT_WS(' ', title, body)) @@ plainto_tsquery('english', ?))",
		[]any{"safe go"},
	)
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
