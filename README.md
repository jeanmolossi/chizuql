# ChizuQL

ChizuQL é um query builder fluente para Go. Ele permite montar SQL parametrizado com clareza, incluindo seleções simples, JOINs, subqueries, CTEs (`WITH`), buscas textuais e cláusulas `RETURNING`. O foco é gerar SQL seguro com placeholders (`?`) e uma API expressiva.

## Instalação

```bash
go get github.com/jeanmolossi/chizuql
```

## Uso rápido

### SELECT básico
```go
q := chizuql.New().
    Select("id", chizuql.ColAlias("name", "nome")).
    From(chizuql.TableAlias("users", "u")).
    Where(
        chizuql.Col("status").Eq("active"),
    ).
    OrderBy("created_at DESC").
    Limit(20)

sql, args := q.Build()
```

### UPDATE com subquery e comparações entre colunas
```go
q := chizuql.New().
    Update(chizuql.TableAlias("job.search", "s")).
    Set(
        chizuql.Set("updated_at", chizuql.Raw("now()")),
    ).
    From(
        chizuql.New().
            From(chizuql.TableAlias("job.reports", "r")).
            Select("r.job_id").
            Where(chizuql.Col("r.job_id").Eq(jobID)),
    ).
    Where(
        chizuql.Col("s.job_id").Eq(jobID),
        chizuql.Col("s.job_id").Eq(chizuql.Col("r.job_id")),
    ).
    Returning("s.job_id")

sql, args := q.Build()
```

### INSERT com múltiplas linhas e RETURNING
```go
insert := chizuql.New().
    InsertInto("users", "name", "email").
    Values("Jane", "jane@example.com").
    Values("John", "john@example.com").
    Returning("id")

sql, args := insert.Build()
```

### DELETE com CTE
```go
cte := chizuql.New().
    Select("id").
    From("sessions").
    Where(chizuql.Col("expires_at").Lt(chizuql.Raw("now()")))

del := chizuql.New().
    With("expired", cte).
    DeleteFrom("sessions").
    Where(chizuql.Col("id").In(chizuql.New().Select("id").From("expired")))

sql, args := del.Build()
```

### Buscas textuais
```go
// MySQL
match := chizuql.Match("title", "body").Against("golang", "BOOLEAN MODE")
q := chizuql.New().Select("id").From("posts").Where(match)

// PostgreSQL
fts := chizuql.TsVector("title", "body").WebSearch("golang seguro")
pq := chizuql.New().Select("id").From("posts").Where(fts)
```

### Raw query
```go
raw := chizuql.RawQuery("SELECT now()")
sql, args := raw.Build()
```

## Recursos principais
- SELECT, INSERT, UPDATE, DELETE com cláusulas fluentes
- JOINs, subqueries e CTEs (`WITH` e `WITH RECURSIVE`)
- Predicados compostos (`AND`/`OR`), `IN`, `BETWEEN`, `LIKE`, `IS NULL`
- Comparação entre colunas e uso de expressões cruas com `Raw`
- Suporte a `RETURNING`
- Builders de busca textual para MySQL (`MATCH ... AGAINST`) e PostgreSQL (`to_tsvector` + `websearch_to_tsquery` ou `plainto_tsquery`)
- Geração de SQL parametrizado com placeholders `?`

## Compatibilidade
O builder é agnóstico ao banco e produz placeholders `?`, compatíveis diretamente com MySQL. Para PostgreSQL, utilize a estratégia de substituir placeholders no driver ou no prepared statement conforme sua stack.

## Licença
MIT
