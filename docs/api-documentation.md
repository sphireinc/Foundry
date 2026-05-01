---
title: API Documentation
slug: api-documentation
layout: page
draft: false
summary: Complete API reference for Foundry CMS including Admin API, Platform API, and Plugin APIs
date: 2026-04-27T00:00:00Z
author: admin
last_editor: admin
tags: ["api", "documentation", "reference"]
categories: ["developer"]
taxonomies: {}
workflow: published
---

# API Documentation

Foundry provides a comprehensive set of APIs for managing content, administering the system, and integrating with frontend applications. This documentation covers all available endpoints across three main API surfaces.

## API Surfaces

### Admin API
**Base URL:** `/{admin_path}/api` (default: `/__admin/api`)  
**Authentication:** Required (session-based)  
**Purpose:** Administrative operations including content management, user administration, system configuration, and monitoring.

### Platform API
**Base URL:** `/__foundry/api`  
**Authentication:** None (public)  
**Purpose:** Frontend SDK endpoints for content retrieval, search, and site information.

### Plugin APIs
**Base URL:** `/{admin_path}/plugin-api/{plugin_name}`  
**Authentication:** Required (session-based)  
**Purpose:** Plugin-specific functionality and extensions.

## Authentication

The Admin API and Plugin APIs require authentication via session cookies. Authentication is handled through the following endpoints:

### Login
```http
POST /{admin_path}/api/login
Content-Type: application/json

{
  "username": "admin",
  "password": "your_password",
  "totp_code": "123456"  // Optional, for MFA
}
```

**Response:**
```json
{
  "authenticated": true,
  "username": "admin",
  "name": "Administrator",
  "email": "admin@example.com",
  "role": "admin",
  "capabilities": ["dashboard.read", "documents.read", "config.manage"],
  "mfa_complete": true,
  "csrf_token": "abc123...",
  "ttl_seconds": 3600
}
```

### Session Management
```http
GET /{admin_path}/api/session
Cookie: session=your_session_token
```

**Response:**
```json
{
  "authenticated": true,
  "username": "admin",
  "name": "Administrator",
  "email": "admin@example.com",
  "role": "admin",
  "capabilities": ["dashboard.read", "documents.read", "config.manage"],
  "mfa_complete": true,
  "csrf_token": "abc123...",
  "ttl_seconds": 3600
}
```

## Admin API Reference

### Authentication Endpoints

#### POST `/login`
Authenticate and create a new session.
- **Public:** Yes
- **Request Body:**
  ```json
  {
    "username": "string",
    "password": "string",
    "totp_code": "string"  // Optional, for MFA
  }
  ```
- **Response:**
  ```json
  {
    "authenticated": true,
    "username": "string",
    "name": "string",
    "email": "string",
    "role": "string",
    "capabilities": ["string"],
    "mfa_complete": true,
    "csrf_token": "string",
    "ttl_seconds": 3600
  }
  ```

#### POST `/logout`
Destroy the current session.
- **Public:** Yes
- **Response:**
  ```json
  {
    "authenticated": false,
    "username": "",
    "capabilities": []
  }
  ```

#### GET `/session`
Get current session information.
- **Capabilities:** None (authenticated users only)
- **Response:**
  ```json
  {
    "authenticated": true,
    "username": "string",
    "name": "string",
    "email": "string",
    "role": "string",
    "capabilities": ["string"],
    "mfa_complete": true,
    "csrf_token": "string",
    "ttl_seconds": 3600
  }
  ```

#### GET `/sessions`
List all active sessions.
- **Capabilities:** `users.manage`
- **Query Parameters:** `username` (optional)
- **Response:**
  ```json
  [
    {
      "id": "string",
      "username": "string",
      "name": "string",
      "email": "string",
      "role": "string",
      "mfa_complete": true,
      "remote_addr": "192.168.1.1",
      "user_agent": "Mozilla/5.0...",
      "issued_at": "2026-04-27T10:00:00Z",
      "last_seen": "2026-04-27T10:30:00Z",
      "expires_at": "2026-04-27T11:00:00Z",
      "current": true
    }
  ]
  ```

