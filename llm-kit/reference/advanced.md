# Advanced Elements — Charts, Mermaid, Maps, Layouts

Deeper worked examples beyond the quick reference in `elements.md`.
Everything here works in both ZiraDocs flex and DocLang; charts, mermaid,
maps, and layouts also work in ZiraDocs strict (quote/checklist/grid do
not — see `elements.md`'s quick-table note), unless noted otherwise.

## Chart types, worked

### Combo (bars + line, dual metric)

```
<<chart: combo>>
  data: [
    ["Q1", 250000, 0],
    ["Q2", 320000, 28],
    ["Q3", 410000, 28],
    ["Q4", 520000, 27]
  ]
  series: ["Revenue", "Growth Rate %"]
  options:
    responsive: true
```

### Scatter

```
<<chart: scatter>>
  data: [
    [1, 65, 60],
    [2, 70, 68],
    [3, 75, 74]
  ]
  series: ["Team A", "Team B"]
```

### Radar

```
<<chart: radar>>
  data: [
    [8, 7, 6, 9, 7],
    [9, 8, 8, 9, 9]
  ]
  labels: ["Communication", "Technical", "Leadership", "Creativity", "Analysis"]
  series: ["Current Level", "Target Level"]
```

### Stacked bar

```
<<chart: bar>>
  data: [
    ["North", 25, 15, 10],
    ["South", 30, 20, 12]
  ]
  series: ["Product A", "Product B", "Product C"]
  options:
    stacked: true
```

Data-quality guidance (not enforced by the linter — your responsibility):
keep column count consistent across rows, keep `series` length equal to
the numeric column count, keep values realistic and units consistent
(don't mix `%` and raw counts in the same series).

## Mermaid, worked

### Sequence diagram

```
<<mermaid>>
  sequenceDiagram
      participant User
      participant API
      User->>API: Login Request
      API-->>User: Auth Token
```

### Gantt

```
<<mermaid>>
  gantt
      title Project Timeline
      dateFormat YYYY-MM-DD
      section Planning
      Research & Analysis    :a1, 2024-01-01, 30d
      section Development
      MVP Development       :2024-03-01, 45d
```

### Mindmap

```
<<mermaid>>
  mindmap
    root((Strategy))
      Area A
        Sub A1
        Sub A2
      Area B
        Sub B1
```

### System architecture (subgraphs)

```
<<mermaid>>
  graph TB
    subgraph "Frontend"
      A[Web App] --> B[Mobile App]
    end
    subgraph "Backend"
      C[API] --> D[Database]
    end
    A --> C
    B --> C
```

### State diagram

```
<<mermaid>>
  stateDiagram-v2
      [*] --> Draft
      Draft --> Review : submit
      Review --> Approved : approve
      Review --> Draft : reject
      Approved --> [*]
```

All of these render **client-side** in the browser (mermaid.js) — there is
no server-side or build-time diagram rendering in the default browser
render path.

## Layout typing (ZiraDocs only — strict mode only)

Only **strict mode** can set a slide's layout type, and only via
`SLIDE <type>` (see `slidelang-strict.md`):

```
SLIDE comparison
  title: "Solution Comparison"
  ::: info
  ### Traditional Approach
  - High costs
  - Long timeline
  :::
  ::: success
  ### Our Approach
  - Cost-effective
  - Rapid deployment
  :::
```

The linter then validates the slide against that layout's schema (see
`../validation-checklist.md` for the full table of required properties,
allowed/forbidden elements, and min/max element counts per layout).

**A `---\nlayout: <type>\n---` mini-frontmatter block does *not* work in
flex mode — don't use it.** `FlexParser.parseContentBlock`
(`parser/flex.go`) treats any `---`-delimited block containing only simple
`key: value` lines as inert metadata and discards it without reading
`layout` at all (the parser's own comment names this "no parser support
today, tracked separately" — see also `slidelang-flex.md`'s "Structure
rules"). The heading that follows is parsed as an ordinary `title`/`content`
slide; the `layout:` line has zero effect. If you need a specific layout's
schema validation (comparison/stats/timeline/etc.) in an LLM-generated
deck, use strict mode's `SLIDE <type>` — flex mode has no equivalent today.

DocLang has no layout system either — sections are just Markdown headings,
no `layout:` frontmatter block applies there.

## Color palettes (presentation-only convention, not enforced)

These are conventions worth following for visual consistency; the linter
does not check colors at all.

```yaml
# Corporate/Business
colors: ["#2C3E50", "#3498DB", "#E74C3C", "#27AE60", "#F39C12"]

# Academic/Research
colors: ["#34495E", "#3498DB", "#9B59B6", "#1ABC9C", "#E67E22"]

# Creative/Design
colors: ["#FF6B6B", "#4ECDC4", "#45B7D1", "#96CEB4", "#FFEAA7"]
```

## Realistic data ranges (for believable demo/example content)

```yaml
# Revenue growth (annual), by company stage
startup: 50-200% year-over-year
small_business: 10-25% year-over-year
enterprise: 3-15% year-over-year

# Customer satisfaction
poor: 60-75%
average: 75-85%
excellent: 85-95%

# Academic sample sizes
survey_research: 100-300 participants
experimental: 30-50 per condition
```
