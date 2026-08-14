# Specialized Layouts - Complete Examples

This folder contains demonstrative examples of all **18 specialized layouts** in slidelang, documented in the [Specialized Layouts Guide](https://ziradocs.com/docs/slidelang/language-reference/specialized-layouts/).

## 📋 Example Structure

### 🎯 Main File
- **`product_launch_presentation_flex.slidelang`** - Complete product launch presentation that uses multiple specialized layouts in a real-world scenario.

### 🔧 Specialized Examples
- **`18.1_code_examples_flex.slidelang`** - Focused on the `code_example` layout for technical documentation
- **`18.2_comparison_analysis_flex.slidelang`** - Extensive use of the `comparison` layout for competitive analysis  
- **`18.3_testimonials_success_flex.slidelang`** - Success stories using the `testimonial`, `before_after`, and `stats` layouts
- **`18.4_dashboard_metrics_flex.slidelang`** - Executive dashboards with the `dashboard` and `stats` layouts

## 🎨 Layouts Demonstrated

| Layout | Main File | Specialized | Purpose |
|--------|------------------|----------------|-----------|
| `title` | ✅ | ✅ ✅ ✅ ✅ | Covers and main titles |
| `hero` | ✅ | ✅ | High-impact visual slides |
| `section` | ✅ | ✅ ✅ | Section introductions |
| `stats` | ✅ | ✅ ✅ ✅ | Data and metrics presentation |
| `comparison` | ✅ | ✅ | Side-by-side comparisons |
| `testimonial` | ✅ | ✅ | Success stories and testimonials |
| `before_after` | ✅ | ✅ ✅ | Transformations and results |
| `feature_showcase` | ✅ | ✅ | Product features |
| `pricing` | ✅ | | Pricing plans and options |
| `team` | ✅ | ✅ | Team presentation |
| `timeline` | ✅ | ✅ | Timelines and roadmaps |
| `process` | ✅ | ✅ ✅ | Methodologies and workflows |
| `call_to_action` | ✅ | ✅ ✅ ✅ | Calls to action |
| `dashboard` | ✅ | ✅ | Dashboards and metrics |
| `code_example` | | ✅ | Technical documentation |
| `content` | ✅ | ✅ | General content |
| `default` | | | Default layout |

## 🚀 How to Use These Examples

### 1. **Run the Linter**
```bash
# Verify layout validations
./slidelang lint examples/18_specialized_layouts/
```

### 2. **Generate HTML**
```bash
# Generate HTML presentation
./slidelang build examples/18_specialized_layouts/product_launch_presentation_flex.slidelang
```

### 3. **Development Mode**
```bash
# Serve with hot reload
./slidelang serve examples/18_specialized_layouts/product_launch_presentation_flex.slidelang
```

## 📖 Usage Scenarios

### 🎯 Product Launch Presentation
**File:** `product_launch_presentation_flex.slidelang`

Demonstrates a complete flow from the announcement to the call-to-action:
- `title` → Impactful cover
- `hero` → Emotional introduction  
- `stats` → Data backing the product
- `feature_showcase` → Main features
- `comparison` → Competitive advantages
- `testimonial` → Social validation
- `pricing` → Commercial options
- `call_to_action` → Final conversion

### 💻 API Documentation
**File:** `18.1_code_examples_flex.slidelang`

Perfect for technical documentation:
- Dominant `code_example` with multiple languages
- `comparison` between SDKs and APIs
- `section` to organize complex topics

### 📊 Competitive Analysis
**File:** `18.2_comparison_analysis_flex.slidelang`

Comprehensive market analysis:
- `comparison` for features and pricing
- `stats` with market metrics
- `before_after` showing migrations
- `testimonial` validating decisions

### 🏆 Success Stories
**File:** `18.3_testimonials_success_flex.slidelang`

Storytelling with measurable results:
- `testimonial` as the central element
- `before_after` to show transformations
- `process` explaining methodology
- `team` presenting the people behind it

### 📈 Executive Dashboard
**File:** `18.4_dashboard_metrics_flex.slidelang`

Advanced business reporting:
- `dashboard` with real-time metrics
- `stats` for historical comparisons
- `timeline` with future projections

## 🎨 Customization

### Suggested Themes
```yaml
# In frontmatter
theme: "modern-blue"    # For corporate presentations
theme: "code-dark"      # For technical documentation  
theme: "professional"   # For analysis and reports
theme: "testimonial"    # For success stories
theme: "dashboard"      # For metrics and analytics
```

### Common Variables
```yaml
# Reusable variables
company_name: "TechFlow"
product_name: "TechFlow Pro"  
contact_email: "hello@techflow.com"
brand_color: "#0066CC"
```

## ✅ Automatic Validations

The linter automatically verifies:

- ✅ `title` slides have a title or heading
- ✅ `comparison` slides have at least 2 elements
- ✅ `stats` slides include tabular data or charts  
- ✅ `code_example` slides contain code blocks
- ✅ `testimonial` slides include quotes and authors
- ✅ `timeline` slides have at least 2 events
- ✅ `pricing` slides include prices
- ✅ And all other documented validations

## 🔗 References

- [Complete Layouts Documentation](https://ziradocs.com/docs/slidelang/language-reference/specialized-layouts/)
- [Flex Mode Syntax](https://ziradocs.com/docs/slidelang/language-reference/flex-mode/) 
- [Themes Guide](https://ziradocs.com/docs/slidelang/features/themes-styling/)
- [Use Cases](https://ziradocs.com/docs/slidelang/features/)

## 💡 Tips for Better Results

### 1. **Combine Layouts Strategically**
- Start with `title` or `hero` for impact
- Use `section` for clear transitions
- Intersperse `stats` and `testimonial` for credibility
- End with `call_to_action` for conversion

### 2. **Maintain Visual Consistency**
- Use the same theme throughout the presentation
- Define variables for reusable colors and text
- Keep a consistent style across images

### 3. **Optimize for Your Audience**
- `code_example` for developers
- `dashboard` for executives
- `testimonial` for sales
- `comparison` for decision-makers

### 4. **Validate Regularly**
- Run the linter frequently
- Verify that each layout fulfills its purpose
- Test the presentation on different devices

---

**Explore, experiment, and create impactful presentations with slidelang!** 🚀
