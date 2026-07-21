# Special Blocks

Special blocks are powerful elements in ZiraDocs that allow you to highlight information, group content, create layouts, and add interactive elements. They work in both [Strict Mode](strict-mode.md) and [Flex Mode](flex-mode.md) and help you organize content with visual emphasis and structured layouts.

## Information Blocks

Use information blocks to draw attention to specific types of content with automatic styling and icons.

### Basic Information Types

```markdown
:::info
💡 **Important Information**
This is an informational block with an icon and special styling.
Use it for helpful tips, explanations, or additional context.
:::

:::warning
⚠️ **Warning**
Be careful with this point. Warnings indicate situations that could 
cause problems if not handled correctly.
:::

:::danger
🚨 **Critical Alert**
This is critical information that requires immediate attention.
Use for actions that could result in data loss or system failures.
:::

:::success
✅ **Success**
Excellent! This action completed successfully. Use to confirm
that operations worked as expected.
:::

:::tip
💡 **Pro Tip**
Useful tricks to improve your workflow. Tips provide advice
and best practices for optimizing system usage.
:::
```

### Extended Information Types

ZiraDocs supports additional block types for comprehensive documentation:

```markdown
:::note
📝 **Note**
General notes and observations that provide additional context
without requiring immediate action.
:::

:::error
❌ **Error**
Indicates errors, failures, or issues that need to be resolved.
Use when documenting common problems and their solutions.
:::

:::example
📋 **Example**
Code examples, use cases, or practical demonstrations.
Perfect for tutorials and documentation.
:::
```

## Collapsible Sections

Create expandable content sections that help organize information without overwhelming the reader.

### Basic Collapsible

```markdown
:::details Summary Title
Hidden content that is revealed when clicked.
Can contain **formatting**, lists, code blocks, and other elements.

- Bullet points work fine
- Multiple paragraphs supported
- Any markdown content

```code
// Even code blocks work inside
function example() {
  return "Hello World";
}
```
:::
```

### Advanced Collapsible with Styling

```markdown
:::details Technical Implementation 🔧
**Prerequisites:**
- Node.js 16+ installed
- Git access configured
- Development environment ready

**Step-by-step process:**

1. Clone the repository
2. Install dependencies with `npm install`
3. Configure environment variables
4. Run `npm start` to begin development

**Common issues:**
- Port conflicts → Use `PORT=3001 npm start`
- Permission errors → Run with `sudo` on Unix systems
:::
```

## Code Groups

Display multiple code examples with interactive tabs, perfect for multi-language documentation.

### Basic Code Group

```markdown
:::code-group

```javascript [JavaScript]
const response = await fetch('/api/users');
const users = await response.json();
console.log(users);
```

```python [Python]
import requests

response = requests.get('/api/users')
users = response.json()
print(users)
```

```curl [cURL]
curl -X GET https://api.example.com/users \
  -H "Authorization: Bearer your-token"
```

:::
```

### Extended Code Group

```markdown
:::code-group

```typescript [TypeScript]
interface User {
  id: number;
  name: string;
  email: string;
}

async function getUsers(): Promise<User[]> {
  const response = await fetch('/api/users');
  return response.json();
}
```

```go [Go]
type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

func getUsers() ([]User, error) {
    resp, err := http.Get("/api/users")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var users []User
    err = json.NewDecoder(resp.Body).Decode(&users)
    return users, err
}
```

```rust [Rust]
#[derive(Deserialize)]
struct User {
    id: u32,
    name: String,
    email: String,
}

async fn get_users() -> Result<Vec<User>, reqwest::Error> {
    let users: Vec<User> = reqwest::get("/api/users")
        .await?
        .json()
        .await?;
    Ok(users)
}
```

:::
```

## Presenter Notes

Add speaker notes that are only visible in presenter mode:

```markdown
:::notes
These notes will appear in the presenter notes panel when you press 'N' 
or use the floating menu. Great for speaker talking points and reminders.

- Key statistics to mention
- Transition to next slide
- Q&A preparation points
:::
```

