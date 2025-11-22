# Agent Instructions

- Utilize Go na versão ^1.25.x para desenvolver, testar e validar o projeto.
- Mantenha este requisito explícito sempre que ajustar toolchains ou documentações relacionadas.
- Sempre execute `go test ./...` e `golangci-lint run ./...` antes de abrir PRs ou concluir entregas; corrija eventuais alertas de lint.
- Atualize o `CHANGELOG.md` seguindo o formato Keep a Changelog, adicionando entradas em "Unreleased" e abrindo novas seções versionadas (tags `vX.Y.Z`) para cada release.
- Releases devem seguir versionamento semântico. Para lançar uma versão: prepare a entrada no changelog, atualize documentação pertinente, gere a tag `vX.Y.Z` e garanta que o lint e os testes passaram.
- Novas contribuições devem respeitar esta lista de checagens e apontar a versão/tag correspondente no changelog quando aplicável.
