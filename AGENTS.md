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
- Toda nova feature deve vir acompanhada de um exemplo em `examples/` demonstrando o uso da API adicionada ou estendida.
- As pipelines de CI devem manter, no mínimo, as versões atuais das actions utilizadas: `actions/checkout@v6.0.0`, `actions/setup-go@v6.1.0`, `actions/cache@v4.3.0`, `golangci/golangci-lint-action@v9.1.0`, `github/codeql-action/*@v4.31.4`, `actions/upload-artifact@v5.0.0` e `actions/download-artifact@v6.0.0`. Não faça downgrade; ao atualizar qualquer versão, ajuste esta instrução para que o novo número passe a ser o mínimo aceito.
