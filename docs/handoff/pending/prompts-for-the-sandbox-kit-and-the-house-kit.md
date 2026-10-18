# Prompts for the sandbox kit and the house kit

Two prompts were drafted on 2026-10-16. The operator pastes them into the
sessions that own those kits. They are not yet merged into one final
version each.

- The sandbox kit:
  - The image ships mise 2026.9.15, while repositories pin 2026.10.3.
  - Subagent briefs now need a `SKILLS:` line, and a Go brief must name
    cc-skills-golang:golang-how-to and cc-skills-golang:golang-gopls.
  - After the mise swap, `mise trust` was needed in each checkout,
    because gh is a mise shim.
- The house kit:
  - Name the orchestrator "orchestrator-builder".
  - State that every builder, the orchestrator included, adversarially
    reviews its own work before the external review.
  - Decide where session-state notes live in a public repository.
  - Decided on 2026-10-17: in a public repository a CI job that failed
    for a cause outside the diff may be re-run without asking.
  - The house kit's `spec.yaml` ("Safety") needs that rule, only where
    the repository has no Actions secret and no environment with a
    reviewer or a secret: read the failing log first, re-run only a
    failure outside the diff, and record it so a repeat is fixed.
  - The auto mode classifier needs a permission rule for `gh run rerun`,
    which it blocked once as "CI Bypass".
  - This is the prompt to paste in the kit session:

```
Regra nova para o kit claude-home (decidida em 2026-10-17): re-run de CI em repositório público sem pedir.

Contexto: num repositório público (romeu-e-julieta), um job de CI falhou num teste instável que não tinha relação com o diff (só docs). O `gh run rerun <run> --failed` foi bloqueado pelo classificador do auto mode como "CI Bypass", e a regra atual de "Safety" não diz que o re-run é livre. O operador decidiu: em repositório público, o agente pode refazer um job que falhou sempre que precisar.

Pedido:

1. Em kits/claude-home/spec.yaml (a fonte de ~/.claude/CLAUDE.md), na seção "Safety", ao lado da frase que hoje exige pedir antes de "every push, workflow dispatch and re-run" em repositório com Actions secrets ou environment com reviewer/secret, acrescente uma regra em inglês, no estilo do arquivo, com este sentido:
   - In a repository that `gh repo view --json visibility` measures PUBLIC and that has no Actions secret and no environment with a reviewer or a secret, re-running a failed CI job (`gh run rerun <run> --failed`) needs no confirmation.
   - Before a re-run, read the failing log and state the cause; re-run only a failure outside the diff (a flaky test, a runner fault), and record it (a pending note or the Done record) so a repeat is fixed rather than re-run again.
   - A re-run never stands in for a fix of a failure the diff caused, and never weakens a check.
   A regra de secrets/environments continua valendo como está: lá, todo re-run ainda é perguntado.

2. Faça o classificador do auto mode aceitar isso: acrescente nas settings que o kit semeia (seed-settings.py ou equivalente) uma regra de permissão que libere `Bash(gh run rerun:*)`. A regra de permissão não sabe a visibilidade do repositório; quem limita a repositório público é a regra do item 1. Se achar que liberar em todo repositório é largo demais, proponha a alternativa e me pergunte antes.

3. Entregue no fluxo normal do kit (PR, revisão, CI verde) e me diga o que não foi verificado.
```

Recommended: ask the next session for the two final prompts in one
message.

Reopen: the operator asks for the prompts.
