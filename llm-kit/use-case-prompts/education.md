# Use Case: Educational / Training

Narrative arc: **Objectives → Warm-up → Core Concepts → Practice →
Summary**. Tone: progressive complexity, interactive framing — but see the
important caveat below about what "interactive" can actually mean in
ZiraDocs output.

## Interactive elements: the two real tags, and nothing else

Polls and quizzes are real elements. Use `<<poll>>` and `<<quiz>>` with a YAML
body closed by `<<end>>` — never `:::poll`, `:::qa_session`, `:::reveal` or a
`:::notes` block, which are not implemented (`SPECIAL001`). Presenter notes are
the `@notes` directive.

```
# Pre-Session Check

<<poll>>
question: "What's your experience with data visualization?"
options:
  - "Beginner"
  - "Some Excel"
  - "Some Python/R"
  - "I build them regularly"
<<end>>

<<quiz>>
question: "Which chart type fits a part-to-whole comparison?"
options: ["Line", "Pie", "Scatter"]
answer: 1
explanation: "A pie chart shows shares of a single total."
<<end>>
```

`answer` is a **0-based index**: `answer: 1` selects the *second* option. An
out-of-range value is a hard error (`QUIZ001`), not a warning.

Both are **static**: the HTML reveals the answer on click, and every other
format (PDF, PPTX, DOCX, Markdown) renders the quiz already solved. Nothing is
collected — there is no backend, so never promise the audience their answers
are recorded.

```
# Knowledge Check (placeholder)

**Question**: Which chart type best shows parts of a whole?
**Expected answer**: Pie chart. Discuss reasoning with the group.
```

## Skeleton

The skeleton below is flex mode — every slide is just `title`/`content`
type unless a `layout:` block types it (a lone `---` between slides is a
plain separator, not a layout block — see `reference/slidelang-flex.md`).

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
