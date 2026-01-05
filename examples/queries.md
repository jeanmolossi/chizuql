# Coletânea de exemplos do ChizuQL

Cada exemplo abaixo segue o formato "query com o builder" → "saída da query" → "comentários" para mostrar, na prática, como o ChizuQL gera SQL parametrizado. Todos os exemplos foram escritos para Go 1.25 e assumem `import chizuql "github.com/jeanmolossi/chizuql"`.

### 1. Seleção básica com ordenação e paginação
**Query**
```go
q := chizuql.New().
    Select("id", chizuql.ColAlias("name", "nome")).
    From(chizuql.TableAlias("users", "u")).
    Where(
        chizuql.Col("status").Eq("active"),
        chizuql.Col("age").Gt(18),
    ).
    OrderBy("created_at DESC").
    Limit(10).
    Offset(5)

sql, args := q.Build()
```

**Saída gerada**
```
SELECT id, name AS nome FROM users AS u WHERE (status = ? AND age > ?) ORDER BY created_at DESC LIMIT 10 OFFSET 5
args: ["active" 18]
```

**Comentários**
- O dialeto padrão é MySQL, então os placeholders são `?`.
- O encadeamento `Where` recebe vários predicados e agrupa tudo em um único bloco com `AND`.

### 2. JOIN com agregação e filtro em HAVING
**Query**
```go
q := chizuql.New().
    Select("u.id", chizuql.ColAlias("COUNT(p.id)", "post_count")).
    From(chizuql.TableAlias("users", "u")).
    LeftJoin(chizuql.TableAlias("posts", "p"), chizuql.Col("u.id").Eq(chizuql.Col("p.user_id"))).
    GroupBy("u.id").
    Having(chizuql.Col("COUNT(p.id)").Gt(0)).
    OrderBy("post_count DESC")

sql, args := q.Build()
```

**Saída gerada**
```
SELECT u.id, COUNT(p.id) AS post_count FROM users AS u LEFT JOIN posts AS p ON (u.id = p.user_id) GROUP BY u.id HAVING (COUNT(p.id) > ?) ORDER BY post_count DESC
args: [0]
```

**Comentários**
- `Having` aceita um predicado separado do `Where`, útil para filtros pós-agrupamento.
- O alias `post_count` pode ser usado diretamente na cláusula `ORDER BY`.

### 3. Subconsulta em FROM com alias automático
**Query**
```go
q := chizuql.New().
    Select("subq_1.user_id").
    From(
        chizuql.New().
            Select("user_id").
            From("votes"),
    ).
    Where(chizuql.Col("subq_1.user_id").Gt(10))

sql, args := q.Build()
```

**Saída gerada**
```
SELECT subq_1.user_id FROM (SELECT user_id FROM votes) AS subq_1 WHERE (subq_1.user_id > ?)
args: [10]
```

**Comentários**
- Quando o alias não é informado no `FromSubquery`, o builder gera nomes únicos (`subq_1`, `subq_2`...).
- Os aliases automáticos também são reutilizados em joins ou filtros subsequentes.

### 4. UNION ALL com paginação global
**Query**
```go
recent := chizuql.New().
    Select("id", "title").
    From("posts").
    Limit(5)

archived := chizuql.New().
    Select("id", "title").
    From("archived_posts").
    Limit(5)

union := recent.UnionAll(archived).
    OrderBy("id DESC").
    Limit(5)

sql, args := union.Build()
```

**Saída gerada**
```
SELECT id, title FROM posts LIMIT 5 UNION ALL (SELECT id, title FROM archived_posts LIMIT 5) ORDER BY id DESC LIMIT 5
args: []
```

**Comentários**
- Cada operando mantém sua própria paginação; `Limit(5)` após `UnionAll` afeta o resultado consolidado.
- Os parênteses são adicionados automaticamente em cada SELECT combinado.

### 4.1 Paginação por cursor (keyset) para página seguinte
**Query**
```go
page := chizuql.New().
    Select("id", "created_at").
    From("posts").
    OrderBy(
        chizuql.Col("id").Asc(),
        chizuql.Col("created_at").Desc(),
    ).
    KeysetAfter(120, "2025-01-01 00:00:00").
    Limit(20)

sql, args := page.Build()
```

