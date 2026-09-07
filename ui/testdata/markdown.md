# Executive summary

The contract is **valid** and *renewable*, with one clause that
needs attention. See the [full text](https://example.com/doc?a=1&b=2)
or write to [us](mailto:legal@example.com).

## Findings

- Clause 4.2 fixes the price for `24 months`
- Clause 7 allows termination
  - with 30 days' notice
  - in writing
- Clause 9 is ~~void~~ superseded

1. Read clause 4.2
2. Compare with the previous version
3. Decide

> The party may terminate at any time.
> Notice must be in writing.

| Clause | Risk | Note |
|---|---|---|
| 4.2 | low | fixed price |
| 7 | medium | 30 days |

Here is the check:

```go
if c.Query("ver") != "" {
	return c.Inline(a.Nome, corpo, a.Tipo)
}
```

And an inline snippet: `ui.Markdown(text, ui.MarkdownOpts{})`.

A hard break comes next.  
This line is after it.

---

Anything else is text: <script>alert(1)</script> and <b>bold</b>.
