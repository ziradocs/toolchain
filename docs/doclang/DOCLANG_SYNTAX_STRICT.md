# DocLang Strict Mode Syntax

> ⚠️ **Aspirational — not implemented.** `doclang build` always parses with
> `DocumentFlexParser`, regardless of a document's frontmatter `mode:` value
> (see `doclang/internal/cli/build.go`). The strict syntax described
> below is not recognized by any real doclang pipeline today; a document
> written this way will parse as (mostly empty) flex content rather than as
> the structured `SECTION`/`TEXT`/`POINTS` blocks shown here. Tracked in
> [ziradocs/toolchain#198](https://github.com/ziradocs/toolchain/issues/198). Treat
> this document as a design sketch for a future strict mode, not current
> behavior.

**DocLang Strict Mode** provides a formal, structured syntax for defining documents. It fully inherits the syntax of ZiraDocs Strict Mode, but adapts the "slides" semantics to "document sections".

## 🎯 Philosophy

Instead of thinking in terms of **discrete slides**, think in terms of **hierarchical sections** that flow within a continuous document.

## 🔑 Key Concepts

| ZiraDocs Concept | DocLang Equivalent |
|-------------------|-------------------|
| SLIDE | SECTION (document section) |
| Slide type (title, content) | Section level (1, 2, 3...) |
| Slide separator | Section boundary (automatic) |
| Presentation flow | Document hierarchy |

## 📝 Fundamental Syntax

### Declaring Sections

```doclang
---
mode: strict
doctype: document
title: "Technical Specification"
---

SECTION "Introduction"
  level: 1
  id: "intro"
  
  TEXT
    This document describes the technical specifications.

SECTION "Background"
  level: 2
  id: "intro-background"
  
  TEXT
    Historical context and motivation.

SECTION "Architecture"
  level: 1
  id: "architecture"
  
  TEXT
    System architecture overview.
```

**SECTION properties:**
- `level`: Hierarchical level (1-6, corresponding to H1-H6)
- `id`: Unique identifier for cross-references
- `numbered`: Whether the section is auto-numbered (default: true)
- `pagebreak`: Force a page break before the section

### Text Elements

Identical to ZiraDocs:

```doclang
SECTION "Methodology"
  level: 1
  
  TEXT
    Our research methodology follows established practices in the field.
    
    We conducted a **comprehensive analysis** of existing systems and 
    identified *key patterns* that inform our design.
    
    The methodology includes three phases:

  POINTS
    1. Data collection and analysis
       a. Survey distribution
       b. Interview sessions
    2. System design
       - Architecture planning
       - Component specification
    3. Validation and testing
```

### Code Blocks

```doclang
SECTION "Implementation"
  level: 2
  
  TEXT
    The authentication module is implemented as follows:
    
  CODE javascript
    class AuthenticationService {
      async authenticate(credentials) {
        const token = await this.validateCredentials(credentials);
        return this.generateSession(token);
      }
      
      validateCredentials(credentials) {
        // Validation logic
        return bcrypt.compare(
          credentials.password,
          this.storedHash
        );
      }
    }
  
  TEXT
    This implementation uses industry-standard bcrypt hashing.
```

### Tables

```doclang
SECTION "Performance Metrics"
  level: 2
  
  TEXT
    The following table summarizes performance across different scenarios:
    
  TABLE
    headers: ["Scenario", "Response Time", "Throughput", "Error Rate"]
    rows: [
      ["Light Load", "45ms", "1000 req/s", "0.01%"],
      ["Medium Load", "120ms", "2500 req/s", "0.05%"],
      ["Heavy Load", "340ms", "5000 req/s", "0.15%"]
    ]
    caption: "Table 1: Performance under different load conditions"
    id: "table-performance"
```

### Images

```doclang
SECTION "User Interface"
  level: 2
  
  TEXT
    The main dashboard provides an overview of system status:
    
  IMAGE "assets/dashboard-screenshot.png" "Main dashboard interface"
    caption: "Figure 1: System dashboard showing key metrics"
    id: "fig-dashboard"
    width: "80%"
    alignment: "center"
```

## 🎨 Special Blocks

### Info Blocks

```doclang
SECTION "Important Considerations"
  level: 2
  
  :::info
  **Note**: All API endpoints require authentication.
  :::
  
  :::warning
  **Security Warning**: Never store credentials in plain text.
  :::
  
  :::danger
  **Critical**: Database migration is irreversible. Backup first!
  :::
  
  :::success
  **Best Practice**: Use environment variables for configuration.
  :::
```

## 📊 Advanced Elements

### Charts

```doclang
SECTION "Revenue Analysis"
  level: 2
  
  TEXT
    Revenue growth over the past four quarters:
    
  <<chart: line>>
    data: [
      ["Q1 2024", 125000],
      ["Q2 2024", 145000],
      ["Q3 2024", 167000],
      ["Q4 2024", 189000]
    ]
    series: ["Revenue ($)"]
    options:
      responsive: true
      plugins:
        title:
          display: true
          text: "Quarterly Revenue Growth"
    caption: "Chart 1: Revenue trends Q1-Q4 2024"
    id: "chart-revenue"
  <<>>
```

### Mermaid Diagrams

```doclang
SECTION "System Architecture"
  level: 2
  
  TEXT
    The system follows a microservices architecture:
    
  <<mermaid>>
    graph TB
      A[Client Application] --> B[API Gateway]
      B --> C[Auth Service]
      B --> D[Data Service]
      B --> E[Analytics Service]
      C --> F[(User DB)]
      D --> G[(Main DB)]
      E --> H[(Analytics DB)]
    caption: "Figure 2: System architecture diagram"
    id: "diagram-architecture"
  <<>>
  
  TEXT
    Each service is independently deployable and scalable.
```

### Interactive Maps

```doclang
SECTION "Geographic Distribution"
  level: 2
  
  TEXT
    Our services are deployed across multiple global regions:
    
  <<map>>
    type: world
    markers:
      - lat: 37.7749
        lng: -122.4194
        label: "US West (San Francisco)"
        value: 15000
      - lat: 40.7128
        lng: -74.0060
        label: "US East (New York)"
        value: 12000
      - lat: 51.5074
        lng: -0.1278
        label: "Europe (London)"
        value: 8500
    zoom: 2
    caption: "Map 1: Global deployment centers"
    id: "map-deployment"
  <<>>
```

## 🔗 DocLang-Specific Elements

### Table of Contents

```doclang
SECTION "Contents"
  level: 1
  pagebreak: false
  
  <<toc>>
    depth: 3
    title: "Table of Contents"
    numbered: true
  <<>>
```

### Cross-References

```doclang
SECTION "Related Information"
  level: 2
  
  TEXT
    For more details on the architecture, see <<ref: architecture>>.
    
    Performance metrics are detailed in <<ref: table-performance>>.
    
    The dashboard interface is shown in <<ref: fig-dashboard>>.
```

### Footnotes

```doclang
SECTION "Research Methodology"
  level: 2
  
  TEXT
    Our approach follows the established framework<<footnote: fn1>>.
    
  <<footnote: fn1>>
    Smith, J. et al. (2023). "Modern Software Architecture Patterns". 
    Journal of Software Engineering, 45(2), 123-145.
  <<>>
```

### Page Breaks

```doclang
SECTION "Executive Summary"
  level: 1
  pagebreak: false
  
  TEXT
    Summary content here...

<<pagebreak>>

SECTION "Detailed Analysis"
  level: 1
  
  TEXT
    Detailed content starts on a new page...
```

### Bibliography

```doclang
SECTION "References"
  level: 1
  numbered: false
  
  <<bibliography>>
    style: "apa"
    entries:
      - id: "smith2023"
        type: "article"
        authors: ["Smith, J.", "Jones, M."]
        title: "Modern Software Architecture Patterns"
        journal: "Journal of Software Engineering"
        year: 2023
        volume: 45
        issue: 2
        pages: "123-145"
      - id: "doe2024"
        type: "book"
        authors: ["Doe, J."]
        title: "Microservices in Practice"
        publisher: "Tech Press"
        year: 2024
        isbn: "978-1234567890"
  <<>>
```

## 📋 Complete FrontMatter for Strict Mode

```yaml
---
# Basic configuration
mode: strict
doctype: document

# Metadata
title: "System Architecture Specification"
subtitle: "Version 2.0"
author: "Engineering Team"
authors:
  - name: "John Doe"
    affiliation: "Lead Architect"
    email: "john@example.com"
  - name: "Jane Smith"
    affiliation: "Senior Engineer"
    email: "jane@example.com"
date: "2024-10-08"
version: "2.0.0"
status: "Draft"

# Output
output:
  format: [html, pdf]
  path: "./dist"
  filename: "architecture-spec-v2"

# Page configuration
page:
  size: "A4"
  orientation: "portrait"
  margins:
    top: "2.5cm"
    bottom: "2.5cm"
    left: "3cm"
    right: "3cm"

# Headers and footers
header:
  enabled: true
  odd_pages: "{{title}} - v{{version}}"
  even_pages: "Section {{section_number}}: {{section_title}}"
  style: "minimal"
  divider: true
  
footer:
  enabled: true
  page_numbers:
    enabled: true
    format: "Page {{current}} of {{total}}"
    alignment: "center"
    start_from: 1
  odd_pages: "{{company}}"
  even_pages: "{{confidentiality}}"
  divider: true

# Table of contents
toc:
  enabled: true
  depth: 3
  title: "Table of Contents"
  page_numbers: true
  position: "after-cover"

# Numbering
numbering:
  enabled: true
  style: "hierarchical"
  prefix: ""
  suffix: "."
  start_from: 1

# References
references:
  style: "apa"
  
# Theme
theme: "technical"

# Custom variables
variables:
  company: "Acme Corporation"
  project: "Phoenix System"
  confidentiality: "Confidential - Internal Use Only"
---
```

## 📚 Complete Example

```doclang
---
mode: strict
doctype: document
title: "API Integration Guide"
author: "Platform Team"
theme: "technical"
toc:
  enabled: true
  depth: 3
---

SECTION "Introduction"
  level: 1
  id: "intro"
  
  TEXT
    This guide provides comprehensive instructions for integrating 
    with our API platform.

SECTION "Getting Started"
  level: 2
  id: "getting-started"
  
  TEXT
    Before you begin, ensure you have:
    
  POINTS
    - A valid API key
    - Access to the developer portal
    - Basic understanding of REST APIs

SECTION "Authentication"
  level: 1
  id: "authentication"
  
  TEXT
    All API requests require authentication using Bearer tokens.
    
  CODE javascript
    const response = await fetch('https://api.example.com/data', {
      headers: {
        'Authorization': 'Bearer YOUR_API_KEY',
        'Content-Type': 'application/json'
      }
    });
  
  :::warning
  **Security Note**: Never expose your API key in client-side code.
  :::

SECTION "Endpoints"
  level: 1
  id: "endpoints"
  
  TABLE
    headers: ["Endpoint", "Method", "Description"]
    rows: [
      ["/api/users", "GET", "List all users"],
      ["/api/users/:id", "GET", "Get user by ID"],
      ["/api/users", "POST", "Create new user"]
    ]
    caption: "Table 1: Available API endpoints"
    id: "table-endpoints"

SECTION "Rate Limiting"
  level: 1
  id: "rate-limiting"
  
  TEXT
    Our API implements rate limiting to ensure fair usage:
    
  <<chart: bar>>
    data: [
      ["Free Tier", 100],
      ["Basic Plan", 1000],
      ["Pro Plan", 10000],
      ["Enterprise", 100000]
    ]
    series: ["Requests per hour"]
    options:
      responsive: true
    caption: "Chart 1: Rate limits by plan"
    id: "chart-rate-limits"
  <<>>

SECTION "Error Handling"
  level: 1
  id: "error-handling"
  
  TEXT
    The API uses standard HTTP status codes. For detailed error 
    information, see <<ref: table-error-codes>>.
    
  TABLE
    headers: ["Status Code", "Meaning", "Description"]
    rows: [
      ["200", "OK", "Request successful"],
      ["400", "Bad Request", "Invalid request format"],
      ["401", "Unauthorized", "Invalid or missing API key"],
      ["429", "Too Many Requests", "Rate limit exceeded"],
      ["500", "Server Error", "Internal server error"]
    ]
    caption: "Table 2: HTTP status codes"
    id: "table-error-codes"

SECTION "References"
  level: 1
  numbered: false
  
  <<bibliography>>
    style: "apa"
    entries:
      - id: "rest-spec"
        type: "web"
        title: "REST API Design Specification"
        url: "https://restfulapi.net/"
        accessed: "2024-10-08"
  <<>>
```

## 🎯 Best Practices

1. **Use descriptive section titles** - Clear, concise titles improve navigation
2. **Maintain consistent hierarchy** - Follow logical document structure
3. **Leverage cross-references** - Link related sections with `<<ref>>`
4. **Add captions to elements** - Tables, figures, and charts should be captioned
5. **Use appropriate section levels** - Don't skip levels (e.g., 1 → 3)
6. **Include IDs for referenceable elements** - Tables, figures, sections
7. **Break long sections** - Use subsections for better readability

## 🔄 Migrating from Flex Mode

| Flex Mode | Strict Mode |
|-----------|-------------|
| `# Section` | `SECTION "Section" level: 1` |
| `## Subsection` | `SECTION "Subsection" level: 2` |
| Text paragraph | `TEXT` block |
| `- List` | `POINTS` block |
| ` ```code``` ` | `CODE` block |

## 📖 See Also

- [DocLang Overview](DOCLANG_OVERVIEW.md)
- [DocLang Flex Mode](DOCLANG_SYNTAX_FLEX.md)
- [ZiraDocs Strict Mode](../user/language-reference/strict-mode.md)
- [DocLang FrontMatter](DOCLANG_FRONTMATTER.md)
