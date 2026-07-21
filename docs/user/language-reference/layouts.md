# 📐 Slide Layouts

ZiraDocs provides a comprehensive collection of predefined layouts that help you create professional presentations with optimal visual structure. Layouts control how content is arranged and displayed on each slide.

## 🎯 **Layout Overview**

Layouts are applied using the `@layout` directive or frontmatter and automatically organize your content according to proven design patterns.

### Quick Usage
```yaml
---
layout: hero
---
# Welcome to Our Product
## Revolutionary solutions for modern businesses
```

```slidelang
SLIDE intro
  layout: "testimonial"
  
  TEXT
    "This product changed our business completely."
    - Jane Smith, CEO
```

## 📋 **Available Layouts**

### 🎬 **Impact Layouts**

#### Hero Layout
**Best for:** Product launches, major announcements, opening slides
```yaml
layout: hero
```
- Full-screen impact design
- Large title with subtitle
- Optional background image/video
- Centered content with strong visual hierarchy
- Call-to-action button support

#### Testimonial Layout  
**Best for:** Customer feedback, social proof, case studies
```yaml
layout: testimonial
```
- Quote-focused design
- Large testimonial text
- Author attribution with photo
- Company logo integration
- Elegant typography emphasis

#### Call to Action Layout
**Best for:** Conversion slides, next steps, contact information
```yaml
layout: call_to_action
```
- Action-oriented design
- Prominent CTA button
- Supporting text and benefits
- Contact information display
- Urgency/scarcity messaging support

### 📊 **Business Layouts**

#### Stats Layout
**Best for:** KPIs, metrics, performance data
```yaml
layout: stats
```
- Grid-based metric display
- Large numbers with descriptions
- Progress indicators
- Comparative visualizations
- Color-coded performance indicators

#### Dashboard Layout
**Best for:** Real-time data, monitoring, analytics
```yaml
layout: dashboard
```
- Multi-panel arrangement
- Chart integration areas
- Status indicators
- Data table sections
- Real-time update support

#### Pricing Layout
**Best for:** Product pricing, plan comparisons, packages
```yaml
layout: pricing
```
- Side-by-side plan comparison
- Feature lists with checkmarks
- Pricing highlight sections
- Recommended plan emphasis
- CTA button integration

#### Comparison Layout
**Best for:** Feature comparisons, before/after, competitive analysis
```yaml
layout: comparison
```
- Two-column comparison structure
- Feature-by-feature breakdown
- Visual differentiators
- Advantage highlighting
- Decision-making support

### 🔧 **Technical Layouts**

#### Code Example Layout
**Best for:** API documentation, tutorials, technical demos
```yaml
layout: code_example
```
- Split-screen code and explanation
- Syntax highlighting optimization
- Copy-to-clipboard functionality
- Line number display
- Multiple code tabs support

#### Feature Showcase Layout
**Best for:** Product features, capability demonstrations
```yaml
layout: feature_showcase
```
- Feature-focused presentation
- Screenshot/demo integration
- Benefit callouts
- Feature list with icons
- Progressive disclosure

#### Process Layout
**Best for:** Workflows, step-by-step guides, methodologies
```yaml
layout: process
```
- Sequential step display
- Progress indicators
- Step descriptions
- Arrow/flow connections
- Timeline representation

### 🏢 **Corporate Layouts**

#### Team Layout
**Best for:** Team introductions, about us, organizational charts
```yaml
layout: team
```
- Grid-based team member display
- Photo integration
- Role and bio sections
- Social media links
- Organizational hierarchy

#### Timeline Layout
**Best for:** Company history, project milestones, roadmaps
```yaml
layout: timeline
```
- Chronological event display
- Milestone markers
- Date progression
- Event descriptions
- Visual timeline flow

#### Before/After Layout
**Best for:** Transformation stories, case studies, improvements
```yaml
layout: before_after
```
- Split comparison design
- Visual transformation display
- Metric improvements
- Story narrative structure
- Impact highlighting

## 🛠️ **Layout Customization**

### Using Layout with Content Types

#### With Text Elements
```yaml
---
layout: hero
---

# Main Title
## Subtitle

Regular content goes here and will be arranged according to the hero layout structure.
```

#### With Multiple Elements
```slidelang
SLIDE feature_demo
  layout: "feature_showcase"
  
  TEXT
    # New Feature: Smart Analytics
    Our latest feature provides intelligent insights.
    
  IMAGE
    src: ./screenshots/analytics-dashboard.png
    alt: Analytics Dashboard Screenshot
    
  POINTS
    - Real-time data processing
    - Automated insights generation
    - Custom report building
```

#### With Interactive Elements
```yaml
---
layout: testimonial
---

> "This solution transformed our workflow completely. We saw 40% improvement in productivity within the first month."

<<poll>>
question: Have you experienced similar improvements?
options: ["Yes, significant", "Some improvement", "Not yet", "Need more info"]
<</poll>>
```

