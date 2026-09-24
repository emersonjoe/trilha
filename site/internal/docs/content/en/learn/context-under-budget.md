---
title: Context under budget
description: trilha ctx answers what the project has in one read, sliced for one job and priced in estimated tokens — so an agent stops opening files to find out.
---

An agent pays for every file it opens to find out what the project already has. Trilha
answers the question in one read: [`trilha ctx`](/reference/ctx) prints the map — routes, API
contracts, types, what setup provides, the recipes installed, the conventions the tree
actually uses — and prices itself at the foot, in estimated tokens.

    trilha ctx
    trilha ctx --json          # the same map, for a tool
    trilha ctx --pack login    # what the login recipe touches, and nothing else
    trilha ctx --pack app --budget 1500
    trilha ctx --pack app --budget 1500 --strict

## What a pack is

`--pack` slices the map for one job. `app` is the whole map; any other name is an installed
recipe's, and the slice keeps only the routes that recipe answers and the files it brought.
Each slice ends with a footer that prices the page — `tokens est.`, at four characters per
token, the same rate the [ruler](/learn/the-ruler) prices with. It is a budget, not a bill:
estimates say `est.`, and no release number is ever derived from them.

## The order things are cut

A budget cuts the map in the order information gets cheap — and never cuts routes, which are
the minimum useful:

1. **Contracts** — the API paths, what handlers bind and answer.
2. **Recipes** — what is installed and which files it brought.
3. **Conventions** — which conventions the tree actually uses.

What did not fit is named in the answer (`cut: contracts, recipes`), and with `--strict` a
cut is a failure: `E_CTX_BUDGET` lists what was left out and says to raise `--budget`. An
empty section is absence, not a cut — a project with no recipes pays nothing for the section.

## The same slice over MCP and llms.txt

An agent without a shell reads the same slices from the site: the docs MCP server answers
`get_context(pack)` with the pages one recipe touches, each priced, and `search_code(query)`
with `path:line` windows into the cookbook's Go sources — never a whole file. Every recipe
also has an `llms.txt` of its own, linked from the badge on the recipe's page, with the pack
priced and the page verbatim.

## AGENTS.md is held to a size

The file `trilha new` writes for agents has four fixed sections — the map first, the recipes
installed, the gates, the narrow-reading rules — and is tested to stay under 2,500 estimated
tokens. It points at `trilha ctx` instead of duplicating the map, so it cannot age the way a
copy would.

## Challenge

`trilha ctx --pack app --budget 10` still answers with routes and exits 0 without `--strict`.
Why do routes survive a budget that cuts everything else, and what would go wrong if a tight
budget could cut them too?

:::solution
The routes are the minimum useful answer — the one thing every other section exists to
explain. A map without the routes is not a smaller map; it is no answer at all, and an agent
holding it would open files anyway, which is the exact cost the command exists to avoid. That
is why the cut order has a floor: contracts, recipes and conventions can go, the routes
cannot — and `--strict` turns the cut into `E_CTX_BUDGET` so the caller decides between a
bigger budget and a partial answer, instead of receiving a silence it will pay to fill.
