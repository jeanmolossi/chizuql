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
> Observação: os exemplos de PostgreSQL abaixo assumem que `chizuql.SetDefaultDialect(chizuql.DialectPostgres)` foi chamado previamente, pois o dialeto PostgreSQL é necessário.
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

### Filtros com BETWEEN/NOT BETWEEN
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

### Locks de linha com `FOR UPDATE`/`LOCK IN SHARE MODE`
```go
lockShared := chizuql.New().
    Select("id", "email").
    From("users").
    Where(chizuql.Col("status").Eq("pending")).
    OrderBy("id").
    Limit(10).
    LockInShareMode()

lockUpdate := chizuql.New().
    WithDialect(chizuql.DialectPostgres).
    Select("id").
    From("jobs").
    Where(chizuql.Col("locked_at").IsNull()).
    ForUpdate()
```

- `LockInShareMode` adapta a sintaxe ao dialeto: MySQL recebe `LOCK IN SHARE MODE`; PostgreSQL usa `FOR SHARE`.
- `ForUpdate` sempre gera `FOR UPDATE`, útil para filas e workers que precisam impedir leitura concorrente enquanto processam.

### Agrupamentos avançados
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
    OrderBy("total DESC")

sql, args := q.Build()
```

### RETURNING no MySQL com fallback configurável
```go
// Renderiza RETURNING (MySQL 8.0+) ou omite a cláusula para versões antigas
update := chizuql.New().
    WithDialect(chizuql.DialectMySQL).
    WithMySQLReturningMode(chizuql.MySQLReturningOmit). // troque para MySQLReturningStrict em servidores 8.0+
    Update("users").
    Set(chizuql.Set("name", "Ana")).
    Where(chizuql.Col("id").Eq(1)).
    Returning("id")

sql, args := update.Build()
```

### Build com `context.Context` e métricas
```go
ctx := context.Background()

q := chizuql.New().
    Select("id", "name").
    From("users").
    Where(chizuql.Col("deleted_at").IsNull())

sql, args, report, err := q.BuildContext(ctx)
if err != nil {
    log.Fatal(err)
}

fmt.Println("dialeto:", report.DialectKind)
fmt.Println("args:", args)
fmt.Println("latência de build:", report.RenderDuration)
```

- `BuildContext` aceita cancelamento por contexto (útil em builds longos) e retorna métricas de renderização.
- O método `Build` original continua disponível para uso rápido sem contexto.

## Recursos principais
- SELECT, INSERT, UPDATE, DELETE com cláusulas fluentes
- JOINs, subqueries e CTEs (`WITH` e `WITH RECURSIVE`)
- Predicados compostos (`AND`/`OR`), `IN`, `BETWEEN`, `LIKE`, `IS NULL`
- Comparação entre colunas e uso de expressões cruas com `Raw`
- Suporte a `RETURNING`
- Agrupamentos avançados (`GROUPING SETS`, `ROLLUP`, `CUBE`) e window functions com frames
- Builders de busca textual para MySQL (`MATCH ... AGAINST`) e PostgreSQL (`to_tsvector` + `websearch_to_tsquery` ou `plainto_tsquery`)
- Extração e filtros JSON/JSONB com paths parametrizados e compatíveis com MySQL/PostgreSQL
- Combinação de consultas com `UNION`/`UNION ALL` e ordenação/paginação finais
- Subconsultas em FROM/JOIN recebem aliases automáticos (`subq_1`, `subq_2`, ...) quando omitidos
- Geração de SQL parametrizado com placeholders ajustados por dialeto (`?` para MySQL, `$1` para PostgreSQL)

## Compatibilidade
Escolha o dialeto com `WithDialect`, que alterna automaticamente os placeholders entre `?` (MySQL) e `$n` (PostgreSQL) enquanto mantém o rastreamento de argumentos.

Em servidores MySQL anteriores à 8.0, utilize `WithMySQLReturningMode(MySQLReturningOmit)` para suprimir `RETURNING` em consultas DML; o modo padrão (`MySQLReturningStrict`) mantém a cláusula para ambientes que já suportam o recurso.

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

## Hints de otimizador específicos por dialeto
Use `OptimizerHints` para injetar comentários `/*+ ... */` logo após o verbo (SELECT/INSERT/UPDATE/DELETE), filtrando por dialeto quando necessário:

```go
q := chizuql.New().
    Select("id").
    From("users").
    OptimizerHints(
        chizuql.OptimizerHint("MAX_EXECUTION_TIME(500)"), // aplicado a todos os dialetos
        chizuql.MySQLHint("INDEX(users idx_users_status)"), // somente MySQL
        chizuql.PostgresHint("SeqScan(users) OFF"),         // somente PostgreSQL
    )

