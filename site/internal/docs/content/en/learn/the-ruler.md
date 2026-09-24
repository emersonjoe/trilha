---
title: The ruler — what a feature costs an agent
description: Trilha measures what an AI agent spends to build a feature, against pure Go, with frozen prompts and committed baselines. The numbers live on /custos.
---

Trilha makes a promise about agents: building a feature costs **less than half** of what the
same task costs in pure Go. A promise like that is only worth what its proof is worth, so the
proof is part of the repository — [the ruler](https://github.com/emersonjoe/trilha/tree/main/bench/agent)
(`bench/agent`) and the published series at **[/custos](/custos)**.

## What is measured

Eight scenarios, each a task an agent actually does — add an API route, a form, pagination, a
login; fix three planted defects. Every scenario is measured **on both sides**: as Trilha asks
it, and as the same task in Go puro (`net/http` + `html/template`), with the baseline committed
in the repository. The saving of a scenario is `1 − tokens(Trilha)/tokens(pure Go)`; the page
shows the average and each scenario, with rounds and token counts.

## The rules that keep it honest

- **Prompts are frozen** before the first measurement, one per side, with their SHA-256 pinned
  in `CHECKSUMS.txt`. Changing a prompt reopens the series.
- **Only measured numbers are published.** Medians of the runs that came out green, three runs
  per side; estimates say `est.` and this page carries none.
- **The saving is never stored** — it is derived from the two sides wherever it is read,
  including on [/custos](/custos).
- **Regression is a bug**: a release fails its gate when any scenario drops more than five
  points, any scenario falls under 60%, or the average misses the milestone.

The methodology, the per-scenario table and the current numbers are on
**[/custos](/custos)**; the chapter on [agentic development](/learn/agentic-cloud) is about
running agents on Trilha projects once the feature exists.

## Challenge

The series today is honest about being empty: the page says the first measurement is pending.
Read [the ruler's runner](https://github.com/emersonjoe/trilha/tree/main/bench/agent) and
answer without running anything: which of the two sides of a scenario should normally spend
more tokens, and what would it mean if the two sides ever came out equal?

:::solution
The baseline should spend more — that is the whole point of the ruler: the same task, asked in
a framework whose conventions, recipes and kit exist so the agent does not have to write the
scaffolding by hand. Equal spends would mean the framework added nothing the agent could reach:
neither a failure (the task would still be done) nor a saving, and the scenario would be
measuring Go against Go. That reading — equal is the alarm, not a tie — is why a scenario needs
a baseline at all, and why `bench-agent-verify` treats a regression as a bug instead of noise.