#### POST `/sessions/revoke`
Revoke specific sessions.
- **Capabilities:** `users.manage`
- **Request Body:**
  ```json
  {
    "username": "string",  // Optional: revoke all sessions for user
    "session_id": "string", // Optional: revoke specific session
    "all": true             // Optional: revoke all sessions globally
  }
  ```
- **Response:**
  ```json
  {
    "revoked": 5
  }
  ```

### Multi-Factor Authentication

#### POST `/totp/setup`
Initialize TOTP setup for MFA.
- **Capabilities:** `dashboard.read`
- **Request Body:**
  ```json
  {
    "username": "string"  // Optional, defaults to current user
  }
  ```
- **Response:**
  ```json
  {
    "username": "admin",
    "secret": "JBSWY3DPEHPK3PXP",
    "provisioning_uri": "otpauth://totp/Foundry:admin?secret=JBSWY3DPEHPK3PXP&issuer=Foundry"
  }
  ```

#### POST `/totp/enable`
Enable TOTP after verification.
- **Capabilities:** `dashboard.read`
- **Request Body:**
  ```json
  {
    "username": "string",  // Optional, defaults to current user
    "code": "123456"       // TOTP code to verify
  }
  ```
- **Response:**
  ```json
  {
    "ok": true
  }
  ```

#### POST `/totp/disable`
Disable TOTP for a user.
- **Capabilities:** `dashboard.read`
- **Request Body:**
  ```json
  {
    "username": "string"  // Optional, defaults to current user
  }
  ```
- **Response:**
  ```json
  {
    "ok": true
  }
  ```

### System Status

#### GET `/status`
Get system status and health information.
- **Capabilities:** None (authenticated users only)
- **Response:**
  ```json
  {
    "version": "1.0.0",
    "build_time": "2026-04-27T00:00:00Z",
    "git_commit": "abc123...",
    "uptime_seconds": 3600,
    "memory_usage": {
      "allocated": "50MB",
      "system": "100MB"
    },
    "disk_usage": {
      "used": "2.5GB",
      "available": "10GB"
    },
    "database_status": "healthy"
  }
  ```

#### GET `/capabilities`
Get admin capabilities and available features.
- **Capabilities:** None
- **Response:**
  ```json
  {
    "sdk_version": "v1",
    "modules": {
      "session": true,
      "status": true,
      "documents": true,
      "media": true,
      "settings": true,
      "users": true,
      "themes": true,
      "plugins": true,
      "audit": true,
      "debug": false
    },
    "features": {
      "history": true,
      "trash": true,
      "diff": true,
      "document_locks": true,
      "workflow": true,
      "structured_editing": true,
      "plugin_admin_registry": true,
      "settings_sections": true,
      "pprof": false
    },
    "capabilities": ["dashboard.read", "documents.read"],
    "identity": {
      "authenticated": true,
      "username": "admin",
      "name": "Administrator",
      "email": "admin@example.com",
      "role": "admin",
      "capabilities": ["dashboard.read", "documents.read"],
      "mfa_complete": true
    }
  }
  ```

### Document Management

#### GET `/documents`
List documents with pagination and filtering.
- **Capabilities:** `dashboard.read`
- **Query Parameters:** `type`, `lang`, `status`, `page`, `limit`
- **Response:**
  ```json
  {
    "items": [
      {
        "id": "post-1",
        "type": "post",
        "lang": "en",
        "title": "Hello World",
        "slug": "hello-world",
        "url": "/posts/hello-world",
        "layout": "post",
        "summary": "My first blog post",
        "date": "2026-04-27T00:00:00Z",
        "author": "admin",
        "last_editor": "admin",
        "taxonomies": {
          "tags": ["introduction"],
          "categories": ["blog"]
        }
      }
    ],
    "page": 1,
    "page_size": 50,
    "total": 1
  }
  ```

