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
        chizuql.FromSubquery(
            chizuql.New().
                From(chizuql.TableAlias("job.reports", "r")).
                Select("r.job_id").
                Where(chizuql.Col("r.job_id").Eq(jobID)),
            "rpt",
        ),
    ).
    Where(
        chizuql.Col("s.job_id").Eq(jobID),
        chizuql.Col("s.job_id").Eq(chizuql.Col("rpt.job_id")),
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

### UNION/UNION ALL com ordenação final
```go
recent := chizuql.New().
    Select("id", "title").
    From("posts")

archived := chizuql.New().
    Select("id", "title").
    From("archived_posts")

union := recent.
    UnionAll(archived).
    OrderBy("id DESC").
    Limit(5)

sql, args := union.Build()
```

### Buscas textuais
```go
// MySQL
match := chizuql.Match("title", "body").Against("golang", "BOOLEAN MODE")
q := chizuql.New().Select("id").From("posts").Where(match)

score := chizuql.Match("title", "body").Score("golang", "BOOLEAN MODE")
ordered := chizuql.New().
    Select(score, "id").
    From("posts").
    Where(match).
    OrderBy(score.Desc())

// PostgreSQL
fts := chizuql.TsVector("title", "body").WebSearch("golang seguro")
pq := chizuql.New().Select("id").From("posts").Where(fts)

rank := chizuql.TsVector("title", "body").RankWebSearch("golang seguro", 16)
orderedPg := chizuql.New().
    WithDialect(chizuql.DialectPostgres).
    Select(rank, "id").
    From("posts").
    Where(fts).
    OrderBy(rank.Desc())

// PostgreSQL com idioma alternado
pt := chizuql.TsVector("title", "body").WithLanguage("portuguese").WebSearch("busca segura")
localized := chizuql.New().
    WithDialect(chizuql.DialectPostgres).
    Select("id").
    From("posts").
    Where(pt)
```

### JSON/JSONB
```go
// MySQL
q := chizuql.New().
    Select(
        chizuql.JSONExtract("metadata", "$.title"),
        chizuql.JSONExtractText("metadata", "$.author"),
    ).
    From("articles").
    Where(chizuql.JSONContains("metadata", `{"published":true}`))

// PostgreSQL
pg := chizuql.New().
    WithDialect(chizuql.DialectPostgres).
    Select(
        chizuql.JSONExtract("metadata", "$.title"),
        chizuql.JSONExtractText("metadata", "$.author"),
    ).
    From("articles").
    Where(chizuql.JSONContains("metadata", `{"published":true}`))
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
- Extração e filtros JSON/JSONB com paths parametrizados e compatíveis com MySQL/PostgreSQL
- Combinação de consultas com `UNION`/`UNION ALL` e ordenação/paginação finais
- Subconsultas em FROM/JOIN recebem aliases automáticos (`subq_1`, `subq_2`, ...) quando omitidos
- Geração de SQL parametrizado com placeholders ajustados por dialeto (`?` para MySQL, `$1` para PostgreSQL)

## Compatibilidade
Escolha o dialeto com `WithDialect`, que alterna automaticamente os placeholders entre `?` (MySQL) e `$n` (PostgreSQL) enquanto mantém o rastreamento de argumentos.

Cláusulas de busca textual são específicas de dialeto: `MATCH ... AGAINST` só funciona com o dialeto MySQL e `TsVector`/`ts_rank` são exclusivos do dialeto PostgreSQL.

Para alternar o idioma/configuração no PostgreSQL, utilize `WithLanguage` (ou `WithConfig`) nos builders `TsVector`, que escapam os nomes de configuração automaticamente.

Caso deseje outro idioma padrão para novas buscas textuais em PostgreSQL, configure-o globalmente (com escapes aplicados na renderização):

```go
chizuql.SetDefaultTextSearchConfig("portuguese")

sql, _ := chizuql.New().
    WithDialect(chizuql.DialectPostgres).
    Select("id").
    From("posts").
    Where(chizuql.TsVector("title").WebSearch("busca segura")).
    Build()
// SELECT id FROM posts WHERE (to_tsvector('portuguese', title) @@ websearch_to_tsquery('portuguese', $1))
```

Subconsultas em `FROM`/`JOIN` recebem aliases gerados automaticamente (`subq_1`, `subq_2`, ...) quando omitidos, garantindo SQL válido em dialetos que exigem nomeação.

Para configurar um dialeto padrão global (sem perder a capacidade de sobrescrever por query), use:

```go
chizuql.SetDefaultDialect(chizuql.DialectPostgres)

sql, args := chizuql.New().Select("id").From("users").Where(chizuql.Col("id").Eq(10)).Build()
// SELECT id FROM users WHERE (id = $1) | args: [10]

sqlMySQL, argsMySQL := chizuql.New().WithDialect(chizuql.DialectMySQL).Select("id").From("users").Where(chizuql.Col("id").Eq(10)).Build()
// SELECT id FROM users WHERE (id = ?) | args: [10]
```

- Desenvolvido e testado em Go 1.25.

## Contribuindo e releases
- Execute sempre `go test ./...` e `golangci-lint run --fix ./...` (versão 2.6.2 ou superior) antes de abrir um PR ou publicar
  uma release.
- Para instalar o lint em outros ambientes ou CI, utilize o script oficial: `curl -sSfL https://raw.githubusercontent.com/golang
ci/golangci-lint/master/install.sh | sh -s -- -b /usr/local/bin v2.6.2` (mais opções em https://golangci-lint.run/docs/welcome
/install/#other-ci).
- Atualize o `CHANGELOG.md` seguindo o modelo do Keep a Changelog: registre alterações em "Unreleased" e mova-as para uma nova seção versionada (`vX.Y.Z`) quando criar uma tag.
- Adotamos versionamento semântico. Para novas releases, garanta que testes e lint passaram, que a documentação foi atualizada e que a tag `vX.Y.Z` foi criada.

## Roadmap
- [x] Converter placeholders para os formatos específicos de drivers (ex.: `$1` em PostgreSQL) automaticamente.
- [x] Adicionar suporte a `INSERT ... ON CONFLICT`/`UPSERT` com API fluente.
- [x] Expandir helpers de busca textual com ranking e ordenação por relevância.
- [x] Incluir suporte a aliases automáticos para subconsultas aninhadas.
- [x] Adicionar geração de SQL parametrizado para cláusulas `JSON`/`JSONB`.
- [x] Suportar construção de `UNION`/`UNION ALL` com controle de ordenação.
- [ ] Permitir `WITH ORDINALITY` em CTEs e funções set-returning no PostgreSQL.
- [ ] Introduzir API para window functions (`OVER`, partitions, frames).
- [ ] Oferecer builders para `GROUPING SETS`/`CUBE`/`ROLLUP`.
- [ ] Implementar suporte a `RETURNING` no MySQL 8.0+ (quando disponível) com fallback configurável.
- [ ] Adicionar helpers para `LOCK IN SHARE MODE`/`FOR UPDATE` conforme dialeto.
- [ ] Criar integração com contextos para cancelar build longo e medir métricas.
- [ ] Documentar exemplos de integração com ORMs (GORM, sqlc) e migrações.
- [ ] Suportar optimizer hints/hints de planner específicos por dialeto.
- [ ] Oferecer helpers para paginação por cursor (keyset pagination) na API fluente.
- [ ] Adicionar builders para `INTERSECT`/`EXCEPT` com ordenação e paginação em nível de conjunto.

## Licença
MIT
