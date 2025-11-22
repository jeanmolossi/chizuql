# ChizuQL

ChizuQL é um fluent query builder em Go, focado em clareza, previsibilidade e compatibilidade com múltiplos bancos de dados.  
Suporte inicial para **PostgreSQL** e **MySQL**, com uma DSL simples e composable para montar queries de forma segura e legível.

---

## 🚀 Bootstrap Rápido

### 1. Requisitos

- Go **1.25+**
- Postgres ou MySQL (opcional para começar, mas recomendado para testar de verdade)
- `git` instalado

### 2. Instalando o módulo

No seu projeto Go:

```bash
go get github.com/jeanmolossi/chizuql
```

### 3. Primeiro uso (SELECT básico)

Exemplo mínimo de uso com Postgres:

```go
package main

import (
    "database/sql"
    "fmt"
    "log"

    _ "github.com/lib/pq"

    "github.com/jeanmolossi/chizuql"
    "github.com/jeanmolossi/chizuql/dialect/postgres"
)

func main() {
    db, err := sql.Open("postgres", "postgres://user:pass@localhost:5432/dbname?sslmode=disable")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Cria um builder usando o dialeto do Postgres
    qb := chizuql.New(postgres.Dialect{})

    query := qb.
        Select("id", "name", "status").
        From("users").
        Where(
            chizuql.Col("status").Eq("active"),
        ).
        OrderBy("created_at DESC").
        Limit(20)

    sqlStr, args := query.Build()

    fmt.Println("SQL:", sqlStr)
    fmt.Println("Args:", args)

    rows, err := db.Query(sqlStr, args...)
    if err != nil {
        log.Fatal(err)
    }
    defer rows.Close()

    for rows.Next() {
        var id int64
        var name string
        var status string

        if err := rows.Scan(&id, &name, &status); err != nil {
            log.Fatal(err)
        }

        fmt.Println(id, name, status)
    }
}
```

Exemplo de saída:

```sql
SELECT id, name, status
FROM users
WHERE status = $1
ORDER BY created_at DESC
LIMIT 20;

```

### 4. Usando com MySQL

Exemplo mínimo com MySQL:

```go
import (
    "database/sql"

    _ "github.com/go-sql-driver/mysql"

    "github.com/jeanmolossi/chizuql"
    "github.com/jeanmolossi/chizuql/dialect/mysql"
)

func exampleMySQL(db *sql.DB) {
    qb := chizuql.New(mysql.Dialect{})

    query := qb.
        Select("id", "email").
        From("customers").
        Where(
            chizuql.Col("active").Eq(true),
        ).
        Limit(10)

    sqlStr, args := query.Build()

    // Em MySQL, os placeholders devem ser "?"
    // Ex.: SELECT id, email FROM customers WHERE active = ? LIMIT 10;
    rows, err := db.Query(sqlStr, args...)
    if err != nil {
        panic(err)
    }
    defer rows.Close()
}

```

## Conceitos principais

### Builder fluente

A API do ChizuQL é baseada em encadeamento de métodos, por exemplo:

```go
q := qb.
    Select("id", "name").
    From("users").
    Where(
        chizuql.And(
            chizuql.Col("status").Eq("active"),
            chizuql.Col("created_at").Gt(chizuql.Param("2024-01-01")),
        ),
    ).
    OrderBy("created_at DESC").
    Limit(50).
    Offset(100)

sqlStr, args := q.Build()
```

### Placeholders

- Postgres → $1, $2, $3, ...
- MySQL → ?

O dialeto é responsável por gerar o placeholder correto, para você não ter que ficar sofrendo com isso.

---

## 🏗 Estrutura sugerida do projeto

Estrutura inicial (pode ser ajustada depois):

```txt
chizuql/
├── go.mod
├── README.md
├── internal/
│   └── core/              # tipos internos, helpers, validações
├── dialect/
│   ├── postgres/          # implementação do dialeto Postgres
│   └── mysql/             # implementação do dialeto MySQL
├── builder/               # implementação dos builders (select, insert, etc)
├── examples/              # exemplos completos de uso
└── tests/                 # testes unitários e de integração
```

🧪 Rodando os testes

Após clonar o repositório:

```bash
git clone https://github.com/jeanmolossi/chizuql.git
cd chizuql

make tests
```

Se você tiver containers de banco para testes de integração, algo como:

```bash
make integration-tests
```

## 🧭 Roadmap inicial

- [ ] SELECT, INSERT, UPDATE, DELETE básicos
- [ ] Suporte completo a WHERE com AND/OR/IN/LIKE
- [ ] JOINs (INNER, LEFT, RIGHT)
- [ ] Subqueries
- [ ] Builder de CTEs (WITH ...)
- [ ] Modo “raw fragment” controlado (para escapes intencionais)
- [ ] Testes de integração com Postgres
- [ ] Testes de integração com MySQL
- [ ] Benchmarks de alocação e performance
