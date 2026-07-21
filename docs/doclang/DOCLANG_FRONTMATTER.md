# DocLang FrontMatter Configuration

FrontMatter is a YAML block at the top of DocLang files that defines metadata and global configuration for documents. This block is delimited by three dashes (`---`) and is crucial for defining the operating mode and document-specific features.

## 🎯 Differences from ZiraDocs

DocLang extends ZiraDocs's FrontMatter with fields specific to documents:

| Field | ZiraDocs | DocLang |
|-------|-----------|---------|
| `doctype` | Not required | **Required** (`document`) |
| `page` | Not applicable | Page configuration |
| `toc` | Not applicable | Table of contents |
| `numbering` | Not applicable | Section numbering |
| `references` | Not applicable | Reference system |

## 📋 Quick Example

```yaml
---
mode: flex
doctype: document
title: "Technical Specification"
author: "Engineering Team"
output:
  format: [html, pdf]
page:
  size: "A4"
  orientation: "portrait"
toc:
  enabled: true
  depth: 3
---

# Document content starts here...
```

## 🔧 Essential Fields

### `mode` (required)
Specifies the syntax mode for the document body.

```yaml
mode: flex  # or "strict"
```

**Values:**
- `flex` - Extended Markdown syntax
- `strict` - Formal syntax with keywords

### `doctype` (required for DocLang)
Specifies that a document is being created instead of a presentation.

```yaml
doctype: document
```

**Values:**
- `document` - Continuous document (DocLang)
- `presentation` - Slide presentation (ZiraDocs) [default]

### `title` (string)
Main title of the document.

```yaml
title: "System Architecture Specification"
```

### `subtitle` (string)
Optional subtitle of the document.

```yaml
subtitle: "Version 2.0 - Technical Documentation"
```

### `author` (string)
Primary author of the document.

```yaml
author: "Dr. Jane Smith"
```

### `authors` (array)
List of multiple authors with detailed information.

```yaml
authors:
  - name: "Dr. Jane Smith"
    affiliation: "University of Technology"
    email: "jane.smith@university.edu"
    orcid: "0000-0001-2345-6789"
  - name: "John Doe"
    affiliation: "Research Institute"
    email: "john.doe@research.org"
    role: "Contributing Author"
```

### `date` (string)
Document date.

```yaml
date: "October 8, 2024"
# or ISO format
date: "2024-10-08"
```

### `version` (string)
Document version.

```yaml
version: "2.0.0"
```

### `status` (string)
Document status.

```yaml
status: "Draft"  # Draft, Review, Final, Published
```

## 📄 Output Configuration

### `output` (object)
Configuration of output formats and destinations.

```yaml
output:
  format: [html, pdf, docx]
  path: "./dist/documents"
  filename: "technical-spec-v2"
```

**Properties:**
- `format` (array) - Output formats: `html`, `pdf`, `docx`, `markdown`
- `path` (string) - Output directory
- `filename` (string) - Base file name (without extension)

## 📏 Page Configuration

### `page` (object)
Page format and margin configuration.

```yaml
page:
  size: "A4"
  orientation: "portrait"
  margins:
    top: "2.5cm"
    bottom: "2.5cm"
    left: "3cm"
    right: "3cm"
    gutter: "0cm"
```

**page.size** (string):
- `A4` (210 x 297 mm)
- `Letter` (8.5 x 11 in)
- `Legal` (8.5 x 14 in)
- `A3` (297 x 420 mm)
- `A5` (148 x 210 mm)
- `B5` (176 x 250 mm)
- Custom: `"21cm x 29.7cm"`

**page.orientation** (string):
- `portrait` - Vertical (default)
- `landscape` - Horizontal

**page.margins** (object):
Margins in units: `cm`, `mm`, `in`, `pt`, `px`

## 📑 Headers and Footers

### `header` (object)
Document header configuration.

```yaml
header:
  enabled: true
  odd_pages: "{{title}} - v{{version}}"
  even_pages: "Chapter {{chapter_number}}: {{chapter_title}}"
  logo:
    src: "./assets/company-logo.png"
    height: "30px"
    position: "left"  # left, center, right
  text:
    position: "right"
    style: "minimal"  # minimal, standard, detailed
  divider: true
  first_page: false  # Do not show on the first page
```

