# DocLang Flex Mode Syntax

**DocLang Flex Mode** uses extended Markdown syntax, familiar to most users. It's ideal for quickly creating professional documents with a natural writing flow.

## 🎯 Philosophy

Write like Markdown, get a professional document. DocLang Flex Mode interprets Markdown headings as the document hierarchy.

## 🔑 Key Concepts

| Markdown Element | DocLang Interpretation |
|-----------------|------------------------|
| `# Heading` | Level 1 section (H1) |
| `## Heading` | Level 2 section (H2) |
| `### Heading` | Level 3 section (H3) |
| `---` | Does **NOT** separate "pages" — only emphasis/section breaks |
| Paragraphs | Continuous flow of text |
| Lists | Bullet points or numbering |

## 📝 Fundamental Syntax

### Basic Structure

```doclang
---
mode: flex
doctype: document
title: "Technical Specification"
---

# 1. Introduction

This document describes the technical specifications for the system.

## 1.1 Purpose

The purpose of this specification is to provide comprehensive details about the architecture and implementation.

## 1.2 Scope

The scope includes all system components, their interactions, and deployment considerations.

---

# 2. Architecture

The system follows a microservices architecture pattern with the following key components:

- **API Gateway**: Entry point for all client requests
- **Authentication Service**: Handles user authentication and authorization
- **Data Service**: Manages data persistence and retrieval
- **Analytics Service**: Processes usage analytics

## 2.1 Component Details

Each component is described in detail below.

### 2.1.1 API Gateway

The API Gateway serves as the single entry point...
```

### Headings and Hierarchy

```doclang
# 1. Top Level Section (H1)

## 1.1 Second Level (H2)

### 1.1.1 Third Level (H3)

#### 1.1.1.1 Fourth Level (H4)

##### Fifth Level (H5)

###### Sixth Level (H6)
```

**Usage Guide:**
- **H1 (`#`)**: Main document sections (chapters)
- **H2 (`##`)**: Main subsections
- **H3 (`###`)**: Sub-subsections
- **H4-H6**: Additional levels of detail

### Text and Paragraphs

```doclang
# Introduction

This is a regular paragraph with **bold text**, *italic text*, 
`inline code`, and [hyperlinks](https://example.com).

You can use ==highlighted text== for emphasis, ~~strikethrough~~ 
for removed content, and ^superscript^ or ~subscript~ for 
scientific notation.

Multiple paragraphs are separated by blank lines.

This is a second paragraph with more content.
```

### Lists

**Unordered lists:**
```doclang
## Key Features

The system provides the following features:

- User authentication and authorization
- Real-time data synchronization
- Advanced analytics dashboard
  - Custom reports
  - Data visualization
  - Export capabilities
- Mobile-responsive interface
```

**Ordered lists:**
```doclang
## Installation Steps

Follow these steps to install the system:

1. Download the installation package
2. Extract files to the target directory
   a. Verify file integrity
   b. Check system requirements
3. Run the installation script
4. Configure the application
5. Start the service
```

**Task lists (checklists):**
```doclang
## Project Status

- [x] Requirements gathering completed
- [x] Design phase finished
- [x] Development in progress
  - [x] Backend API complete
  - [x] Frontend UI complete
  - [ ] Integration testing pending
- [ ] User acceptance testing
- [ ] Production deployment
```

### Code Blocks

````doclang
## API Implementation

The authentication endpoint is implemented as follows:

```javascript
// Authentication service
class AuthService {
  async authenticate(credentials) {
    try {
      const user = await this.validateCredentials(credentials);
      const token = this.generateToken(user);
      return { success: true, token };
    } catch (error) {
      return { success: false, error: error.message };
    }
  }
  
  validateCredentials({ username, password }) {
    // Validation logic
    return this.userRepository.findByCredentials(username, password);
  }
  
  generateToken(user) {
    return jwt.sign({ userId: user.id }, process.env.JWT_SECRET);
  }
}
```

The service uses JSON Web Tokens (JWT) for session management.
````

**Multiple examples with tabs:**

````doclang
## Code Examples

:::code-group

```javascript [JavaScript]
async function fetchData() {
  const response = await fetch('/api/data');
  return await response.json();
}
```

```python [Python]
import requests

def fetch_data():
    response = requests.get('/api/data')
    return response.json()
```

```go [Go]
func fetchData() ([]byte, error) {
    resp, err := http.Get("/api/data")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    return ioutil.ReadAll(resp.Body)
}
```

:::
````

### Tables

