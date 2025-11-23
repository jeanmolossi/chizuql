# Changelog

All notable changes to this project will be documented in this file. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Added
- Builders de ranking para buscas textuais: `Match.Score` no MySQL e `TsVector.RankWebSearch/RankPlainQuery` no PostgreSQL, incluindo ordenação por relevância e normalização opcional.
- Roadmap expandido com novos próximos passos para evolução do pacote.
- Alternância de idioma/configuração para buscas textuais PostgreSQL via `TsVector.WithLanguage` (compatível com `WithConfig`).
- Aliases automáticos para subconsultas em `FROM`/`JOIN`, garantindo SQL válido em dialetos que exigem nomeação.
- Helpers parametrizados para JSON/JSONB (`JSONExtract`, `JSONExtractText`, `JSONContains`) compatíveis com MySQL e PostgreSQL.
- Configuração global do idioma padrão para buscas textuais PostgreSQL via `SetDefaultTextSearchConfig`.
- Combinação de consultas com `UNION`/`UNION ALL` e ordenação/paginação finais.

### Changed
- Cláusulas de busca textual agora validam o dialeto selecionado, impedindo `MATCH ... AGAINST` em PostgreSQL e `TsVector` em MySQL.
- Documentação atualizada destacando compatibilidade de dialetos e os novos recursos de ranking.
- Páginação de cada SELECT em `UNION`/`UNION ALL` preservada por operando enquanto a ordenação permanece aplicada globalmente.

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
