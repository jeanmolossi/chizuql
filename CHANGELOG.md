# Changelog

All notable changes to this project will be documented in this file. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Configuração do Renovate em `.github/renovate.json` para automatizar atualizações de dependências (Go modules e GitHub Actions) com painel dedicado.
- Workflows de qualidade, estabilidade e segurança (`code-quality.yml`, `stability.yml`, `security.yml`) executando lint, testes e varredura CodeQL com Go 1.25.
- Workflow de revisão de dependências (`vulnerability-scan.yml`) para checar vulnerabilidades em PRs utilizando `actions/dependency-review-action`.
- Hooks de build globais e por query (BeforeBuild/AfterBuild) com callbacks opcionais, métricas no `BuildReport` e exemplos de instrumentação no README e em `examples/`.
- Hooks especializados em OpenTelemetry (tracing e métricas) e logging (`log/slog`) disponíveis no pacote `hooks/`, com exemplos e cobertura de testes.

### Changed
- Instruções do `AGENTS.md` reforçadas para evitar downgrade das versões das actions utilizadas nas pipelines de CI.
- `AGENTS.md` agora orienta que qualquer alteração em `.golangci.yml` siga o schema oficial do `golangci-lint`.
- `AGENTS.md` detalha as chaves permitidas para `wsl_v5` e para a raiz de `issues` conforme o schema do golangci-lint.
- `BuildContext` agora retorna apenas SQL e args enquanto propaga o `context.Context` para os hooks; métricas permanecem disponíveis em `BuildResult` dentro dos callbacks.
- Hooks agora recebem o slice de argumentos original; `BuildResult.Args` foi documentado como somente leitura para deixar explícito que ele não deve ser modificado durante a instrumentação.

### Fixed
- Configuração do `.golangci.yml` realinhada com o schema oficial, mantendo `wsl_v5` e `issues` restritos às propriedades suportadas.
- Campo `version` em `.golangci.yml` ajustado para usar string conforme o schema do `golangci-lint`.
- Hook de tracing agora marca o span com o tempo real de renderização, evitando spans com duração zero.

## [v0.5.0] - 2025-11-23

### Added
- Helpers de paginação por cursor (`KeysetAfter`/`KeysetBefore`) reutilizando as ordenações configuradas na query.
- Métodos fluentes no builder para aplicar keyset pagination diretamente a partir do `ORDER BY`.
- Exemplo prático de paginação por cursor em `examples/queries.md`.
- Seção dedicada no README demonstrando o uso de keyset pagination.

### Changed
- Roadmap atualizado com a entrega de paginação por cursor e novo próximo passo para serialização de cursores.

### Fixed
- Corrigido `KeysetAfter/Before` para preservar a direção de ordenações fornecidas como string e evitar SQL inválido em paginação por cursor.

## [v0.4.0] - 2025-11-23

### Added
- Suporte a optimizer hints/planner hints com comentários `/*+ ... */` filtrados por dialeto e testes cobrindo MySQL/PostgreSQL.
- Documentação expandida com exemplos de integração (GORM, sqlc e migrações) e uso de hints no README e em `examples/`.

### Changed
- Nada por enquanto.

## [v0.3.0] - 2025-11-23
### Added
- Coletânea de consultas em `examples/` com código do query builder, saída gerada e notas de uso para MySQL e PostgreSQL.
- Builder para `WITH ORDINALITY` em funções set-returning do PostgreSQL via `WithOrdinality` e `FuncTable`.
- API de window functions (`Func(...).Over`, `Window`, frames `ROWS/RANGE`, `PartitionBy`/`OrderBy`) com novos helpers de limites de frame.
- Exemplos extras cobrindo window functions, `WITH ORDINALITY`, ordenação por colunas (`Asc`/`Desc`) e variações adicionais de consultas.
- Diretriz explícita no `AGENTS.md` exigindo que novas features venham acompanhadas de exemplos em `examples/`.
- Predicados `Between`/`NotBetween` e exemplos de uso no builder.
- Builders de agrupamento `GroupingSets`, `GroupSet`, `Rollup` e `Cube` para `GROUP BY` avançado.
- Configuração `WithMySQLReturningMode` com modo de fallback (`MySQLReturningOmit`) para omitir `RETURNING` em MySQL antigos.
- Exemplos adicionais cobrindo BETWEEN, grouping sets, rollup/cube e uso de `RETURNING` condicionado em MySQL.
- Helpers de lock de linha `ForUpdate` e `LockInShareMode`, adaptando a sintaxe conforme o dialeto.
- `BuildContext` para renderização cancelável com métricas de duração e contagem de argumentos, com exemplos dedicados.

### Changed
- Roadmap atualizado com tarefas concluídas (GROUPING SETS, RETURNING no MySQL, locks de linha e build com contexto) mantendo pendências abertas e próximas etapas claras.

## [v0.2.0] - 2025-11-23
### Added
- Builders de ranking para buscas textuais: `Match.Score` no MySQL e `TsVector.RankWebSearch/RankPlainQuery` no PostgreSQL, incluindo ordenação por relevância e normalização opcional.
- Roadmap expandido com novos próximos passos para evolução do pacote.
- Alternância de idioma/configuração para buscas textuais PostgreSQL via `TsVector.WithLanguage` (compatível com `WithConfig`).
- Aliases automáticos para subconsultas em `FROM`/`JOIN`, garantindo SQL válido em dialetos que exigem nomeação.
- Helpers parametrizados para JSON/JSONB (`JSONExtract`, `JSONExtractText`, `JSONContains`) compatíveis com MySQL e PostgreSQL.
- Configuração global do idioma padrão para buscas textuais PostgreSQL via `SetDefaultTextSearchConfig`.
- Combinação de consultas com `UNION`/`UNION ALL` e ordenação/paginação finais.
- Guia rápido para instalar o `golangci-lint` (>= 2.6.2) com Go 1.25+, incluindo validação de PATH e versão.

### Changed
- Cláusulas de busca textual agora validam o dialeto selecionado, impedindo `MATCH ... AGAINST` em PostgreSQL e `TsVector` em MySQL.
- Documentação atualizada destacando compatibilidade de dialetos e os novos recursos de ranking.
- Páginação de cada SELECT em `UNION`/`UNION ALL` preservada por operando enquanto a ordenação permanece aplicada globalmente.
- Configuração do lint restaurada para `wsl_v5` com ajustes de conveniência e orientações reforçadas sobre versões fixas de Go e golangci-lint.

## [v0.1.0] - 2024-06-27
### Added
- Primeira versão do query builder com SELECT/INSERT/UPDATE/DELETE, CTEs, joins, subqueries e cláusulas `RETURNING`.
- Alternância de dialetos MySQL/PostgreSQL com placeholders adequados via `WithDialect` e configuração global de dialeto padrão.
- Suporte para `INSERT` com `ON CONFLICT`/`ON DUPLICATE KEY UPDATE`, `DO NOTHING` e buscas textuais sensíveis ao dialeto.
- Configuração do `.golangci.yml` com linters obrigatórios, documentação de execução (`--fix`), e instruções de contribuição enfatizando testes, lint e atualização do changelog.

### Changed
- Ajuste no gerenciamento de CTEs recursivas para emitir apenas um `WITH RECURSIVE` por declaração, evitando SQL inválido.
- `RETURNING` agora falha imediatamente em consultas `SELECT` e `UPDATE` sem cláusulas `SET`.
- Predicados `IN` passam a rejeitar listas vazias com panic explícito, inclusive quando construídos diretamente.
- Atualização das instruções de Go/lint para alinhar versões (Go 1.25, golangci-lint 2.6.2+ com `--fix`).