**Saída gerada**
```
SELECT id, created_at FROM posts WHERE ((id > ? OR (id = ? AND created_at < ?))) ORDER BY id ASC, created_at DESC LIMIT 20
args: [120 120 "2025-01-01 00:00:00"]
```

**Comentários**
- `KeysetAfter` constrói o predicado de cursor combinando os mesmos campos do `ORDER BY` e respeita direções `DESC`.
- Para navegar para a página anterior basta usar `KeysetBefore` com o mesmo cursor; os comparadores são invertidos automaticamente.

### 5. INSERT multi-linha com RETURNING
**Query**
```go
q := chizuql.New().
    InsertInto("users", "name", "email").
    Values("Jane", "jane@example.com").
    Values("John", "john@example.com").
    Returning("id")

sql, args := q.Build()
```

**Saída gerada**
```
INSERT INTO users (name, email) VALUES (?, ?), (?, ?) RETURNING id
args: ["Jane" "jane@example.com" "John" "john@example.com"]
```

**Comentários**
- O `RETURNING` funciona para INSERT/UPDATE/DELETE, mas é rejeitado em SELECT.
- O builder aceita quantas linhas `Values` forem necessárias.

### 5.1 INSERT ignorando conflitos por dialeto
**Query**
```go
mysql := chizuql.New().
    InsertInto("users", "email").
    Values("a@example.com").
    InsertIgnore()

postgres := chizuql.New().
    WithDialect(chizuql.DialectPostgres).
    InsertInto("users", "email").
    Values("a@example.com").
    InsertIgnore().
    Returning("id")

sqlite := chizuql.New().
    WithDialect(chizuql.DialectSQLite).
    InsertInto("users", "email").
    Values("a@example.com").
    InsertIgnore()
```

**Saída gerada**
```sql
-- MySQL (default)
INSERT IGNORE INTO users (email) VALUES (?)
args: ["a@example.com"]

-- PostgreSQL
INSERT INTO users (email) VALUES ($1) ON CONFLICT DO NOTHING RETURNING id
args: ["a@example.com"]

-- SQLite
INSERT INTO users (email) VALUES (?) ON CONFLICT DO NOTHING
args: ["a@example.com"]
```

**Comentários**
- `InsertIgnore` adapta a sintaxe: `INSERT IGNORE` em MySQL e `ON CONFLICT DO NOTHING` em PostgreSQL/SQLite.
- O encadeamento de `Returning` continua válido para dialetos que o suportam.

### 6. UPSERT PostgreSQL com `ON CONFLICT DO UPDATE`
**Query**
```go
q := chizuql.New().
    WithDialect(chizuql.DialectPostgres).
    InsertInto("users", "email", "name").
    Values("a@example.com", "Jane").
    OnConflictDoUpdate(
        []string{"email"},
        chizuql.Set("name", chizuql.Raw("EXCLUDED.name")),
        chizuql.Set("updated_at", chizuql.Raw("now()")),
    ).
    Returning("id")

sql, args := q.Build()
```

**Saída gerada**
```
INSERT INTO users (email, name) VALUES ($1, $2) ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name, updated_at = now() RETURNING id
args: ["a@example.com" "Jane"]
```

**Comentários**
- Ao alternar para PostgreSQL, os placeholders passam a ser numerados (`$1`, `$2`...).
- `OnConflictDoUpdate` aceita a lista de colunas que formam a chave de conflito e múltiplos `Set` para a atualização.

### 7. UPDATE com subconsulta e `RETURNING`
**Query**
```go
jobID := 42

q := chizuql.New().
    Update(chizuql.TableAlias("job.search", "s")).
    Set(chizuql.Set("updated_at", chizuql.Raw("now()"))).
    From(
        chizuql.New().
            Select("r.job_id").
            From(chizuql.TableAlias("job.reports", "r")).
            Where(chizuql.Col("r.job_id").Eq(jobID)),
    ).
    Where(
        chizuql.Col("s.job_id").Eq(jobID),
        chizuql.Col("s.job_id").Eq(chizuql.Col("subq_1.job_id")),
    ).
    Returning("s.job_id")

sql, args := q.Build()
```

