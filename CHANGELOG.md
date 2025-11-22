# Changelog

All notable changes to this project will be documented in this file. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Added
- Suporte para configurar um dialeto SQL padrão global sem perder a possibilidade de trocá-lo por query.
- Configuração do `.golangci.yml` com linters obrigatórios e documentação de como rodá-los.
- Instruções de contribuição reforçando testes, lint e atualização do changelog.

### Changed
- Ajuste no gerenciamento de CTEs recursivas para emitir apenas um `WITH RECURSIVE` por declaração, evitando SQL inválido.
- `RETURNING` agora falha imediatamente em consultas `SELECT` e `UPDATE` sem cláusulas `SET`.
- Predicados `IN` passam a rejeitar listas vazias com panic explícito, inclusive quando construídos diretamente.
- Atualização das instruções de Go/lint para alinhar versões (Go 1.25, golangci-lint 2.6.2+ com `--fix`).

## [0.2.0] - 2024-06-26
### Added
- Alternância de dialetos MySQL/PostgreSQL com placeholders adequados via `WithDialect`.
- API de `INSERT` com `ON CONFLICT`/`ON DUPLICATE KEY UPDATE` e suporte a `DO NOTHING`.
- Builders de busca textual sensíveis ao dialeto.

## [0.1.0] - 2024-06-10
### Added
- Primeira versão do query builder com SELECT/INSERT/UPDATE/DELETE, CTEs, joins, subqueries e cláusulas `RETURNING`.