#### GET `/document`
Get detailed document information.
- **Capabilities:** `dashboard.read`
- **Query Parameters:** `path` (required)
- **Response:**
  ```json
  {
    "id": "post-1",
    "type": "post",
    "lang": "en",
    "title": "Hello World",
    "slug": "hello-world",
    "url": "/posts/hello-world",
    "layout": "post",
    "summary": "My first blog post",
    "date": "2026-04-27T00:00:00Z",
    "author": "admin",
    "last_editor": "admin",
    "taxonomies": {
      "tags": ["introduction"],
      "categories": ["blog"]
    },
    "html_body": "<h1>Hello World</h1><p>This is my first post.</p>",
    "raw_body": "# Hello World\n\nThis is my first post.",
    "fields": {
      "custom_field": "value"
    },
    "params": {},
    "created_at": "2026-04-27T00:00:00Z",
    "updated_at": "2026-04-27T00:00:00Z"
  }
  ```

#### POST `/documents/create`
Create a new document.
- **Capabilities:** `dashboard.read`
- **Request Body:**
  ```json
  {
    "type": "post",
    "lang": "en",
    "title": "New Post",
    "slug": "new-post",
    "content": "# New Post\n\nContent here...",
    "frontmatter": {
      "draft": false,
      "tags": ["example"],
      "categories": ["blog"]
    }
  }
  ```
- **Response:** DocumentDetail (same as GET `/document`)

#### POST `/documents/save`
Save changes to an existing document.
- **Capabilities:** `dashboard.read`
- **Request Body:**
  ```json
  {
    "path": "content/posts/new-post.md",
    "content": "# Updated Post\n\nUpdated content...",
    "frontmatter": {
      "title": "Updated Post",
      "tags": ["updated"]
    }
  }
  ```
- **Response:** DocumentDetail

#### POST `/documents/lock`
Lock a document for exclusive editing.
- **Capabilities:** `dashboard.read`
- **Request Body:**
  ```json
  {
    "path": "content/posts/new-post.md"
  }
  ```
- **Response:**
  ```json
  {
    "ok": true
  }
  ```

#### POST `/documents/unlock`
Release a document lock.
- **Capabilities:** `dashboard.read`
- **Request Body:**
  ```json
  {
    "path": "content/posts/new-post.md"
  }
  ```
- **Response:**
  ```json
  {
    "ok": true
  }
  ```

#### GET `/documents/history`
Get document revision history.
- **Capabilities:** `dashboard.read`
- **Query Parameters:** `path` (required)
- **Response:**
  ```json
  [
    {
      "hash": "abc123...",
      "author": "admin",
      "message": "Initial commit",
      "timestamp": "2026-04-27T00:00:00Z"
    }
  ]
  ```

#### GET `/documents/trash`
List documents in trash.
- **Capabilities:** `dashboard.read`
- **Response:**
  ```json
  [
    {
      "path": "content/posts/deleted-post.md",
      "type": "post",
      "title": "Deleted Post",
      "deleted_at": "2026-04-27T00:00:00Z",
      "deleted_by": "admin"
    }
  ]
  ```

#### POST `/documents/restore`
Restore a document from trash.
- **Capabilities:** `dashboard.read`
- **Request Body:**
  ```json
  {
    "path": "content/posts/deleted-post.md"
  }
  ```
- **Response:**
  ```json
  {
    "ok": true
  }
  ```

#### POST `/documents/delete`
Move document to trash.
- **Capabilities:** `dashboard.read`
- **Request Body:**
  ```json
  {
    "path": "content/posts/old-post.md"
  }
  ```
- **Response:**
  ```json
  {
    "ok": true
  }
  ```

### Media Management

#### GET `/media`
List media files with pagination.
- **Capabilities:** `dashboard.read`
- **Query Parameters:** `page`, `limit`
- **Response:**
  ```json
  {
    "items": [
      {
        "path": "static/images/photo.jpg",
        "name": "photo.jpg",
        "size": 1024000,
        "type": "image/jpeg",
        "url": "/static/images/photo.jpg",
        "thumbnail_url": "/static/images/photo-thumb.jpg",
        "uploaded_at": "2026-04-27T00:00:00Z",
        "uploaded_by": "admin"
      }
    ],
    "page": 1,
    "page_size": 50,
    "total": 1
  }
  ```