**Saída gerada**
```
UPDATE job.search AS s SET updated_at = now() FROM (SELECT r.job_id FROM job.reports AS r WHERE (r.job_id = ?)) AS subq_1 WHERE (s.job_id = ? AND s.job_id = subq_1.job_id) RETURNING s.job_id
args: [42 42]
```

**Comentários**
- A subconsulta em `FROM` ganha alias automático (`subq_1`), reutilizado no filtro principal.
- `Set` aceita expressões cruas, permitindo uso de funções (`now()`) sem placeholders.

### 8. DELETE amarrado a uma CTE
**Query**
```go
expired := chizuql.New().
    Select("id").
    From("sessions").
    Where(chizuql.Col("expires_at").Lt(chizuql.Raw("now()")))

q := chizuql.New().
    With("expired", expired).
    DeleteFrom("sessions").
    Where(chizuql.Col("id").In(chizuql.New().Select("id").From("expired")))

sql, args := q.Build()
```

**Saída gerada**
```
WITH expired AS (SELECT id FROM sessions WHERE (expires_at < now())) DELETE FROM sessions WHERE (id IN (SELECT id FROM expired))
args: []
```

**Comentários**
- A CTE pode ser recursiva (`WithRecursive`) quando necessário; o builder insere `WITH RECURSIVE` automaticamente.
- A cláusula `In` aceita tanto listas quanto subconsultas completas.

### 9. Busca textual em MySQL e PostgreSQL
**Query (MySQL)**
```go
q := chizuql.New().
    Select(chizuql.Match("title", "body").Score("golang", "BOOLEAN MODE"), "id").
    From("posts").
    Where(chizuql.Match("title", "body").Against("golang", "BOOLEAN MODE")).
    OrderBy(chizuql.Match("title", "body").Score("golang", "BOOLEAN MODE").Desc())

sql, args := q.Build()
```

**Saída gerada (MySQL)**
```
SELECT MATCH(title, body) AGAINST (? IN BOOLEAN MODE), id FROM posts WHERE (MATCH(title, body) AGAINST (? IN BOOLEAN MODE)) ORDER BY MATCH(title, body) AGAINST (? IN BOOLEAN MODE) DESC
args: ["golang" "golang" "golang"]
```

**Query (PostgreSQL)**
```go
pg := chizuql.New().
    WithDialect(chizuql.DialectPostgres).
    Select(
        chizuql.TsVector("title", "body").RankWebSearch("safe go"),
        "id",
    ).
    From("posts").
    Where(chizuql.TsVector("title", "body").WebSearch("safe go")).
    OrderBy(chizuql.TsVector("title", "body").RankWebSearch("safe go", 16).Desc())

sql, args := pg.Build()
```

**Saída gerada (PostgreSQL)**
```
SELECT ts_rank(to_tsvector('english', CONCAT_WS(' ', title, body)), websearch_to_tsquery('english', $1)), id FROM posts WHERE (to_tsvector('english', CONCAT_WS(' ', title, body)) @@ websearch_to_tsquery('english', $2)) ORDER BY ts_rank(to_tsvector('english', CONCAT_WS(' ', title, body)), websearch_to_tsquery('english', $3), 16) DESC
args: ["safe go" "safe go" "safe go"]
```

**Comentários**
- Cada dialeto valida as funções disponíveis: `MATCH ... AGAINST` só funciona no MySQL e `TsVector` apenas no PostgreSQL.
- `RankWebSearch` aceita um fator de normalização opcional (ex.: `16`) para ajustar a pontuação.

