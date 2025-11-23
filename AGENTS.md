# Agent Instructions

- Utilize Go na versão 1.25 para desenvolver, testar e validar o projeto.
- Mantenha este requisito explícito sempre que ajustar toolchains ou documentações relacionadas.
- Utilize o `golangci-lint` na versão 2.6.2 ou superior (consulte o guia de instalação em https://golangci-lint.run/docs/welcome/install/#other-ci) e documente a execução.
- Confirme explicitamente as versões de Go (1.25) e do `golangci-lint` (2.6.2+) antes de iniciar qualquer alteração e não altere versões ou toolchains a menos que seja solicitado de forma explícita.
- Sempre execute `go test ./...` e `golangci-lint run --fix ./...` antes de abrir PRs ou concluir entregas; corrija eventuais alertas de lint. O lint deve ser executado apenas quando houver alterações em arquivos `*.go` no diff final.
- Atualize o `CHANGELOG.md` seguindo o formato Keep a Changelog, adicionando entradas em "Unreleased" e abrindo novas seções versionadas (tags `vX.Y.Z`) para cada release.
- Releases devem seguir versionamento semântico. Para lançar uma versão: prepare a entrada no changelog, atualize documentação pertinente, gere a tag `vX.Y.Z` e garanta que o lint e os testes passaram.
- Novas contribuições devem respeitar esta lista de checagens e apontar a versão/tag correspondente no changelog quando aplicável.
- Ao concluir tarefas do roadmap, verifique quais itens foram marcados como feitos e acrescente novos próximos passos para manter o roadmap sempre com pendências atualizadas.