```doclang
## Performance Metrics

The following table shows performance under different load conditions:

| Scenario | Response Time | Throughput | Error Rate |
|----------|--------------|------------|------------|
| Light Load | 45ms | 1,000 req/s | 0.01% |
| Medium Load | 120ms | 2,500 req/s | 0.05% |
| Heavy Load | 340ms | 5,000 req/s | 0.15% |
| Peak Load | 780ms | 8,000 req/s | 0.50% |

*Table 1: Performance metrics under various load conditions*

## API Endpoints

| Endpoint | Method | Description | Auth Required |
|----------|--------|-------------|---------------|
| `/api/users` | GET | List all users | Yes |
| `/api/users/:id` | GET | Get user by ID | Yes |
| `/api/users` | POST | Create new user | Yes |
| `/api/auth/login` | POST | User login | No |
```

### Images

```doclang
## System Dashboard

The main dashboard provides a comprehensive overview of system status:

![Main Dashboard](assets/dashboard-screenshot.png "System dashboard interface")

*Figure 1: Main dashboard showing real-time metrics and alerts*

## Architecture Diagram

![System Architecture](assets/architecture-diagram.svg)

*Figure 2: High-level system architecture*
```

### Quotes (Blockquotes)

```doclang
## Design Principles

Our design follows key principles from industry experts:

> The best software is simple, maintainable, and does one thing well.
> Complexity is the enemy of reliability.
> 
> — Unix Philosophy

> Premature optimization is the root of all evil.
> 
> **— Donald Knuth**, *The Art of Computer Programming*

These principles guide our architectural decisions.
```

## 🎨 Special Blocks

### Information Blocks

```doclang
## Important Considerations

:::info
**Note**: All API endpoints require authentication using Bearer tokens.
Ensure your API key is kept secure and never committed to version control.
:::

:::warning
**Security Warning**: Never store passwords in plain text. Always use 
industry-standard hashing algorithms like bcrypt or Argon2.
:::

:::danger
**Critical**: Database migrations are irreversible. Always backup your 
database before running migrations in production environments.
:::

:::success
**Best Practice**: Use environment variables for all configuration values.
This allows for easy deployment across different environments without 
code changes.
:::

:::tip
**Pro Tip**: Enable request logging in development but be mindful of 
sensitive data in production logs. Consider using structured logging 
with different log levels.
:::
```

## 📊 Advanced Elements

### Interactive Charts

```doclang
## Revenue Analysis

Quarterly revenue growth over the past year:

<<chart: line>>
  data: [
    ["Q1 2024", 125000, 98000],
    ["Q2 2024", 145000, 112000],
    ["Q3 2024", 167000, 128000],
    ["Q4 2024", 189000, 145000]
  ]
  series: ["Revenue ($)", "Profit ($)"]
  options:
    responsive: true
    plugins:
      title:
        display: true
        text: "Quarterly Financial Performance"
      legend:
        position: "bottom"

*Chart 1: Revenue and profit trends Q1-Q4 2024*

## Market Share Distribution

<<chart: pie>>
  data: [
    ["Product A", 35],
    ["Product B", 28],
    ["Product C", 22],
    ["Product D", 15]
  ]
  options:
    responsive: true
    plugins:
      title:
        display: true
        text: "Market Share by Product"

*Chart 2: Product market share distribution*

## Performance Comparison

<<chart: bar>>
  data: [
    ["Response Time", 45, 120, 340],
    ["Throughput", 1000, 2500, 5000],
    ["Error Rate", 0.01, 0.05, 0.15]
  ]
  series: ["Light Load", "Medium Load", "Heavy Load"]
  options:
    responsive: true
    scales:
      y:
        beginAtZero: true

*Chart 3: Performance metrics across different load scenarios*
```

### Mermaid Diagrams

```doclang
## System Architecture

The following diagram illustrates the system architecture:

<<mermaid>>
  graph TB
    A[Client Application] --> B[API Gateway]
    B --> C[Auth Service]
    B --> D[Data Service]
    B --> E[Analytics Service]
    
    C --> F[(User DB)]
    D --> G[(Main DB)]
    E --> H[(Analytics DB)]
    
    B --> I[Cache Layer]
    I --> J[Redis Cluster]
    
    style A fill:#e1f5ff
    style B fill:#fff4e1
    style C fill:#ffe1e1
    style D fill:#ffe1e1
    style E fill:#ffe1e1

*Figure 3: Microservices architecture diagram*

## Authentication Flow

<<mermaid>>
  sequenceDiagram
    participant U as User
    participant C as Client
    participant G as API Gateway
    participant A as Auth Service
    participant D as Database
    
    U->>C: Enter credentials
    C->>G: POST /auth/login
    G->>A: Validate credentials
    A->>D: Query user
    D-->>A: User data
    A->>A: Generate JWT
    A-->>G: Return token
    G-->>C: Authentication response
    C-->>U: Login success

*Figure 4: User authentication sequence diagram*

## Deployment Pipeline

<<mermaid>>
  graph LR
    A[Source Code] --> B[Build]
    B --> C[Unit Tests]
    C --> D[Integration Tests]
    D --> E{Tests Pass?}
    E -->|Yes| F[Deploy to Staging]
    E -->|No| G[Notify Developer]
    F --> H[Smoke Tests]
    H --> I{Staging OK?}
    I -->|Yes| J[Deploy to Production]
    I -->|No| G

*Figure 5: Continuous deployment pipeline*
```