### 10. Filtros e extração em JSON/JSONB
**Query**
```go
mysql := chizuql.New().
    Select(
        chizuql.JSONExtract("metadata", "$.title"),
        chizuql.JSONExtractText("metadata", "$.author"),
    ).
    From("articles").
    Where(chizuql.JSONContains("metadata", `{"published":true}`))

pg := chizuql.New().
    WithDialect(chizuql.DialectPostgres).
    Select(
        chizuql.JSONExtract("metadata", "$.title"),
        chizuql.JSONExtractText("metadata", "$.author"),
    ).
    From("articles").
    Where(chizuql.JSONContains("metadata", `{"published":true}`))

mysqlSQL, mysqlArgs := mysql.Build()
pgSQL, pgArgs := pg.Build()
```

**Saída gerada (MySQL)**
```
SELECT JSON_EXTRACT(metadata, ?), JSON_UNQUOTE(JSON_EXTRACT(metadata, ?)) FROM articles WHERE (JSON_CONTAINS(metadata, ?))
args: ["$.title" "$.author" "{\"published\":true}"]
```

**Saída gerada (PostgreSQL)**
```
SELECT jsonb_path_query_first(to_jsonb(metadata), ($1)::jsonpath), (jsonb_path_query_first(to_jsonb(metadata), ($2)::jsonpath))::text FROM articles WHERE (to_jsonb(metadata) @> ($3)::jsonb)
args: ["$.title" "$.author" "{\"published\":true}"]
```

**Comentários**
- A mesma API gera SQL compatível com ambos dialetos, alterando apenas os placeholders e funções nativas.
- `JSONContains` converte o JSON para `jsonb` em PostgreSQL e usa `JSON_CONTAINS` em MySQL.

### 11. Expansão de arrays com `WITH ORDINALITY` (PostgreSQL)
**Query**
```go
q := chizuql.New().
    WithDialect(chizuql.DialectPostgres).
    Select("u.id", "tags.tag_value", "tags.ord").
    From(chizuql.TableAlias("users", "u")).
    Join(
        chizuql.WithOrdinality(
            chizuql.FuncTable("unnest", chizuql.Col("u.tags")),
            "tags",
            "tag_value",
            "ord",
        ),
        chizuql.Col("tags.tag_value").Eq("go"),
    ).
    OrderBy(chizuql.Col("tags.ord").Asc())

sql, args := q.Build()
```

**Saída gerada**
```
SELECT u.id, tags.tag_value, tags.ord FROM users AS u JOIN unnest(u.tags) WITH ORDINALITY AS tags (tag_value, ord) ON (tags.tag_value = $1) ORDER BY tags.ord ASC
args: ["go"]
```

**Comentários**
- `FuncTable("unnest", ...)` cria uma função set-returning segura para usar em `FROM`/`JOIN`.
- `WithOrdinality` só está disponível para PostgreSQL; a coluna adicional de ordem recebe o nome informado na lista de colunas.
- `Asc`/`Desc` em colunas permitem expressar ordenação sem cair em `Raw`.

### 12. Window functions com partição e frame
**Query**
```go
spec := chizuql.Window().
    PartitionBy(chizuql.Col("department_id")).
    OrderBy(chizuql.Col("salary").Desc()).
    RowsBetween(chizuql.UnboundedPreceding(), chizuql.CurrentRow())

q := chizuql.New().
    Select(
        chizuql.Func("row_number").Over(spec),
        chizuql.Func("sum", chizuql.Col("salary")).Over(spec),
        "employee_id",
    ).
    From("employees")

sql, args := q.Build()
```

**Saída gerada**
```
SELECT row_number() OVER (PARTITION BY department_id ORDER BY salary DESC ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW), sum(salary) OVER (PARTITION BY department_id ORDER BY salary DESC ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW), employee_id FROM employees
args: []
```

**Comentários**
- `Window()` compõe `PARTITION BY`, `ORDER BY` e frames `ROWS`/`RANGE` de forma declarativa.
- `Func(...).Over(spec)` funciona para qualquer função agregada ou analítica, incluindo aliases ou expressões mais complexas.
- Frames aceitam limites como `UnboundedPreceding`, `CurrentRow`, `Preceding(n)` e `Following(n)` para personalizar janelas.