### Layout Properties

Some layouts accept additional configuration:

```yaml
---
layout: pricing
layout_config:
  columns: 3
  highlight_plan: "pro"
  show_annual_toggle: true
---
```

```slidelang
SLIDE team_intro
  layout: "team"
  layout_config: |
    {
      "grid_columns": 4,
      "show_social_links": true,
      "compact_mode": false
    }
```

## 📱 **Responsive Behavior**

All layouts automatically adapt to different screen sizes:

### Desktop (>1024px)
- Full layout structure displayed
- Optimal spacing and typography
- Complete feature set

### Tablet (768px - 1024px)
- Adjusted grid columns
- Modified spacing
- Touch-optimized interactions

### Mobile (<768px)
- Single-column layouts
- Simplified navigation
- Thumb-friendly buttons
- Condensed content display

## 🎨 **Layout with Themes**

Layouts work seamlessly with the theming system:

```json
{
  "name": "corporate-theme",
  "variables": {
    // Layout-specific variables
    "--slidelang-layout-hero-bg": "linear-gradient(135deg, #667eea 0%, #764ba2 100%)",
    "--slidelang-layout-stats-accent": "#06b6d4",
    "--slidelang-layout-comparison-border": "#e5e7eb"
  }
}
```

### Layout-Specific CSS Classes

Each layout adds specific CSS classes for targeted styling:

```css
/* Hero layout styling */
.slidelang-slide.slidelang-layout-hero {
  background: var(--slidelang-layout-hero-bg);
  justify-content: center;
  text-align: center;
}

.slidelang-layout-hero .hero-title {
  font-size: 3.5rem;
  font-weight: 700;
  margin-bottom: 1rem;
}

.slidelang-layout-hero .hero-subtitle {
  font-size: 1.5rem;
  opacity: 0.9;
  margin-bottom: 2rem;
}

/* Stats layout styling */
.slidelang-slide.slidelang-layout-stats .stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 2rem;
}

.slidelang-layout-stats .stat-item {
  text-align: center;
  padding: 1.5rem;
  background: rgba(255, 255, 255, 0.05);
  border-radius: var(--slidelang-border-radius);
}

/* Comparison layout styling */
.slidelang-slide.slidelang-layout-comparison .comparison-container {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 3rem;
  align-items: start;
}
```

## 🔧 **Advanced Layout Features**

### Layout Inheritance
```yaml
---
layout: hero
extends: corporate-base
layout_config:
  custom_background: true
  animation_style: "fade-in"
---
```

### Conditional Layouts
```yaml
---
layout: "{{ device == 'mobile' ? 'simple' : 'dashboard' }}"
variables:
  device: "desktop"
---
```

### Multi-Layout Slides
```slidelang
SLIDE complex_slide
  layout: "feature_showcase"
  
  SECTION intro
    layout: "hero"
    
    TEXT
      # Feature Introduction
      
  SECTION details  
    layout: "code_example"
    
    CODE
      language: javascript
      content: |
        const feature = new AdvancedFeature();
        feature.activate();
```

## 📊 **Layout Performance**

### Best Practices
1. **Choose appropriate layouts** for your content type
2. **Test responsive behavior** on different devices
3. **Optimize images** used in layout backgrounds
4. **Use layout caching** for repeated presentations
5. **Minimize custom CSS** overrides when possible

### Performance Metrics
- **Hero Layout**: Optimized for impact, ~150ms render time
- **Dashboard Layout**: Complex grid, ~300ms render time  
- **Simple Layouts**: Minimal overhead, ~50ms render time

## 🎯 **Layout Selection Guide**

| Content Type | Recommended Layout | Alternative |
|--------------|-------------------|-------------|
| **Opening slide** | `hero` | `call_to_action` |
| **Product features** | `feature_showcase` | `comparison` |
| **Team introduction** | `team` | `testimonial` |
| **Data presentation** | `stats` | `dashboard` |
| **Process explanation** | `process` | `timeline` |
| **Case study** | `before_after` | `testimonial` |
| **Pricing** | `pricing` | `comparison` |
| **Technical demo** | `code_example` | `feature_showcase` |
| **Call to action** | `call_to_action` | `hero` |

## 🔗 **Related Documentation**

- **[Themes & Styling](../features/themes-styling.md)** - Customizing layout appearance
- **[Variables & Templates](../features/variables-templates.md)** - Dynamic layout configuration
- **[Flex Mode Reference](flex-mode.md)** - Layout syntax in Flex mode
- **[Directives & Configuration](directives-configuration.md)** - Layout directive usage

---

**Next:** Learn how to create [custom layout templates](../theme-implementation/layout-creation.md) or explore [advanced layout techniques](../guides/advanced-layouts.md).