**Properties:**
- `enabled` (boolean) - Enable/disable headers
- `odd_pages` (string) - Content for odd pages
- `even_pages` (string) - Content for even pages
- `logo` (object) - Logo configuration
- `divider` (boolean) - Separator line below the header
- `first_page` (boolean) - Show on the first page

### `footer` (object)
Document footer configuration.

```yaml
footer:
  enabled: true
  page_numbers:
    enabled: true
    format: "Page {{current}} of {{total}}"
    alignment: "center"  # left, center, right
    start_from: 1
    exclude_first: true
  odd_pages: "{{company}} - {{confidentiality}}"
  even_pages: "{{author}} - {{date}}"
  divider: true
```

**page_numbers.format** - Available variables:
- `{{current}}` - Current page number
- `{{total}}` - Total number of pages
- `{{section}}` - Current section number
- `{{chapter}}` - Current chapter number

## 📚 Table of Contents

### `toc` (object)
Table of contents configuration.

```yaml
toc:
  enabled: true
  depth: 3
  title: "Table of Contents"
  position: "before-content"  # before-content, after-cover, custom
  page_numbers: true
  hyperlinks: true
  style: "detailed"  # simple, detailed, nested
  exclude:
    - "Appendix"
    - "References"
```

**Properties:**
- `enabled` (boolean) - Generate TOC
- `depth` (integer) - Heading levels to include (1-6)
- `title` (string) - TOC title
- `position` (string) - Position in the document
- `page_numbers` (boolean) - Show page numbers
- `hyperlinks` (boolean) - Clickable links
- `style` (string) - Display style
- `exclude` (array) - Sections to exclude

## 🔢 Numbering

### `numbering` (object)
Automatic numbering configuration.

```yaml
numbering:
  enabled: true
  style: "hierarchical"  # hierarchical, sequential, custom
  prefix: ""
  suffix: "."
  separator: "."
  sections:
    enabled: true
    start_from: 1
    format: "{{number}}."
  figures:
    enabled: true
    prefix: "Figure"
    format: "{{prefix}} {{number}}:"
  tables:
    enabled: true
    prefix: "Table"
    format: "{{prefix}} {{number}}:"
  charts:
    enabled: true
    prefix: "Chart"
    format: "{{prefix}} {{number}}:"
  equations:
    enabled: true
    prefix: "Eq."
    format: "({{prefix}} {{number}})"
```

**numbering.style**:
- `hierarchical` - 1, 1.1, 1.1.1, 1.1.2, 1.2, 2, 2.1...
- `sequential` - 1, 2, 3, 4, 5, 6...
- `custom` - Custom format

## 📖 References and Citations

### `references` (object)
Bibliographic reference system.

```yaml
references:
  enabled: true
  style: "apa"  # apa, mla, chicago, ieee, harvard
  footnotes:
    enabled: true
    position: "bottom"  # bottom, end-of-section, end-of-document
    numbering: "sequential"  # sequential, per-section, per-page
  bibliography:
    enabled: true
    title: "References"
    sort: "author"  # author, year, title, citation-order
    position: "end"
```

**Supported citation styles:**
- `apa` - American Psychological Association (7th ed.)
- `mla` - Modern Language Association (9th ed.)
- `chicago` - Chicago Manual of Style (17th ed.)
- `ieee` - Institute of Electrical and Electronics Engineers
- `harvard` - Harvard referencing style

## 🎨 Themes and Styles

### `theme` (string)
Visual theme of the document.

```yaml
theme: "professional"
```

**Available themes:**
- `professional` - Formal corporate style
- `academic` - Academic/scientific style
- `technical` - Technical documentation
- `minimal` - Minimalist and clean
- `modern` - Modern design
- `legal` - Legal/contractual format
- `report` - Corporate reports
- `manuscript` - Academic manuscript

### `custom_css` (string | array)
Custom CSS stylesheets.

```yaml
custom_css: "./styles/custom-document.css"

# or multiple files
custom_css:
  - "./styles/base.css"
  - "./styles/print.css"
  - "./styles/custom.css"
```

### `fonts` (object)
Typography configuration.

```yaml
fonts:
  body: "Georgia, serif"
  headings: "Arial, sans-serif"
  code: "Courier New, monospace"
  size:
    base: "12pt"
    small: "10pt"
    large: "14pt"
  line_height: 1.6
```

## 🌍 Language Configuration

### `language` (string)
Main language of the content.

```yaml
language: "en-US"
```