### 13. Filtros com BETWEEN e NOT BETWEEN
**Query**
```go
q := chizuql.New().
    Select("id", "occurred_at", "status").
    From("events").
    Where(
        chizuql.Col("occurred_at").Between("2024-01-01", "2024-02-01"),
        chizuql.Col("status").NotBetween("archived", "zzz"),
    )

sql, args := q.Build()
```

**Saída gerada**
```
SELECT id, occurred_at, status FROM events WHERE (occurred_at BETWEEN ? AND ? AND status NOT BETWEEN ? AND ?)
args: ["2024-01-01" "2024-02-01" "archived" "zzz"]
```

**Comentários**
- `Between` e `NotBetween` aceitam qualquer expressão, inclusive `Raw` ou subconsultas.
- Vários predicados informados em `Where` continuam agrupados por `AND`.

### 13.1 Filtro com NOT IN usando CastAsAny
**Query**
```go
logIDs := []int{10, 11, 12}

q := chizuql.New().
    Select("v.vag_id").
    From(chizuql.TableAlias("vag", "v")).
    Where(
        chizuql.Col("v.vag_id").NotIn(chizuql.CastAsAny(logIDs)...),
        chizuql.Col("v.vag_id").Gt(100),
    )

sql, args := q.Build()
```

**Saída gerada**
```
SELECT v.vag_id FROM vag AS v WHERE (v.vag_id NOT IN (?, ?, ?) AND v.vag_id > ?)
args: [10 11 12 100]
```

**Comentários**
- `CastAsAny` converte slices tipados em `[]any`, facilitando o uso em chamadas variádicas como `In`/`NotIn`.
- `NotIn` aceita tanto slices quanto subconsultas, mantendo o tratamento de placeholders por dialeto.

### 14. Agrupamentos avançados com GROUPING SETS e ROLLUP
**Query**
```go
q := chizuql.New().
    Select("region", "channel", chizuql.Raw("SUM(amount) AS total")).
    From("sales").
    GroupBy(
        chizuql.GroupingSets(
            chizuql.GroupSet("region"),
            chizuql.GroupSet("channel"),
            chizuql.GroupSet(), // total geral
        ),
        chizuql.Rollup("region", "channel"),
    ).
    Having(chizuql.Col("total").Gt(1000)).
    OrderBy("total DESC")

sql, args := q.Build()
```

**Saída gerada**
```
SELECT region, channel, SUM(amount) AS total FROM sales GROUP BY GROUPING SETS ((region), (channel), ()), ROLLUP (region, channel) HAVING (total > ?) ORDER BY total DESC
args: [1000]
```

**Comentários**
- `GroupingSets` aceita uma lista de `GroupSet` (incluindo o conjunto vazio `()` para totalizações gerais).
- `Rollup` gera totais acumulados na ordem informada.
- É possível combinar `GroupingSets`, `Rollup` e colunas comuns na mesma cláusula `GROUP BY`.

### 15. CUBE e controle de RETURNING no MySQL
**Query**
```go
cube := chizuql.New().
    Select("category", "region", chizuql.Raw("SUM(amount) AS sum_amount")).
    From("sales").
    GroupBy(chizuql.Cube("category", "region"))

// UPDATE com RETURNING desabilitado para MySQL abaixo da 8.0
update := chizuql.New().
    WithDialect(chizuql.DialectMySQL).
    WithMySQLReturningMode(chizuql.MySQLReturningOmit).
    Update("users").
    Set(chizuql.Set("name", "Ana")).
    Where(chizuql.Col("id").Eq(1)).
    Returning("id")

cubeSQL, cubeArgs := cube.Build()
updateSQL, updateArgs := update.Build()
```

**Saída gerada**
```
SELECT category, region, SUM(amount) AS sum_amount FROM sales GROUP BY CUBE (category, region)
args: []

UPDATE users SET name = ? WHERE (id = ?)
args: ["Ana" 1]
```

