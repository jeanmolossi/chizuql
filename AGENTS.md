# Agent Instructions

- Utilize Go na versão 1.25 para desenvolver, testar e validar o projeto.
- Mantenha este requisito explícito sempre que ajustar toolchains ou documentações relacionadas.
- Sempre execute `go test ./...` e `golangci-lint run --fix ./...` antes de abrir PRs ou concluir entregas; corrija eventuais alertas de lint. Utilize o `golangci-lint` na versão 2.6.2 ou superior (consulte o guia de instalação em https://golangci-lint.run/docs/welcome/install/#other-ci) e documente a execução.
- Atualize o `CHANGELOG.md` seguindo o formato Keep a Changelog, adicionando entradas em "Unreleased" e abrindo novas seções versionadas (tags `vX.Y.Z`) para cada release.
- Releases devem seguir versionamento semântico. Para lançar uma versão: prepare a entrada no changelog, atualize documentação pertinente, gere a tag `vX.Y.Z` e garanta que o lint e os testes passaram.
- Novas contribuições devem respeitar esta lista de checagens e apontar a versão/tag correspondente no changelog quando aplicável.
