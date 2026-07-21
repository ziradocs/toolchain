# Variables & Templates

ZiraDocs features a comprehensive variable and templating system that enables content reuse, dynamic customization, and efficient management of complex presentations. This system helps maintain consistency and reduce duplication across your presentations.

## Quick Example

```slidelang
---
mode: flex
title: "{{company}} {{quarter}} Report"
variables:
  company: "TechCorp"
  quarter: "Q4 2024"
  revenue: 5200000
  growth: 45.5
---

# {{company}} Results - {{quarter}}

We achieved revenues of **${{revenue|number}}** with a **{{growth}}%** YoY growth.

## Key Metrics
- Revenue: ${{revenue|currency}}
- Growth: {{growth}}%
- Team Size: {{team_size|default:127}}
```

## 1. Basic Variables

Define variables directly in your FrontMatter for use throughout your presentation.

### Simple Variables

```yaml
---
mode: flex
title: "Presentation {{quarter}} {{year}}"
variables:
  # String variables
  company: "TechCorp"
  author: "María García"
  quarter: "Q4"
  year: "2025"
  
  # Numeric variables
  revenue: 5200000
  growth: 45.5
  employees: 127
---

## {{company}} Results - {{quarter}} {{year}}

We achieved revenues of **${{revenue}}** with a **{{growth}}%** YoY growth.
```

### Structured Variables

Organize related variables using nested objects:

```yaml
variables:
  # Product information
  product:
    name: "ZiraDocs Pro"
    version: "v3.1"
    category: "Productivity"
  
  # Business metrics
  metrics:
    users: "15,847"
    revenue: "$487K"
    growth: "+28.5%"
  
  # Contact information
  contact:
    sales: "sales@company.com"
    support: "+1 (555) 123-4567"
```

**Usage:**
```markdown
## Introducing {{product.name}} {{product.version}}

- Users: {{metrics.users}}
- Growth: {{metrics.growth}}
- Contact: {{contact.sales}}
```

## 2. Variable Types and Formats

ZiraDocs supports various data types and formatting options.

### Numbers and Formatting

```yaml
variables:
  # Raw numeric values
  revenue: 5200000
  growth_decimal: 0.455
  employees: 127
  
  # Formatted display values (conceptual filters)
  revenue_display: "{{revenue|currency:USD}}"    # $5,200,000
  revenue_compact: "{{revenue|compact}}"         # $5.2M
  growth_percent: "{{growth_decimal|percent}}"   # 45.5%
```

### Dates and Times

```yaml
variables:
  # Date strings
  date: "2025-01-15"
  
  # Formatted dates (conceptual filters)
  date_full: "{{date|format:MMMM DD, YYYY}}"    # January 15, 2025
  date_short: "{{date|format:DD/MM/YY}}"        # 15/01/25
```

### Arrays and Lists

```yaml
variables:
  regions: ["North", "South", "East", "West"]
  top_region: "{{regions[0]}}"                   # North
  region_count: "{{regions|length}}"             # 4
```

### Conditional Values

```yaml
variables:
  revenue: 5200000
  growth: 45.5
  
  # Conditional logic (conceptual syntax)
  status: "{{revenue > 5000000 ? 'Exceptional' : 'Normal'}}"
  emoji: "{{growth > 40 ? '🚀' : '📈'}}"
```

## 3. Dynamic and Computed Variables

Perform calculations and use system-provided data.

### System Variables

```yaml
variables:
  # System-provided values (conceptual)
  _today: "{{system.date}}"
  _author: "{{system.user}}"
  _version: "{{system.version}}"
```

### Calculations

```yaml
variables:
  revenue: 5200000
  employees: 127
  
  # Computed values
  monthly_revenue: "{{revenue / 12}}"
  revenue_per_employee: "{{revenue / employees}}"
  
  # Comparisons
  last_year_revenue: 4000000
  growth_rate: "{{(revenue / last_year_revenue - 1) * 100}}"
```

**Usage:**
```markdown
## Key Metrics

- Daily Revenue: **{{monthly_revenue|currency}}/month**
- Revenue Per Employee: **{{revenue_per_employee|currency}}**
- Growth Rate: **{{growth_rate|round:1}}%**
```

## 4. Contextual Variables

Manage variable scope from global to slide-local.