### Interactive Maps

```doclang
## Global Deployment

Our services are deployed across multiple geographic regions:

<<map>>
  type: world
  markers:
    - lat: 37.7749
      lng: -122.4194
      label: "US West - San Francisco"
      value: 15420
      color: "#4285F4"
    - lat: 40.7128
      lng: -74.0060
      label: "US East - New York"
      value: 12890
      color: "#4285F4"
    - lat: 51.5074
      lng: -0.1278
      label: "Europe - London"
      value: 8930
      color: "#34A853"
    - lat: 35.6762
      lng: 139.6503
      label: "Asia Pacific - Tokyo"
      value: 6780
      color: "#FBBC04"
  zoom: 2
  center:
    lat: 30
    lng: 0

*Map 1: Global server distribution and active users*
```

## 🔗 DocLang-Specific Elements

### Table of Contents

```doclang
---
mode: flex
doctype: document
title: "Technical Manual"
toc:
  enabled: true
  depth: 3
---

# Table of Contents

<<toc>>

*The table of contents is generated automatically based on the document's headings*

---

# 1. Introduction

Content starts here...
```

### Cross-References

```doclang
## Cross-Referencing

For architectural details, see [Section 2: Architecture](#architecture).

Performance metrics are detailed in [Table 1](#table-performance).

The system diagram is shown in [Figure 3](#fig-architecture).

You can also use the shorthand notation: <<ref: architecture>>
```

### Footnotes

```doclang
## Research Methodology

Our approach follows established software engineering practices[^1] and 
is informed by recent research in distributed systems[^2].

The implementation uses industry-standard protocols[^3].

[^1]: Smith, J. et al. (2023). "Modern Software Architecture Patterns". 
      Journal of Software Engineering, 45(2), 123-145.

[^2]: Johnson, M. (2024). "Distributed Systems Design". Tech Press.

[^3]: RFC 7519 - JSON Web Token (JWT) standard.
```

### Page Breaks

```doclang
# Executive Summary

Summary content here...

<<pagebreak>>

# Detailed Analysis

This section starts on a new page...
```

### Term Index

```doclang
# Glossary

<<index>>
  sort: alphabetical
  style: detailed

The glossary will be auto-generated from key terms marked in the text.
```

### Bibliography

```doclang
# References

<<bibliography>>
  style: apa
  sort: author

The bibliography is auto-generated from citations throughout the document.
```

## 📋 Complete FrontMatter for Flex Mode

```yaml
---
# Basic configuration
mode: flex
doctype: document

# Metadata
title: "API Integration Guide"
subtitle: "Developer Documentation"
author: "Platform Team"
authors:
  - name: "Alice Johnson"
    role: "API Architect"
    email: "alice@example.com"
  - name: "Bob Smith"
    role: "Technical Writer"
    email: "bob@example.com"
date: "October 8, 2024"
version: "3.1.0"
status: "Published"

# Output
output:
  format: [html, pdf, docx]
  path: "./docs/output"
  filename: "api-integration-guide-v3.1"

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
  even_pages: "{{section_title}}"
  logo:
    src: "./assets/logo.png"
    height: "30px"
    position: "left"
  style: "professional"
  divider: true

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
  title: "Contents"
  page_numbers: true
  hyperlinks: true
  position: "before-content"

# Numbering
numbering:
  enabled: true
  style: "hierarchical"
  prefix: ""
  suffix: "."
  sections: true
  figures: true
  tables: true
  charts: true

# References and citations
references:
  style: "apa"
  footnotes:
    enabled: true
    position: "bottom"
  
# Theme
theme: "technical-documentation"

# Custom variables
variables:
  company: "TechCorp Inc."
  product: "CloudAPI Platform"
  api_version: "3.1"
  base_url: "https://api.techcorp.com/v3"
  confidentiality: "Public Documentation"
  support_email: "support@techcorp.com"

# Advanced features
features:
  search: true
  print_friendly: true
  responsive: true
  dark_mode: false
---
```

