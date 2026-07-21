# Use Case: Creative / Personal

Narrative arc: **Hook → Journey → Showcase → Insights → Connect**. Tone:
authentic, emotional, storytelling-focused. Favor `> quote` blocks,
timeline-style pacing, and images over dense tables/charts.

All patterns and the skeleton below are flex mode — every slide is just
`title`/`content` type (flex has no per-slide layout typing; section
breaks are plain `---` separators, not a `layout:` block — see
`reference/slidelang-flex.md`).

## Storytelling patterns

### Problem → solution flow

```
# Slide 1: Hook with a dramatic problem
::: danger
**Crisis Point**: [Compelling statistic or story]
:::

---
# Slide 2: Problem amplification
[Supporting context]

---
# Slide 3: Current approaches falling short
[Why existing approaches don't work]

---
# Slide 4: Solution introduction
::: success
**The Breakthrough**: [Clear solution statement]
:::

---
# Slide 5: Proof
[Data, case studies, validation]

---
# Slide 6: Call to action
[What happens next]
```

### Before → after transformation

```
# Slide 1: Current state
::: warning
**Current Reality**: [Description of the problematic status quo]
:::

---
# Slide 2: Pain points

<<chart: radar>>
  data: [
    ["Efficiency", 30],
    ["Cost", 40],
    ["Quality", 50]
  ]
  series: ["Current State"]

---
# Slide 3: Vision of the future
::: success
**Transformed Reality**: [Description of the improved future]
:::

---
# Slide 4: The improvement, visualized

<<chart: radar>>
  data: [
    ["Efficiency", 30, 85],
    ["Cost", 40, 75],
    ["Quality", 50, 90]
  ]
  series: ["Current State", "Future State"]

---
# Slide 5: The bridge
[How to get from current to future state]
```

## Skeleton (personal narrative)

```
---
mode: flex
title: "[Story Title]"
---
# [Story Title]
## [One-line framing]
### [Your name] | [Date/context]

---
> "[An opening quote that sets the tone]"
>
> **— [attribution, if any]**

---
# The Journey Begins
*[Where it started]*

---
# [A turning point]

[Narrative text — favor short paragraphs over bullet lists here]

---
# What I Learned

::: tip
1. [Insight 1]
2. [Insight 2]
:::

---
# Let's Connect

**Contact**: [email / social]
```

## Worked example

`examples/gallery/06_maps_and_geodata.slidelang` (for a travel/journey
narrative using the map element) and
`examples/gallery/02_flex_mode_essentials.slidelang` (for the `> quote`
pattern) are good references for tone and pacing in this category.

## Sample audience-calibration questions

- "Is this for a personal audience (friends/family), a portfolio review,
  or a public talk?"
- "Do you have real photos/images to reference, or should placeholders
  describe the intended visual?"