### Global Variables

```yaml
---
variables:
  company: "TechCorp"
  author: "Global Author"
  footer_text: "{{company}} - Confidential"
---
```

### Slide-Local Variables

Use slide-specific variables with the `@var` directive:

```slidelang
## Regional Performance
@var:revenue_local: 1200000
@var:region: "North"

Revenue in {{region}}: **{{revenue_local|currency}}**

This represents **{{(revenue_local/revenue)*100|round:1}}%** of the total.
Footer: {{footer_text}}
```

## 5. Advanced Data Binding

Work with structured data and perform aggregations.

### Structured Data

```yaml
data:
  sales:
    - region: "North"
      q1: 250000
      q2: 280000
      q3: 320000
      q4: 380000
    - region: "South"
      q1: 150000
      q2: 170000
      q3: 190000
      q4: 220000
  
  # Computed aggregations (conceptual)
  totals:
    q4: "{{data.sales|sum:'q4'}}"              # 600000
    best_q4: "{{data.sales|max:'q4'}}"         # 380000
    avg_q4: "{{data.sales|average:'q4'}}"      # 300000
```

### Data Iteration

```slidelang
## Results by Region

{{#each data.sales}}
### {{region}}
- Q4: **{{q4|currency}}**
- Growth (Q4 vs Q3): **{{(q4/q3-1)*100|round:1}}%**
{{/each}}

**Totals:**
- Total Q4: {{totals.q4|currency}}
- Average Q4: {{totals.avg_q4|currency}}
```

## 6. Templates and Reusable Blocks

Define reusable content blocks for consistency.

### Template Definition

```yaml
templates:
  metric_card: |
    ### {{title}}
    **{{value|format:type}}** {{unit}}
    {{trend}} vs {{comparison}}
    
  employee_card: |
    ![{{name}}]({{photo}})
    **{{name}}**
    {{role}} | {{department}}
    📧 {{email}}
```

### Template Usage

```slidelang
## Key Metrics

{{>metric_card 
  title="Revenue" 
  value=revenue 
  type="currency" 
  trend="+45%" 
  comparison="Q3"}}

{{>metric_card 
  title="Employees" 
  value=employees 
  type="number" 
  trend="+15" 
  comparison="last month"}}
```

## 7. Multi-language Support

Manage internationalized strings and localized content.

### Language Configuration

```yaml
lang: "es"
variables:
  company: "TechCorp"
  revenue: 5200000
  
i18n:
  es:
    title: "Resultados Trimestrales"
    revenue_label: "Ingresos"
    growth_label: "Crecimiento"
  en:
    title: "Quarterly Results"
    revenue_label: "Revenue"
    growth_label: "Growth"
```

### Localized Content

```slidelang
## {{i18n[lang].title}}

- {{i18n[lang].revenue_label}}: **{{revenue|currency}}**
- {{i18n[lang].growth_label}}: **{{growth}}%**
```

## 8. Environment Variables

Manage settings for different environments and handle configuration.

### Environment-Based Configuration

```yaml
# Environment-specific settings
env:
  production:
    api_url: "https://api.techcorp.com"
    debug: false
  development:
    api_url: "http://localhost:3000"
    debug: true

# External environment variables
secrets:
  api_key: "{{env.SLIDELANG_API_KEY}}"
  analytics_id: "{{env.ANALYTICS_ID}}"

# Active environment
active_env: "{{env.SLIDELANG_ENV || 'development'}}"
```

**Usage:**
```markdown
## Configuration

- API URL: {{env[active_env].api_url}}
- Debug Mode: {{env[active_env].debug}}
- API Key Loaded: {{secrets.api_key ? "✓" : "✗"}}
```

## 9. Best Practices

Follow these guidelines for effective variable management.

### ✅ Good Practices

```yaml
variables:
  # Descriptive names
  quarterly_revenue: 5200000     # Not just 'revenue'
  year_over_year_growth: 45      # Not just 'growth'
  
  # Logical grouping
  financial_metrics:
    revenue: 5200000
    expenses: 4200000
    profit: 1000000
    
  # Default values
  currency_symbol: "{{config.currency || '$'}}"
  default_region: "{{params.region || 'Global'}}"
```

### ❌ Avoid