## 📚 Complete Document Example

```doclang
---
mode: flex
doctype: document
title: "CloudAPI Integration Guide"
author: "Platform Engineering Team"
version: "3.1.0"
theme: "technical-documentation"
toc:
  enabled: true
  depth: 3
variables:
  api_version: "3.1"
  base_url: "https://api.example.com/v3"
---

# Introduction

Welcome to the **CloudAPI Integration Guide** version {{api_version}}. This comprehensive guide provides everything you need to successfully integrate with our API platform.

## About This Guide

This guide covers:

- Getting started with API authentication
- Core API concepts and endpoints
- Code examples in multiple languages
- Best practices and common patterns
- Troubleshooting and error handling

## Prerequisites

Before you begin, ensure you have:

- [x] An active CloudAPI account
- [x] API credentials (API key and secret)
- [ ] Basic understanding of REST APIs
- [ ] Development environment set up

:::info
**Need help?** Contact our support team at support@example.com or visit our developer forum at forum.example.com
:::

---

# Getting Started

## Creating Your Account

Visit [developer.example.com](https://developer.example.com) to create your account.

## Obtaining API Credentials

1. Log in to the developer portal
2. Navigate to **Settings** > **API Keys**
3. Click **Generate New Key**
4. Store your credentials securely

:::warning
**Security Notice**: Never expose your API secret in client-side code or public repositories. Use environment variables and keep credentials secure.
:::

## Making Your First Request

Here's a simple example to verify your credentials:

:::code-group

```javascript [JavaScript]
const API_KEY = process.env.CLOUDAPI_KEY;
const BASE_URL = '{{base_url}}';

async function testConnection() {
  const response = await fetch(`${BASE_URL}/status`, {
    headers: {
      'Authorization': `Bearer ${API_KEY}`,
      'Content-Type': 'application/json'
    }
  });
  
  const data = await response.json();
  console.log('Connection successful:', data);
}

testConnection();
```

```python [Python]
import os
import requests

API_KEY = os.getenv('CLOUDAPI_KEY')
BASE_URL = '{{base_url}}'

def test_connection():
    response = requests.get(
        f'{BASE_URL}/status',
        headers={
            'Authorization': f'Bearer {API_KEY}',
            'Content-Type': 'application/json'
        }
    )
    
    data = response.json()
    print('Connection successful:', data)

test_connection()
```

```go [Go]
package main

import (
    "fmt"
    "net/http"
    "os"
)