**Comentários**
- `Cube` cria todas as combinações possíveis entre as colunas informadas.
- `WithMySQLReturningMode(MySQLReturningOmit)` remove `RETURNING` para dialetos MySQL, evitando erros em servidores que não suportam a cláusula.
- Para MySQL 8.0+, mantenha o modo padrão (`MySQLReturningStrict`) e a cláusula `RETURNING` será emitida normalmente.

### 16. Locks de linha (`FOR UPDATE`/`LOCK IN SHARE MODE`)
**Query**
```go
lockShared := chizuql.New().
    Select("id", "email").
    From("users").
    Where(chizuql.Col("status").Eq("pending")).
    OrderBy("id ASC").
    Limit(10).
    LockInShareMode()

lockUpdate := chizuql.New().
    WithDialect(chizuql.DialectPostgres).
    Select("id").
    From("jobs").
    Where(chizuql.Col("locked_at").IsNull()).
    OrderBy("created_at").
    ForUpdate()

sqlShared, argsShared := lockShared.Build()
sqlUpdate, argsUpdate := lockUpdate.Build()
```

**Saída gerada**
```
SELECT id, email FROM users WHERE (status = ?) ORDER BY id ASC LIMIT 10 LOCK IN SHARE MODE
args: ["pending"]

SELECT id FROM jobs WHERE (locked_at IS NULL) ORDER BY created_at FOR UPDATE
args: []
```

**Comentários**
- `LockInShareMode` adapta a cláusula ao dialeto: em MySQL gera `LOCK IN SHARE MODE`; em PostgreSQL rende `FOR SHARE`.
- `ForUpdate` sempre gera `FOR UPDATE` e é útil para enfileirar/consumir registros com controle de concorrência.
- Locks são aplicados após `ORDER BY`/`LIMIT`, combinando bem com paginação de lotes.

### 17. Build com `context.Context` e métricas
**Query**
```go
ctx := context.Background()

q := chizuql.New().
    Select("id", "name").
    From("users").
    Where(chizuql.Col("deleted_at").IsNull())

sql, args, err := q.BuildContext(ctx)
if err != nil {
    return err
}

fmt.Println("SQL:", sql)
fmt.Println("args:", args)
```

**Saída gerada**
```
SQL: SELECT id, name FROM users WHERE (deleted_at IS NULL)
args: []
```

**Comentários**
- `BuildContext` propaga o contexto para os hooks de build (útil para tracing/logs) e aceita cancelamento por contexto.
- Métricas de renderização continuam acessíveis via `BuildResult` dentro dos hooks `BeforeBuild`/`AfterBuild`.
- Para cancelar rapidamente builds longos, passe um contexto com deadline ou cancelamento antecipado.
- O método `Build` tradicional permanece disponível para chamadas simples sem contexto.

### 18. Optimizer hints específicos por dialeto
**Query (MySQL)**
```go
q := chizuql.New().
    Select("id").
    From("users").
    OptimizerHints(
        chizuql.OptimizerHint("MAX_EXECUTION_TIME(500)"),
        chizuql.MySQLHint("INDEX(users idx_users_status)"),
        chizuql.PostgresHint("SeqScan(users) OFF"),
    )

sql, args := q.Build()
```

**Saída gerada (MySQL)**
```
SELECT /*+ MAX_EXECUTION_TIME(500) INDEX(users idx_users_status) */ id FROM users
args: []
```

**Query (PostgreSQL)**
```go
pg := q.WithDialect(chizuql.DialectPostgres)

sqlPg, argsPg := pg.Build()
```

**Saída gerada (PostgreSQL)**
```
SELECT /*+ SeqScan(users) OFF */ id FROM users
args: []
```

**Comentários**
- `OptimizerHints` injeta comentários `/*+ ... */` logo após o verbo da query.
- Hints restritos a um dialeto são ignorados automaticamente quando o dialeto atual não corresponde.

### 19. Integração com GORM, sqlc e migrações
**Query (builder + GORM)**
```go
q := chizuql.New().
    Select("id", "name").
    From("users").
    Where(chizuql.Col("status").Eq("active")).
    OptimizerHints(chizuql.MySQLHint("INDEX(users idx_users_status)"))

sql, args := q.Build()
var users []User
err := db.Raw(sql, args...).Scan(&users).Error
```