```yaml
# Poor practices
variables:
  r: 5200000              # Too short, lacks clarity
  revenue$: "5.2M"        # Mixing formatting with data
  Q42025revenue: 1000000  # Hard to read naming
```

### Variable Naming

- Use **snake_case** consistently
- Be **descriptive** but not verbose
- **Group** related variables under objects
- **Document** complex variables with comments

## 10. Specific Use Cases

### Corporate Reports

```yaml
variables:
  # Update only these each quarter
  report_quarter: "Q1"
  report_year: "2025"
  current_revenue: 5200000
  current_expenses: 4200000
  
  # Calculated metrics
  profit: "{{current_revenue - current_expenses}}"
  margin: "{{(profit / current_revenue * 100)|round:1}}"
```

### Personalized Presentations

```yaml
variables:
  # CLI-injected variables
  client_name: "{{params.client || 'Valued Client'}}"
  client_industry: "{{params.industry || 'your industry'}}"
  client_size: "{{params.size || 'your company'}}"
```

**Usage:**
```markdown
# Proposal for {{client_name}}

We have extensive experience in {{client_industry}} with companies like {{client_size}}.
```

### A/B Testing

```yaml
variables:
  test_variant: "A"
  
  # Version-specific content
  headline_a: "Increase Productivity 300%"
  headline_b: "The Tool You've Been Waiting For"
  
  # Dynamic selection
  current_headline: "{{test_variant == 'A' ? headline_a : headline_b}}"
```

## 11. Integration with Themes

Variables work seamlessly with ZiraDocs's theme system.

### Theme Variables

```yaml
theme: "corporate"
variables:
  brand_primary: "#2c5282"
  brand_secondary: "#4a90e2"
  company_logo: "./assets/logo.png"
```

### Header/Footer Integration

```yaml
header:
  logo:
    src: "{{company_logo}}"
    alt: "{{company}} Logo"
  text:
    content: "{{department}} • {{quarter}} {{year}}"

footer:
  text:
    left: "Confidential - {{company}}"
    center: "{{title}}"
    right: "{{date}}"
```

## 12. CLI Integration

Variables work with ZiraDocs CLI commands and options.

### Build with Variables

```bash
# Basic build with variables from FrontMatter
slidelang build presentation.slidelang

# Override theme with variables
slidelang build slides.slidelang --theme corporate

# Environment-specific builds
SLIDELANG_ENV=production slidelang build presentation.slidelang
```

### Variable Validation

The CLI validates variables during build:

```bash
# Enable detailed logging to see variable processing
slidelang build presentation.slidelang --log-level debug

# Lint-only mode checks variables without generating output
slidelang build --lint-only presentation.slidelang
```

## Common Patterns

### Dashboard Slides

```yaml
variables:
  kpis:
    - name: "Revenue"
      value: "{{revenue|currency}}"
      trend: "+12%"
      color: "green"
    - name: "Users"
      value: "{{users|number}}"
      trend: "+8%"
      color: "blue"
```

### Team Presentations

```yaml
variables:
  team_members:
    - name: "Alice Johnson"
      role: "Lead Developer"
      email: "alice@company.com"
    - name: "Bob Smith"
      role: "Product Manager"
      email: "bob@company.com"
```

### Progress Reports

```yaml
variables:
  milestones:
    - title: "Phase 1: Planning"
      status: "completed"
      date: "2024-Q1"
    - title: "Phase 2: Development"
      status: "in-progress"
      date: "2024-Q2"
```

## Related Documentation

- [FrontMatter Configuration](../language-reference/frontmatter.md) - Complete FrontMatter reference
- [Flex Mode Syntax](../language-reference/flex-mode.md) - Using variables in Flex mode
- [Strict Mode Syntax](../language-reference/strict-mode.md) - Using variables in Strict mode
- [Themes & Styling](themes-styling.md) - Theme integration with variables
- [Headers & Footers](headers-footers.md) - Using variables in headers and footers

## Examples

See the [Variables Examples](../../examples/01_title_and_content/) directory for complete working examples demonstrating:

- Basic variable usage
- Advanced templating
- Multi-language presentations
- Dynamic content generation
- Corporate report templates

---

**💡 Pro Tip:** Start with simple variables for repeated content, then gradually adopt advanced features like templates and computed values as your presentations become more complex.