sql, _ := q.Build()
// SELECT /*+ MAX_EXECUTION_TIME(500) INDEX(users idx_users_status) */ id FROM users

sqlPg, _ := q.WithDialect(chizuql.DialectPostgres).Build()
// SELECT /*+ SeqScan(users) OFF */ id FROM users
```

### Paginação por cursor (keyset)
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
// SELECT id, created_at FROM posts WHERE ((id > ? OR (id = ? AND created_at < ?))) ORDER BY id ASC, created_at DESC LIMIT 20
// args: [120 120 "2025-01-01 00:00:00"]
```

Use `KeysetBefore` com os mesmos campos de ordenação para navegar para a página anterior; a direção do comparador é invertida
automaticamente para ordenações `DESC`.

## Integração com ORMs (GORM, sqlc) e migrações
### GORM
```go
type User struct {
    ID   int64
    Name string
}

q := chizuql.New().
    Select("id", "name").
    From("users").
    Where(chizuql.Col("status").Eq("active")).
    OptimizerHints(chizuql.MySQLHint("INDEX(users idx_users_status)"))

sqlStr, args := q.Build()

var users []User
if err := db.Raw(sqlStr, args...).Scan(&users).Error; err != nil {
    return err
}
```
- `OptimizerHints` funciona com o dialeto configurado na conexão do GORM; os argumentos retornados pelo builder são repassados diretamente ao método `Raw`.

### sqlc
```go
ctx := context.Background()
q := chizuql.New().
    WithDialect(chizuql.DialectPostgres).
    Select("id", "email").
    From("users").
    Where(chizuql.Col("id").In(1, 2, 3))

sqlStr, args := q.Build()
rows, err := db.QueryContext(ctx, sqlStr, args...)
if err != nil {
    return err
}
defer rows.Close()

for rows.Next() {
    var id int64
    var email string
    if err := rows.Scan(&id, &email); err != nil {
        return err
    }
    // envie os dados para seu código gerado pelo sqlc
}
```
- Utilize `WithDialect` para alinhar os placeholders com o banco configurado no sqlc; o slice de `args` se encaixa diretamente nos métodos `QueryContext`/`ExecContext`.

### Migrações
```go
// Exemplo de migração com goose/hcl/atlas usando uma query gerada
migration := chizuql.New().
    WithDialect(chizuql.DialectPostgres).
    RawQuery("ALTER TABLE users ADD COLUMN metadata JSONB DEFAULT '{}'::jsonb")

sqlStr, args := migration.Build()
if len(args) != 0 {
    return fmt.Errorf("migration expected zero args, got %d", len(args))
}

// Execute via o driver padrão usado pelo seu runner de migração
if _, err := db.ExecContext(ctx, sqlStr); err != nil {
    return err
}
```
- Para migrações declarativas que exigem SQL estático, `RawQuery` ajuda a compartilhar snippets consistentes (inclusive para rollback) mantendo a mesma estratégia de placeholders.

## Hooks de build para métricas e logs

- Use `BuildHook` para instrumentar o processo de renderização com callbacks `BeforeBuild`/`AfterBuild`. Registre hooks globais com `RegisterBuildHooks` ou restritos à query com `WithHooks`. Erros retornados pelos hooks são ignorados para que a geração de SQL não seja interrompida.