**Query (builder + sqlc)**
```go
pg := chizuql.New().
    WithDialect(chizuql.DialectPostgres).
    Select("id", "email").
    From("users").
    Where(chizuql.Col("id").In(1, 2))

sqlPg, argsPg := pg.Build()
rows, err := db.QueryContext(ctx, sqlPg, argsPg...)
```

**Query (migração direta)**
```go
migration := chizuql.New().
    WithDialect(chizuql.DialectPostgres).
    RawQuery("ALTER TABLE users ADD COLUMN metadata JSONB DEFAULT '{}'::jsonb")

sqlMig, argsMig := migration.Build()
_, err := db.ExecContext(ctx, sqlMig, argsMig...)
```

**Comentários**
- A saída do builder (`sql` + `args`) pode ser repassada diretamente a `db.Raw` do GORM ou aos métodos `QueryContext`/`ExecContext` usados pelo sqlc.
- Para migrações estáticas, `RawQuery` permite compartilhar comandos validados (inclusive para rollback) com a mesma convenção de placeholders do projeto.

### 20. Hooks de build para métricas e logs
**Query**
```go
var lastDuration time.Duration

chizuql.RegisterBuildHooks(chizuql.BuildHookFuncs{
    After: func(ctx context.Context, result chizuql.BuildResult) error {
        lastDuration = result.Report.RenderDuration

        fmt.Printf("[hook] sql=%s | args=%v | placeholders=%d | duration=%s\n", result.SQL, result.Args, result.Report.ArgsCount, result.Report.RenderDuration)

        return nil
    },
})

q := chizuql.New().
    WithHooks(chizuql.BuildHookFuncs{
        Before: func(ctx context.Context, _ *chizuql.Query) error {
            fmt.Println("montando consulta de usuários ativos")

            return nil
        },
    }).
    Select("id", "email").
    From("users").
    Where(chizuql.Col("status").Eq("active"))

sql, args := q.Build()
```

**Saída gerada**
```
montando consulta de usuários ativos
[hook] sql=SELECT id, email FROM users WHERE (status = ?) | args=["active"] | placeholders=1 | duration=173.191µs
SELECT id, email FROM users WHERE (status = ?)
args: ["active"]
```

**Comentários**
- Hooks globais (via `RegisterBuildHooks`) e específicos por query (`WithHooks`) podem coexistir. As callbacks recebem o contexto usado na build, o SQL final, os argumentos e o `BuildReport` com métricas de duração e contagem de placeholders.
- Erros retornados pelos hooks são ignorados para não bloquear a renderização; use-os para métricas, logs ou tracing.

### 21. Hooks especializados (OpenTelemetry + slog)
**Query**
```go
tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
defer tp.Shutdown(context.Background())
defer mp.Shutdown(context.Background())

chizuql.RegisterBuildHooks(
    hooks.TracingHook{Tracer: tp.Tracer("queries"), IncludeSQL: true},
    &hooks.MetricsHook{Meter: mp.Meter("queries")},
    hooks.LoggingHook{IncludeSQL: true},
)

ctx := context.WithValue(context.Background(), "traceparent", "00-abcdef0123456789abcdef0123456789-0123456789abcdef-01")

sql, args, err := chizuql.New().
    Select("id").
    From("orders").
    Where(chizuql.Col("status").Eq("paid")).
    BuildContext(ctx)

if err != nil {
    panic(err)
}

fmt.Println(sql, args)
```

**Comentários**
- `TracingHook` cria spans com `db.system`, contagem de argumentos e duração de renderização, opcionalmente incluindo o SQL em `db.statement`.
- `MetricsHook` publica histogramas de duração (`chizuql.build.duration_ms`) e contadores de placeholders (`chizuql.build.args`) usando o `Meter` fornecido.
- `LoggingHook` emite logs estruturados com `log/slog`, podendo anexar SQL e argumentos quando `IncludeSQL` estiver habilitado.