#### POST `/media/upload`
Upload a new media file.
- **Capabilities:** `dashboard.read`
- **Content-Type:** `multipart/form-data`
- **Form Fields:**
  - `file`: The media file (binary)
  - `path`: Optional target path
- **Response:**
  ```json
  {
    "path": "static/images/uploaded.jpg",
    "name": "uploaded.jpg",
    "size": 512000,
    "type": "image/jpeg",
    "url": "/static/images/uploaded.jpg",
    "thumbnail_url": "/static/images/uploaded-thumb.jpg",
    "uploaded_at": "2026-04-27T00:00:00Z",
    "uploaded_by": "admin",
    "metadata": {},
    "exif": {}
  }
  ```

#### POST `/media/delete`
Move media file to trash.
- **Capabilities:** `dashboard.read`
- **Request Body:**
  ```json
  {
    "path": "static/images/old-photo.jpg"
  }
  ```
- **Response:**
  ```json
  {
    "ok": true
  }
  ```

### System Management

#### GET `/backups`
List available backups.
- **Capabilities:** `config.manage`
- **Response:**
  ```json
  [
    {
      "id": "backup-2026-04-27-001",
      "name": "Daily Backup",
      "size": 104857600,
      "created_at": "2026-04-27T02:00:00Z",
      "created_by": "admin",
      "type": "full"
    }
  ]
  ```

#### POST `/backups/create`
Create a new backup.
- **Capabilities:** `config.manage`
- **Request Body:**
  ```json
  {
    "name": "Manual Backup",
    "include_media": true,
    "include_database": true
  }
  ```
- **Response:**
  ```json
  {
    "id": "backup-2026-04-27-002",
    "name": "Manual Backup",
    "size": 0,
    "created_at": "2026-04-27T10:00:00Z",
    "created_by": "admin",
    "type": "full"
  }
  ```

#### GET `/operations`
Get operations status.
- **Capabilities:** `dashboard.read`
- **Response:**
  ```json
  {
    "status": "idle",
    "last_build": "2026-04-27T09:00:00Z",
    "build_duration": 30,
    "cache_size": 10485760,
    "queue_length": 0
  }
  ```

#### POST `/operations/rebuild`
Trigger site rebuild.
- **Capabilities:** `config.manage`
- **Response:**
  ```json
  {
    "ok": true
  }
  ```

### Settings Management

#### GET `/settings/sections`
Get available settings sections.
- **Capabilities:** `dashboard.read`
- **Response:**
  ```json
  [
    {
      "id": "site",
      "name": "Site Settings",
      "description": "General site configuration",
      "fields": [
        {
          "key": "title",
          "type": "text",
          "label": "Site Title",
          "description": "The title of your site",
          "default": "My Site",
          "required": true
        }
      ]
    }
  ]
  ```

#### GET `/settings/form`
Get settings form for a section.
- **Capabilities:** `config.manage`
- **Query Parameters:** `section` (required)
- **Response:**
  ```json
  {
    "section": "site",
    "fields": [
      {
        "key": "title",
        "type": "text",
        "label": "Site Title",
        "description": "The title of your site",
        "default": "My Site",
        "required": true
      }
    ],
    "values": {
      "title": "My Awesome Site"
    }
  }
  ```

#### POST `/settings/form/save`
Save settings for a section.
- **Capabilities:** `config.manage`
- **Request Body:**
  ```json
  {
    "section": "site",
    "values": {
      "title": "Updated Site Title",
      "description": "New site description"
    }
  }
  ```
- **Response:**
  ```json
  {
    "ok": true
  }
  ```

#### GET `/custom-fields`
Get custom fields configuration.
- **Capabilities:** `documents.read`
- **Response:** Custom fields object

#### POST `/custom-fields/save`
Save custom fields configuration.
- **Capabilities:** `config.manage`
- **Request Body:** Custom fields object
- **Response:** Success confirmation

### User Management