**Common values:**
- `en-US` - English (United States)
- `en-GB` - English (United Kingdom)
- `es-ES` - Spanish (Spain)
- `es-MX` - Spanish (Mexico)
- `fr-FR` - French
- `de-DE` - German
- `pt-BR` - Portuguese (Brazil)

### `localization` (object)
Localized text for automatic elements.

```yaml
localization:
  toc_title: "Table of Contents"
  figure_prefix: "Figure"
  table_prefix: "Table"
  chart_prefix: "Chart"
  page_label: "Page"
  chapter_label: "Chapter"
  section_label: "Section"
```

## 🔐 Metadata and Security

### `metadata` (object)
Additional document metadata.

```yaml
metadata:
  keywords:
    - "software architecture"
    - "microservices"
    - "API design"
  subject: "Technical Specification"
  category: "Software Engineering"
  confidentiality: "Internal Use Only"
  classification: "Confidential"
  distribution: "Limited"
  copyright: "© 2024 TechCorp Inc. All rights reserved."
  license: "CC BY-NC-SA 4.0"
```

### `security` (object)
Security and permissions configuration.

```yaml
security:
  watermark:
    enabled: true
    text: "CONFIDENTIAL - {{company}}"
    opacity: 0.1
    position: "diagonal"  # diagonal, header, footer
  pdf:
    permissions:
      print: true
      copy: false
      modify: false
      annotate: true
    password_protection:
      enabled: false
      password: ""
```

## 🔗 Custom Variables

### `variables` (object)
Reusable variables throughout the document.

```yaml
variables:
  company: "TechCorp Inc."
  product: "CloudAPI Platform"
  version: "3.1.0"
  api_version: "v3"
  base_url: "https://api.techcorp.com"
  support_email: "support@techcorp.com"
  copyright_year: "2024"
  confidentiality: "Internal Use Only"
```

**Usage in the document:**
```markdown
Welcome to {{company}}'s {{product}} documentation.

API Base URL: {{base_url}}/{{api_version}}
```

## ⚙️ Advanced Features

### `features` (object)
Optional document features.

```yaml
features:
  search:
    enabled: true
    index_content: true
    include_code: false
  print_friendly:
    enabled: true
    page_breaks: "auto"  # auto, manual, none
    orphan_control: true
    widow_control: true
  responsive:
    enabled: true
    breakpoints:
      mobile: "768px"
      tablet: "1024px"
      desktop: "1440px"
  dark_mode:
    enabled: false
    auto_switch: false
  accessibility:
    aria_labels: true
    alt_text_required: true
    color_contrast: "AAA"  # AA, AAA
```

### `plugins` (array)
Plugins and extensions.

```yaml
plugins:
  - name: "doclang-charts-advanced"
    version: "^1.0"
    config:
      default_colors: ["#4285F4", "#34A853", "#FBBC04", "#EA4335"]
  - name: "doclang-latex-math"
    version: "^2.0"
    config:
      renderer: "katex"
  - name: "doclang-citations"
    version: "^1.5"
```

## 📊 Full Example - Academic Document

```yaml
---
# Basic configuration
mode: flex
doctype: document

# Metadata
title: "Microservices Architecture Patterns"
subtitle: "A Comprehensive Analysis of Modern Software Design"
authors:
  - name: "Dr. Jane Smith"
    affiliation: "Department of Computer Science, Tech University"
    email: "j.smith@techuniversity.edu"
    orcid: "0000-0001-2345-6789"
  - name: "Prof. John Doe"
    affiliation: "Software Engineering Institute"
    email: "jdoe@sei.org"
    role: "Principal Investigator"
date: "October 8, 2024"
version: "1.0.0"
status: "Published"

# Output
output:
  format: [pdf, html, docx]
  path: "./publications"
  filename: "microservices-architecture-2024"

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
  odd_pages: "{{title}}"
  even_pages: "{{authors[0].name}} et al."
  divider: true
  first_page: false

footer:
  enabled: true
  page_numbers:
    enabled: true
    format: "{{current}}"
    alignment: "center"
    start_from: 1
    exclude_first: true
  divider: true

# Table of contents
toc:
  enabled: true
  depth: 3
  title: "Contents"
  position: "before-content"
  page_numbers: true
  hyperlinks: true
  style: "detailed"

# Numbering
numbering:
  enabled: true
  style: "hierarchical"
  sections:
    enabled: true
  figures:
    enabled: true
    prefix: "Figure"
  tables:
    enabled: true
    prefix: "Table"

# References
references:
  enabled: true
  style: "apa"
  footnotes:
    enabled: true
    position: "bottom"
    numbering: "per-page"
  bibliography:
    enabled: true
    title: "References"
    sort: "author"

# Theme
theme: "academic"

# Language configuration
language: "en-US"
localization:
  toc_title: "Contents"
  figure_prefix: "Figure"
  table_prefix: "Table"

# Metadata
metadata:
  keywords:
    - "microservices"
    - "software architecture"
    - "distributed systems"
    - "design patterns"
  subject: "Software Engineering"
  category: "Research Paper"
  copyright: "© 2024 Authors. CC BY 4.0"
  license: "Creative Commons Attribution 4.0 International"

# Variables
variables:
  institution: "Tech University"
  department: "Computer Science"
  project: "Software Architecture Research Project"
  funding: "National Science Foundation Grant #12345"

# Features
features:
  search:
    enabled: true
    index_content: true
  print_friendly:
    enabled: true
    page_breaks: "auto"
    orphan_control: true
    widow_control: true
  accessibility:
    aria_labels: true
    alt_text_required: true
    color_contrast: "AAA"

# Plugins
plugins:
  - name: "doclang-latex-math"
    config:
      renderer: "katex"
  - name: "doclang-citations"
    config:
      style: "apa"
---
```