**Multiple notes per slide:**
```markdown
:::notes
First set of notes about the introduction.
:::

## Main Content Here

More slide content...

:::notes
Additional notes about the conclusion and next steps.
:::
```

## Interactive Elements

### Checklists and Task Lists

```markdown
:::checklist
**Project Setup Tasks:**

- [x] Repository created and configured
- [x] Development environment set up
- [ ] Database schema designed
- [ ] API endpoints defined
- [ ] Frontend components built
- [ ] Testing suite implemented
:::
```

### Polls and Surveys

```markdown
:::poll
**What's your experience level with ZiraDocs?**

- [ ] Beginner - Just getting started
- [ ] Intermediate - Used for several presentations
- [ ] Advanced - Regular user with complex needs
- [ ] Expert - Contributing to the project
:::
```

## Quote Blocks

Enhanced quote formatting for testimonials and citations:

```markdown
:::quote
> "ZiraDocs has revolutionized how we create presentations. 
> The learning curve is minimal, but the possibilities are endless."
> 
> **— Alex Chen, Technical Writer**
:::

:::testimonial
> "The special blocks feature makes documentation so much clearer.
> Our team adoption rate increased by 300% after switching to ZiraDocs."
> 
> **— Dr. Sarah Martinez, Developer Experience Lead**
> *Microsoft Azure Team*
:::
```

## Grid and Column Layouts

Create organized content layouts with automatic responsive behavior using grid and column blocks:

### Basic Two-Column Layout

```markdown
::: grid
::: column
**Left Column Content:**
- First item
- Second item
- Third item
:::

::: column
**Right Column Content:**
- Different content
- Comparison data
- Related information
:::
:::
```

### Multiple Columns

```markdown
::: grid
::: column
**Step 1**
Initial setup and configuration
:::

::: column
**Step 2**
Development and testing
:::

::: column
**Step 3**
Deployment and monitoring
:::
:::
```

### Before/After Comparisons

```markdown
::: grid
::: column
**❌ Before Implementation:**
- Manual processes
- Inconsistent results
- Time-consuming tasks
- High error rates
:::

::: column
**✅ After Implementation:**
- Automated workflows
- Standardized outputs
- Efficient operations
- Improved accuracy
:::
:::
```

### Mixed Content Types

```markdown
::: grid
::: column
**Key Features:**
- Real-time collaboration
- Version control
- Export options

:::info
💡 **Pro Tip**
Use keyboard shortcuts for faster navigation.
:::
:::

::: column
**System Requirements:**
- Modern web browser
- Internet connection
- 4GB RAM minimum

:::warning
⚠️ **Compatibility**
Some features require JavaScript enabled.
:::
:::
:::
```

**Grid Features:**
- **Responsive Design:** Automatically collapses to single column on mobile devices
- **Equal Width Columns:** Columns automatically share available space equally
- **Nested Content:** Support for any content type within columns (text, lists, images, other special blocks)
- **CSS Grid Implementation:** Uses modern CSS Grid for optimal performance and flexibility

## Mode-Specific Usage

### Strict Mode
In [Strict Mode](strict-mode.md), special blocks are used within slide content:

```slidelang
SLIDE content
  title: "Important Information"
  
  :::info
  💡 **Setup Instructions**
  Follow these steps to configure your environment.
  :::
  
  TEXT
    Additional content after the info block.
```

### Flex Mode
In [Flex Mode](flex-mode.md), use special blocks directly in Markdown:

```markdown
# Important Information

:::info
💡 **Setup Instructions**
Follow these steps to configure your environment.
:::

Additional content after the info block.
```

## Styling and Customization

### Custom Icons and Colors

```markdown
:::info "🔧 Custom Setup"
You can customize the icon and title of any block.
:::

:::warning "⚡ Performance Impact"
This operation may affect system performance.
:::

:::tip "🚀 Speed Boost"
Pro tip for faster development workflow.
:::
```

### Block Combinations