#### GET `/users`
List all users.
- **Capabilities:** `users.manage`
- **Response:**
  ```json
  [
    {
      "username": "admin",
      "name": "Administrator",
      "email": "admin@example.com",
      "role": "admin",
      "enabled": true,
      "last_login": "2026-04-27T09:00:00Z",
      "created_at": "2026-01-01T00:00:00Z"
    }
  ]
  ```

#### POST `/users/save`
Create or update a user.
- **Capabilities:** `users.manage`
- **Request Body:**
  ```json
  {
    "username": "newuser",
    "name": "New User",
    "email": "newuser@example.com",
    "role": "editor",
    "password": "securepassword123",
    "enabled": true
  }
  ```
- **Response:** UserInfo object

#### POST `/users/delete`
Delete a user.
- **Capabilities:** `users.manage`
- **Request Body:** DeleteUserRequest
- **Response:** Success confirmation

### Theme Management

#### GET `/themes`
List available themes.
- **Capabilities:** `themes.manage`
- **Response:** Array of ThemeInfo

#### POST `/themes/install`
Install a new theme.
- **Capabilities:** `themes.manage`
- **Request Body:** InstallThemeRequest
- **Response:** ThemeInfo

#### POST `/themes/switch`
Switch to a different theme.
- **Capabilities:** `themes.manage`
- **Request Body:** SwitchThemeRequest
- **Response:** Success confirmation

### Plugin Management

#### GET `/plugins`
List available plugins.
- **Capabilities:** `plugins.manage`
- **Response:**
  ```json
  [
    {
      "name": "aiwriter",
      "version": "1.0.0",
      "author": "Foundry Team",
      "description": "AI-powered content generation",
      "enabled": true,
      "builtin": false
    }
  ]
  ```

#### POST `/plugins/enable`
Enable a plugin.
- **Capabilities:** `plugins.manage`
- **Request Body:**
  ```json
  {
    "name": "aiwriter"
  }
  ```
- **Response:**
  ```json
  {
    "ok": true
  }
  ```

### Audit Logging

#### GET `/audit`
Get audit log entries.
- **Capabilities:** `audit.read`
- **Query Parameters:** `limit`, `offset`
- **Response:** Array of AuditEntry

## Platform API Reference

### Site Information

#### GET `/capabilities`
Get platform capabilities and features.
- **Public:** Yes
- **Response:**
  ```json
  {
    "sdk_version": "v1",
    "modules": {
      "session": true,
      "status": true,
      "documents": true,
      "media": true,
      "settings": true,
      "users": true,
      "themes": true,
      "plugins": true,
      "audit": true,
      "debug": false
    },
    "features": {
      "history": true,
      "trash": true,
      "diff": true,
      "document_locks": true,
      "workflow": true,
      "structured_editing": true,
      "plugin_admin_registry": true,
      "settings_sections": true,
      "pprof": false
    },
    "capabilities": ["dashboard.read", "documents.read"],
    "identity": {
      "authenticated": true,
      "username": "admin",
      "name": "Administrator",
      "email": "admin@example.com",
      "role": "admin",
      "capabilities": ["dashboard.read", "documents.read"],
      "mfa_complete": true
    }
  }
  ```

#### GET `/site`
Get basic site information.
- **Public:** Yes
- **Response:**
  ```json
  {
    "title": "My Foundry Site",
    "description": "A static site generator",
    "url": "https://example.com",
    "language": "en",
    "version": "1.0.0",
    "build_time": "2026-04-27T00:00:00Z"
  }
  ```

### Content Retrieval

#### GET `/content`
Get content by path or ID.
- **Public:** Yes
- **Query Parameters:** `path`, `id`, `type`, `lang`
- **Response:**
  ```json
  {
    "id": "post-1",
    "type": "post",
    "lang": "en",
    "title": "Hello World",
    "slug": "hello-world",
    "url": "/posts/hello-world",
    "layout": "post",
    "summary": "My first blog post",
    "date": "2026-04-27T00:00:00Z",
    "author": "admin",
    "last_editor": "admin",
    "taxonomies": {
      "tags": ["introduction"],
      "categories": ["blog"]
    },
    "html_body": "<h1>Hello World</h1><p>This is my first post.</p>",
    "raw_body": "# Hello World\n\nThis is my first post.",
    "fields": {
      "custom_field": "value"
    },
    "params": {},
    "created_at": "2026-04-27T00:00:00Z",
    "updated_at": "2026-04-27T00:00:00Z"
  }
  ```

