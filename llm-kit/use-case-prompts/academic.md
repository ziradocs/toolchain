# Use Case: Academic / Research Presentations

Narrative arc: **Question → Literature → Methodology → Results →
Discussion**. Charts should include realistic sample sizes, and where
relevant, statistical framing (p-values, confidence intervals) as prose or
callouts — ZiraDocs has no native statistical element, so express these
as text or `::: info`/`::: success` blocks.

Tone: clear, cited, logical flow. Avoid marketing language; prefer
precision over hype.

The skeleton below is flex mode — every slide is just `title`/`content`
type (flex has no per-slide layout typing; the section breaks below are
plain `---` separators, not a `layout:` block — see `reference/slidelang-flex.md`).

## Skeleton

```
---
mode: flex
title: "[Research Title]"
---
# [Research Title]
## [Subtitle if needed]
### [Your Name], [Institution] | [Conference], [Date]

---
# Research Context
*Understanding [problem domain]*

---
# Literature Gap

::: info
**Current State**: [What we know]

**Gap**: [What's missing]

**Our Contribution**: [How this research fills the gap]
:::

---
# Research Questions

::: tip
**Primary Question**: [Main research question]

**Hypotheses**:
- H1: [Hypothesis 1]
- H2: [Hypothesis 2]
:::

---
# Methodology

<<mermaid>>
  graph LR
      A[Data Collection] --> B[Analysis]
      B --> C[Validation]
      C --> D[Results]

**Sample**: [Size and characteristics]
**Timeline**: [Duration]

---
# Key Findings

<<chart: combo>>
  data: [
    ["Condition A", 0, 0],
    ["Condition B", 0, 0]
  ]
  series: ["Measure 1", "Measure 2"]

::: success
**Statistical Significance**: p < [value] for all primary measures
:::

---
# Discussion

**Key Insights**:
1. [Finding 1] — [Implication]
2. [Finding 2] — [Implication]

**Limitations**:
- [Limitation 1]
- [Limitation 2]

---
# Future Directions

::: tip
**Next Steps**:
- [Research direction 1]
- [Research direction 2]
:::

**Contact**: [email] | **Paper**: [DOI or URL]
```

## Worked example

`examples/gallery/03_academic_paper.doclang` is a full DocLang worked
example (report-style academic writeup) with realistic pacing and data
framing — read it end-to-end. There isn't a dedicated academic ZiraDocs
example in `examples/gallery/` yet; use the skeleton above and the general
ZiraDocs gallery examples (charts, mermaid, tables) for element syntax.

## Sample audience-calibration questions

- "Who is the audience — specialists in this subfield, a broader academic
  audience, or a general/public audience?"
- "Is this a conference talk (10–15 min), a defense, or a seminar (45+
  min)?"
- "Do you have real data, or should I use realistic placeholder figures for
  now?"