func main() {
    apiKey := os.Getenv("CLOUDAPI_KEY")
    baseURL := "{{base_url}}"
    
    req, _ := http.NewRequest("GET", baseURL+"/status", nil)
    req.Header.Set("Authorization", "Bearer "+apiKey)
    req.Header.Set("Content-Type", "application/json")
    
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()
    
    fmt.Println("Connection successful")
}
```

:::

---

# Core Concepts

## Authentication

All API requests require authentication using Bearer tokens[^1].

### Token Types

| Token Type | Use Case | Expiration |
|------------|----------|------------|
| API Key | Server-to-server | Never (until revoked) |
| Access Token | User authentication | 1 hour |
| Refresh Token | Token renewal | 30 days |

*Table 1: Authentication token types and their characteristics*

## Rate Limiting

The API implements tiered rate limiting:

<<chart: bar>>
  data: [
    ["Free", 100],
    ["Starter", 1000],
    ["Professional", 10000],
    ["Enterprise", 100000]
  ]
  series: ["Requests per hour"]
  options:
    responsive: true
    plugins:
      title:
        display: true
        text: "Rate Limits by Plan Tier"

*Chart 1: API rate limits across different subscription tiers*

## API Architecture

<<mermaid>>
  graph TB
    A[Client Application] --> B[Load Balancer]
    B --> C[API Gateway]
    C --> D{Authentication}
    D -->|Valid| E[Service Layer]
    D -->|Invalid| F[401 Error]
    E --> G[Data Service]
    E --> H[Analytics Service]
    G --> I[(Primary DB)]
    H --> J[(Analytics DB)]
    
    style D fill:#ffe1e1
    style E fill:#e1ffe1
    style F fill:#ff0000,color:#fff

*Figure 1: API architecture and request flow*

---

# API Reference

## Endpoints

### User Management

#### List Users

```http
GET {{base_url}}/users
```

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `page` | integer | No | Page number (default: 1) |
| `limit` | integer | No | Results per page (default: 20, max: 100) |
| `sort` | string | No | Sort field (default: 'created_at') |

**Response:**

```json
{
  "data": [
    {
      "id": "usr_123",
      "email": "user@example.com",
      "name": "John Doe",
      "created_at": "2024-10-08T10:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "per_page": 20,
    "total": 150
  }
}
```

## Error Handling

The API uses standard HTTP status codes:

| Status Code | Meaning | Description |
|-------------|---------|-------------|
| 200 | OK | Request successful |
| 201 | Created | Resource created successfully |
| 400 | Bad Request | Invalid request format or parameters |
| 401 | Unauthorized | Invalid or missing authentication |
| 403 | Forbidden | Insufficient permissions |
| 404 | Not Found | Resource does not exist |
| 429 | Too Many Requests | Rate limit exceeded |
| 500 | Internal Server Error | Server-side error |

*Table 2: HTTP status codes and their meanings*

:::tip
**Best Practice**: Always implement proper error handling in your application. Check status codes and handle errors gracefully with user-friendly messages.
:::

---

# Best Practices

## Security

1. **Use HTTPS**: Always use HTTPS for API requests
2. **Rotate Keys**: Regularly rotate API keys
3. **Limit Scope**: Use the minimum required permissions
4. **Monitor Usage**: Track API usage for anomalies

## Performance

- Implement caching where appropriate
- Use pagination for large datasets
- Batch requests when possible
- Implement retry logic with exponential backoff

## Error Recovery

```javascript
async function apiCallWithRetry(url, options, maxRetries = 3) {
  for (let i = 0; i < maxRetries; i++) {
    try {
      const response = await fetch(url, options);
      
      if (response.ok) {
        return await response.json();
      }
      
      if (response.status === 429) {
        // Rate limit - wait and retry
        const retryAfter = response.headers.get('Retry-After') || (2 ** i);
        await sleep(retryAfter * 1000);
        continue;
      }
      
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    } catch (error) {
      if (i === maxRetries - 1) throw error;
      await sleep(2 ** i * 1000); // Exponential backoff
    }
  }
}
```

---

# Support and Resources

## Getting Help

- **Documentation**: [docs.example.com](https://docs.example.com)
- **API Status**: [status.example.com](https://status.example.com)
- **Support Email**: {{support_email}}
- **Developer Forum**: [forum.example.com](https://forum.example.com)

## Additional Resources

- [API Changelog](https://docs.example.com/changelog)
- [Migration Guides](https://docs.example.com/migration)
- [Code Examples Repository](https://github.com/example/api-examples)
- [Postman Collection](https://example.com/postman)

---

# Appendix

## Glossary

**API (Application Programming Interface)**
: A set of protocols and tools for building software applications

**Bearer Token**
: An authentication token sent in the Authorization header

**Rate Limiting**
: Restriction on the number of API requests in a time period

**REST (Representational State Transfer)**
: An architectural style for distributed systems

## References

<<bibliography>>

[^1]: RFC 6750 - The OAuth 2.0 Authorization Framework: Bearer Token Usage

---

*Document generated with DocLang v{{api_version}}*
*Last updated: {{date}}*
```

## 🎯 Best Practices

1. **Use semantic headings** - Create logical document hierarchy
2. **Write descriptive section titles** - Clear titles improve navigation
3. **Break long sections** - Use subsections for better readability
4. **Add visual elements** - Charts, diagrams, and tables enhance understanding
5. **Use info blocks** - Highlight important information with colored blocks
6. **Include code examples** - Provide practical examples in multiple languages
7. **Cross-reference effectively** - Link related sections
8. **Caption visual elements** - Add meaningful captions to figures and tables

## 🔄 Migrating to Strict Mode

If you need more control, you can migrate to Strict Mode:

| Flex Mode | Strict Mode |
|-----------|-------------|
| `# Section` | `SECTION "Section" level: 1` |
| `## Subsection` | `SECTION "Subsection" level: 2` |
| Text paragraph | `TEXT` block |
| `- List` | `POINTS` block |
| ` ```code``` ` | `CODE` block |
| `![img](src)` | `IMAGE` block |

## 📖 See Also

- [DocLang Overview](DOCLANG_OVERVIEW.md)
- [DocLang Strict Mode](DOCLANG_SYNTAX_STRICT.md)
- [ZiraDocs Flex Mode](../user/language-reference/flex-mode.md)
- [DocLang FrontMatter](DOCLANG_FRONTMATTER.md)
- [DocLang Elements Reference](DOCLANG_ELEMENTS.md)