#### GET `/collections`
Get paginated content collections.
- **Public:** Yes
- **Query Parameters:** `type`, `lang`, `taxonomy`, `page`, `page_size`, `sort`, `order`
- **Response:**
  ```json
  {
    "items": [
      {
        "id": "post-1",
        "type": "post",
        "lang": "en",
        "title": "Hello World",
        "slug": "hello-world",
        "url": "/posts/hello-world",
        "layout": "post",
        "summary": "My first blog post",
        "date": "2026-04-27T00:00:00Z",
        "author": "admin",
        "last_editor": "admin",
        "taxonomies": {
          "tags": ["introduction"],
          "categories": ["blog"]
        }
      }
    ],
    "page": 1,
    "page_size": 10,
    "total": 1,
    "has_more": false
  }
  ```

## Plugin APIs

### AI Writer Plugin

#### GET `/aiwriter/settings`
Get AI Writer plugin settings.
- **Capabilities:** `documents.create`
- **Response:**
  ```json
  {
    "enabled": true,
    "api_key_configured": true,
    "model": "gpt-4",
    "max_tokens": 2000,
    "temperature": 0.7
  }
  ```

#### POST `/aiwriter/generate`
Generate content using AI.
- **Capabilities:** `documents.create`
- **Request Body:**
  ```json
  {
    "prompt": "Write a blog post about static site generators",
    "type": "post",
    "title": "The Future of Static Sites",
    "max_length": 1000,
    "style": "professional"
  }
  ```
- **Response:**
  ```json
  {
    "content": "# The Future of Static Sites\n\nStatic site generators are revolutionizing web development...",
    "tokens_used": 450,
    "model": "gpt-4",
    "generated_at": "2026-04-27T10:00:00Z"
  }
  ```

## Code Examples

### JavaScript (Frontend SDK)

```javascript
// Initialize the Foundry SDK
const foundry = new FoundrySDK({
  baseURL: '/__foundry/api'
});

// Get site information
const siteInfo = await foundry.site.get();

// Search for content
const searchResults = await foundry.search.query({
  q: 'getting started',
  type: 'post'
});

// Get content by path
const content = await foundry.content.get({
  path: '/blog/hello-world'
});
```

### cURL Examples

```bash
# Login to admin API
curl -X POST http://localhost:8080/__admin/api/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password"}' \
  --cookie-jar cookies.txt

# Get documents list
curl -X GET http://localhost:8080/__admin/api/documents \
  --cookie cookies.txt

# Create a new document
curl -X POST http://localhost:8080/__admin/api/documents/create \
  -H "Content-Type: application/json" \
  -d '{
    "type": "post",
    "title": "My New Post",
    "content": "# Hello World\n\nThis is my first post."
  }' \
  --cookie cookies.txt
```

## Error Handling

All API endpoints return standard HTTP status codes:

- `200` - Success
- `400` - Bad Request (invalid input)
- `403` - Forbidden (authentication or authorization failure)
- `404` - Not Found
- `405` - Method Not Allowed
- `500` - Internal Server Error

Error responses include a JSON body with error details:

```json
{
  "error": "authentication_required",
  "message": "Admin login required"
}
```

## Rate Limiting

API endpoints may implement rate limiting. When rate limited, you'll receive a `429 Too Many Requests` response with a `Retry-After` header indicating when to retry.

## OpenAPI Specification

For detailed request/response schemas and interactive API documentation, see the complete [OpenAPI specification](api.yaml).

## SDKs and Libraries

- **Frontend SDK**: Available at `/__foundry/sdk/` for browser-based applications
- **Admin SDK**: Available at `/{admin_path}/theme/` for admin interface extensions

## Support

For questions about the API or integration issues, please refer to the [project documentation](/docs) or create an issue in the repository.