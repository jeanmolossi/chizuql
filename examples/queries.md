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
