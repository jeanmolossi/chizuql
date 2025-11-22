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
- Geração de SQL parametrizado com placeholders ajustados por dialeto (`?` para MySQL, `$1` para PostgreSQL)

## Compatibilidade
Escolha o dialeto com `WithDialect`, que alterna automaticamente os placeholders entre `?` (MySQL) e `$n` (PostgreSQL) enquanto mantém o rastreamento de argumentos.

Para configurar um dialeto padrão global (sem perder a capacidade de sobrescrever por query), use:

```go
chizuql.SetDefaultDialect(chizuql.DialectPostgres)

sql, args := chizuql.New().Select("id").From("users").Where(chizuql.Col("id").Eq(10)).Build()
// SELECT id FROM users WHERE (id = $1) | args: [10]

sqlMySQL, argsMySQL := chizuql.New().WithDialect(chizuql.DialectMySQL).Select("id").From("users").Where(chizuql.Col("id").Eq(10)).Build()
// SELECT id FROM users WHERE (id = ?) | args: [10]
```

- Desenvolvido e testado em Go ^1.25.x.

## Contribuindo e releases
- Execute sempre `go test ./...` e `golangci-lint run ./...` antes de abrir um PR ou publicar uma release.
- Atualize o `CHANGELOG.md` seguindo o modelo do Keep a Changelog: registre alterações em "Unreleased" e mova-as para uma nova seção versionada (`vX.Y.Z`) quando criar uma tag.
- Adotamos versionamento semântico. Para novas releases, garanta que testes e lint passaram, que a documentação foi atualizada e que a tag `vX.Y.Z` foi criada.

## Roadmap
- [x] Converter placeholders para os formatos específicos de drivers (ex.: `$1` em PostgreSQL) automaticamente.
- [x] Adicionar suporte a `INSERT ... ON CONFLICT`/`UPSERT` com API fluente.
- [ ] Expandir helpers de busca textual com ranking e ordenação por relevância.

## Licença
MIT