## 📊 Full Example - Technical Document

```yaml
---
# Basic configuration
mode: flex
doctype: document

# Metadata
title: "System Architecture Specification"
subtitle: "CloudAPI Platform v3.1"
author: "Platform Engineering Team"
date: "2024-10-08"
version: "3.1.0"
status: "Final"

# Output
output:
  format: [html, pdf]
  path: "./docs/architecture"
  filename: "system-architecture-v3.1"

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
  odd_pages: "{{title}} v{{version}}"
  even_pages: "Section {{section_number}}: {{section_title}}"
  logo:
    src: "./assets/company-logo.png"
    height: "30px"
    position: "left"
  style: "professional"
  divider: true
  first_page: false

footer:
  enabled: true
  page_numbers:
    enabled: true
    format: "Page {{current}} of {{total}}"
    alignment: "center"
  odd_pages: "{{company}}"
  even_pages: "{{confidentiality}}"
  divider: true

# Table of contents
toc:
  enabled: true
  depth: 3
  title: "Table of Contents"
  page_numbers: true
  hyperlinks: true
  style: "detailed"

# Numbering
numbering:
  enabled: true
  style: "hierarchical"
  sections:
    enabled: true
  figures:
    enabled: true
  tables:
    enabled: true
  charts:
    enabled: true

# Theme
theme: "technical-documentation"

# Variables
variables:
  company: "TechCorp Inc."
  product: "CloudAPI Platform"
  api_version: "v3.1"
  base_url: "https://api.techcorp.com"
  confidentiality: "Confidential - Internal Use Only"

# Metadata
metadata:
  keywords:
    - "architecture"
    - "microservices"
    - "API"
    - "cloud platform"
  confidentiality: "Internal Use Only"
  classification: "Confidential"
  copyright: "© 2024 TechCorp Inc. All rights reserved."

# Security
security:
  watermark:
    enabled: true
    text: "CONFIDENTIAL - {{company}}"
    opacity: 0.1
    position: "diagonal"

# Features
features:
  search:
    enabled: true
  print_friendly:
    enabled: true
    page_breaks: "auto"
  responsive:
    enabled: true
  dark_mode:
    enabled: false
---
```

## 🎯 Best Practices

1. **Include `doctype: document`** - Required for DocLang
2. **Configure the page appropriately** - Adjust margins and size according to use
3. **Enable TOC for long documents** - Improves navigation
4. **Use hierarchical numbering** - Facilitates cross-references
5. **Define variables for repeated content** - Maintains consistency
6. **Configure headers/footers** - Adds professionalism
7. **Specify complete metadata** - Improves organization

## 📚 See Also

- [DocLang Overview](DOCLANG_OVERVIEW.md)
- [DocLang Strict Mode](DOCLANG_SYNTAX_STRICT.md)
- [DocLang Flex Mode](DOCLANG_SYNTAX_FLEX.md)
- [ZiraDocs FrontMatter](../user/language-reference/frontmatter.md)
- [Variables and Templates](../user/features/variables-templates.md)
