# Use Case: Educational / Training

Narrative arc: **Objectives → Warm-up → Core Concepts → Practice →
Summary**. Tone: progressive complexity, interactive framing — but see the
important caveat below about what "interactive" can actually mean in
ZiraDocs output.

## Interactive elements: placeholder only, never invented tags

ZiraDocs has **no** poll, quiz, or presenter-notes element. If a training
deck calls for audience interaction, represent it as descriptive plain text
— never emit `<<poll>>`, `<<quiz>>`, `:::poll`, `:::qa_session`, `:::reveal`,
or a `:::notes` block. These are not implemented by the parser; at best
they're silently dropped, at worst they produce a linter warning
(`SPECIAL001` for an unrecognized special-block type).

```
# Pre-Session Check

**Interactive Poll (placeholder)**: Ask "What's your experience with data
visualization?" Options: beginner | some Excel | some Python/R | regular
creator. Gather a show of hands — no poll tag is emitted.
```

```
# Knowledge Check (placeholder)

**Question**: Which chart type best shows parts of a whole?
**Expected answer**: Pie chart. Discuss reasoning with the group.
```

## Skeleton

The skeleton below is flex mode — every slide is just `title`/`content`
type (flex has no per-slide layout typing; section breaks are plain `---`
separators, not a `layout:` block — see `reference/slidelang-flex.md`).

```
---
mode: flex
title: "[Course/Module Title]"
---
# [Course/Module Title]
## Module [N]: [Topic]
### Duration: [X] minutes | Instructor: [Name]

---
# Learning Objectives

By the end of this session, you will be able to:

✅ **Understand** [concept 1]
✅ **Create** [skill 1]
✅ **Evaluate** [judgment 1]

---
# Why This Matters
*[Hook quote or framing statement]*

---
# Core Concept

<<mermaid>>
  mindmap
    root((Topic))
      Area A
        Sub A1
      Area B
        Sub B1

---
# Hands-On Exercise

::: code-group
```python [Basic]
# minimal example
```

```python [Enhanced]
# more complete example
```
:::

---
# Common Mistakes to Avoid

::: danger
1. [Mistake 1]
2. [Mistake 2]
:::

---
# Key Takeaways

::: tip
1. [Takeaway 1]
2. [Takeaway 2]
:::

---
# Questions & Discussion

**Office Hours**: [when]
**Contact**: [email]
```

## Worked example

`examples/gallery/08_code_and_code_groups.slidelang` and
`examples/gallery/07_special_blocks_and_checklists.slidelang` cover the
code-group and checklist patterns used heavily in training content — read
both before writing a hands-on/workshop deck.

## Sample audience-calibration questions

- "What's the skill level of the audience going in?"
- "Is this self-paced (a document) or live (a presentation)? DocLang
  (`.doclang`) suits self-paced written material better than ZiraDocs."
- "How long is the session, and is there a hands-on component?"