```markdown
:::info
📋 **Complete Example**

This block contains multiple elements:

:::code-group

```bash [Setup]
go install go.ziradocs.com/slidelang/cmd/slidelang@latest
slidelang build my-presentation.slidelang
```

```yaml [Config]
---
title: "My Presentation"
mode: flex
theme: professional
---
```

:::

**Next steps:**
1. Edit your `.slidelang` file
2. Run `slidelang build`
3. Open the generated HTML file
:::
```

## Advanced Features

### Nested Blocks

```markdown
:::details Advanced Configuration
Basic setup is straightforward, but for advanced users:

:::warning
⚠️ **Expert Mode**
These settings can break your presentation if misconfigured.
:::

:::code-group

```yaml [Advanced]
---
mode: strict
advanced:
  parsing:
    strict_validation: true
    custom_elements: enabled
  rendering:
    optimize_performance: true
    lazy_loading: true
---
```

:::
:::
```

### Conditional Blocks

```markdown
:::info "📱 Mobile Users"
This content is optimized for mobile viewing.
Special considerations for smaller screens apply.
:::

:::warning "🖥️ Desktop Only"
This feature requires a desktop browser with JavaScript enabled.
:::
```

## Best Practices

1. **Use appropriate block types** - Match the block type to your content's purpose
2. **Don't overuse** - Too many special blocks can be distracting
3. **Keep content concise** - Blocks should highlight, not overwhelm
4. **Test interactivity** - Verify that collapsible and tabbed content works
5. **Consider accessibility** - Ensure content remains readable with screen readers

## Performance Considerations

- **Collapsible sections** - Large collapsed content doesn't affect initial render time
- **Code groups** - Multiple languages are loaded efficiently with tabs
- **Interactive elements** - Polls and checklists may require additional JavaScript
- **Nested blocks** - Deep nesting can impact compilation time

## Complete Example

Here's a comprehensive example showing multiple special block types:

```markdown
# API Documentation

:::info
📚 **Getting Started**
This guide covers the essential endpoints for our REST API.
All examples use modern async/await syntax.
:::

## Authentication

:::warning
🔐 **Security Notice**
Always use HTTPS in production and never expose API keys in client-side code.
:::

:::code-group

```javascript [Node.js]
const headers = {
  'Authorization': `Bearer ${process.env.API_TOKEN}`,
  'Content-Type': 'application/json'
};
```

```python [Python]
import os
headers = {
    'Authorization': f'Bearer {os.environ["API_TOKEN"]}',
    'Content-Type': 'application/json'
}
```

:::

## Implementation Checklist

:::checklist
**API Integration Steps:**

- [x] Obtain API credentials
- [x] Set up development environment  
- [ ] Implement authentication
- [ ] Test basic endpoints
- [ ] Add error handling
- [ ] Deploy to production
:::

:::details Advanced Error Handling 🔧
For production applications, implement comprehensive error handling:

- **Network errors** - Handle timeouts and connection issues
- **Rate limiting** - Implement exponential backoff
- **Authentication** - Refresh tokens automatically
- **Data validation** - Validate responses before processing

:::tip
💡 **Pro Tip**
Use a library like Axios or Fetch with automatic retries for better reliability.
:::
:::

:::success
🎉 **You're Ready!**
You now have all the tools needed to integrate with our API successfully.
:::
```

## Related Documentation

- [Strict Mode Syntax](strict-mode.md) - Using special blocks in structured syntax
- [Flex Mode Syntax](flex-mode.md) - Using special blocks in Markdown syntax
- [Inline Formatting](inline-formatting.md) - Text formatting within blocks
- [Advanced Elements](advanced-elements.md) - Charts, diagrams, and interactive content
- [Variables & Templates](../features/variables-templates.md) - Dynamic content in blocks

## Migration Notes

When migrating from other documentation tools:

- **GitBook/VuePress** - Most `:::` block syntax translates directly
- **DocuSaurus** - Admonition blocks map to info/warning/danger types
- **MkDocs** - Note/warning callouts become info/warning blocks
- **Standard Markdown** - Convert blockquotes to appropriate special block types