```go
var builds []time.Duration

// Hook global com funções inline; também é possível implementar a interface BuildHook.
chizuql.RegisterBuildHooks(chizuql.BuildHookFuncs{
    After: func(ctx context.Context, result chizuql.BuildResult) error {
        builds = append(builds, result.Report.RenderDuration)

        log.Printf("sql=%s args=%v duration=%s placeholders=%d", result.SQL, result.Args, result.Report.RenderDuration, result.Report.ArgsCount)

        return nil
    },
})

sql, args, report, err := chizuql.New().
    WithHooks(chizuql.BuildHookFuncs{ // hook específico desta query
        After: func(ctx context.Context, result chizuql.BuildResult) error {
            fmt.Printf("captured %d placeholders in %s\n", result.Report.ArgsCount, result.Report.RenderDuration)

            return nil
        },
    }).
    Select("id", "email").
    From("users").
    Where(chizuql.Col("status").Eq("active")).
    BuildContext(context.Background())

if err != nil {
    return err
}

fmt.Println(sql, args, report.RenderDuration)
```

- Desenvolvido e testado em Go 1.25.

## Contribuindo e releases
- Verifique o ambiente antes de qualquer alteração: `go version` deve reportar Go 1.25.x e `golangci-lint --version` deve apontar para a versão 2.6.2 ou superior no PATH em uso.
- Utilize Go 1.25 e não altere a toolchain ou versões de linting salvo solicitação explícita.
- Execute sempre `go test ./...` e, quando houver alterações em arquivos `*.go`, rode `golangci-lint run --fix ./...` (versão 2.6.2 ou superior) antes de abrir um PR ou publicar uma release.
- Atualize o `CHANGELOG.md` seguindo o modelo do Keep a Changelog: registre alterações em "Unreleased" e mova-as para uma nova seção versionada (`vX.Y.Z`) quando criar uma tag.
- Adotamos versionamento semântico. Para novas releases, garanta que testes e lint passaram, que a documentação foi atualizada e que a tag `vX.Y.Z` foi criada.

### Instalação rápida do golangci-lint (>= 2.6.2) com Go 1.25+
1. Confirme o Go ativo com `go version` (saída deve ser 1.25.x).
2. Instale/atualize o lint com o script oficial: `curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /usr/local/bin v2.6.2`.
3. Garanta que o binário correto está no PATH (`which golangci-lint`) e valide a versão com `/usr/local/bin/golangci-lint --version`.
4. Em CI, reutilize o binário já baixado ou armazene-o em cache para acelerar execuções subsequentes.

## Roadmap
- [x] Converter placeholders para os formatos específicos de drivers (ex.: `$1` em PostgreSQL) automaticamente.
- [x] Adicionar suporte a `INSERT ... ON CONFLICT`/`UPSERT` com API fluente.
- [x] Expandir helpers de busca textual com ranking e ordenação por relevância.
- [x] Incluir suporte a aliases automáticos para subconsultas aninhadas.
- [x] Adicionar geração de SQL parametrizado para cláusulas `JSON`/`JSONB`.
- [x] Suportar construção de `UNION`/`UNION ALL` com controle de ordenação.
- [x] Permitir `WITH ORDINALITY` em CTEs e funções set-returning no PostgreSQL.
- [x] Introduzir API para window functions (`OVER`, partitions, frames).
- [x] Oferecer builders para `GROUPING SETS`/`CUBE`/`ROLLUP`.
- [x] Implementar suporte a `RETURNING` no MySQL 8.0+ (quando disponível) com fallback configurável.
- [x] Adicionar helpers para `LOCK IN SHARE MODE`/`FOR UPDATE` conforme dialeto.
- [x] Criar integração com contextos para cancelar build longo e medir métricas.
- [x] Documentar exemplos de integração com ORMs (GORM, sqlc) e migrações.
- [x] Suportar optimizer hints/hints de planner específicos por dialeto.
- [x] Oferecer helpers para paginação por cursor (keyset pagination) na API fluente.
- [ ] Adicionar builders para `INTERSECT`/`EXCEPT` com ordenação e paginação em nível de conjunto.
- [ ] Expor builders para `LATERAL JOIN`/`CROSS APPLY` onde suportados.
- [ ] Oferecer API para `MERGE`/`INSERT ... ON DUPLICATE KEY` com estratégias portáveis.
- [ ] Serializar/deserializar cursores de paginação (token seguro) para facilitar APIs públicas.

## Licença
MIT
