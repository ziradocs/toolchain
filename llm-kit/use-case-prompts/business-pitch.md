# Use Case: Business / Startup Pitch

Narrative arc: **Problem → Solution → Traction → Business Model →
Financials → Ask**. Charts should be metrics-driven: ROI, growth
percentages, market size, unit economics. Tone: action-oriented, confident,
grounded in numbers rather than adjectives.

Both skeletons below are flex mode — every slide is just `title`/`content`
type (flex has no per-slide layout typing; the section breaks below are
plain `---` separators, not a `layout:` block — see `reference/slidelang-flex.md`).

## Skeleton (elevator pitch, ~5 slides)

```
---
mode: flex
title: "[Company/Project Name]"
---
# [Company/Project Name]
## [One-line value proposition]
### [Your name, title, date]

---
# The Problem
*[Hook with a compelling statistic or story]*

---
# Our Solution

::: success
**Value Proposition**: [Clear, measurable benefit]
:::

[Brief explanation with a visual metaphor]

---
# Market Opportunity

<<chart: bar>>
  data: [
    ["Current Market", 0],
    ["Projected", 0]
  ]
  series: ["Market Size ($B)"]

---
# Traction & Results

| Metric | Current | Growth |
|--------|---------|--------|
| **Users** | [Number] | +[%] |
| **Revenue** | $[Amount] | +[%] |

---
# The Ask

::: tip
**Next Steps**: [Specific request with timeline]
:::

**Contact**: [email] | [phone]
```

## Skeleton (board update, metrics-forward)

```
---
mode: flex
title: "Q[X] Board Update"
---
# Q[X] Board Update
## [Company Name] Performance Review
### [Date] | [Presenter Name, Title]

---
# Performance Dashboard

<<chart: combo>>
  data: [
    ["Q1", 0, 0],
    ["Q2", 0, 0],
    ["Q3", 0, 0],
    ["Q4", 0, 0]
  ]
  series: ["Actual Revenue (M)", "Target (M)"]

---
# Strategic Initiatives Update

::: success
### Completed
- [Initiative 1] — [Impact/Result]
:::

::: warning
### In Progress
- [Initiative 3] — [Status, completion date]
:::

---
# Next Quarter Focus

::: tip
**Priorities**:
1. [Priority 1 with success metric]
:::

**Board Decision Required**: [Specific ask if any]
```

## Worked example

`examples/gallery/10_startup_pitch_deck.slidelang` is a full worked pitch
deck — read it end-to-end for pacing and layout variety (avoid repeating
the same layout more than ~2 slides in a row).

## Sample audience-calibration questions

- "Who's the audience — investors (Series A/B), a board, or internal
  stakeholders?"
- "What's the single most important number in this business right now?"
- "How much time do you have to present?"
