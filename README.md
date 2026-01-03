# Mattermost Link Archiver Plugin

[![Build Status](https://github.com/fmartingrmattermost-plugin-link-archiver/actions/workflows/ci.yml/badge.svg)](https://github.com/fmartingrmattermost-plugin-link-archiver/actions/workflows/ci.yml)

A Mattermost plugin that automatically archives links posted to channels. The plugin supports multiple archival methods configurable by hostname and MIME type rules, with intelligent deduplication and inline preview capabilities.

## Features

- **Automatic Link Archival**: Automatically detects and archives URLs posted in messages
- **Multiple Archival Tools**: Supports different archival methods for different content types:
  - **Direct Download**: Downloads files directly (PDFs, images, documents, etc.)
  - **Obelisk**: Archives HTML pages as single, self-contained HTML files with embedded assets
  - **Do Nothing**: Skip archiving for specific content types
- **Rule-Based Matching**: Configure archival rules that match on hostname and/or MIME type patterns using wildcards (e.g., `*.example.com`, `image/*`). Rules are evaluated in order, and the first matching rule determines which archival tool to use.
- **Intelligent Deduplication**:
  - Per-post deduplication to avoid re-archiving the same URL in the same post
  - Global deduplication using ETag and content hash comparison
  - Reuses existing archives when content is unchanged
- **Thread Replies**: Bot automatically replies in threads with archived files and status messages
- **Inline Preview**: Obelisk-archived HTML files can be previewed directly in the Mattermost UI
- **Error Handling**: Detailed error messages when archival fails
- **Bot Account**: Automatically creates and manages a bot account for posting archive notifications

## Installation

1. Download the latest release from the [releases page](https://github.com/fmartingrmattermost-plugin-link-archiver/releases)
2. Upload the plugin file (`com.mattermost.link-archiver-X.X.X.tar.gz`) to your Mattermost server via System Console > Plugins > Management
3. Enable the plugin
4. Configure the plugin (see Configuration section below)

### Building from Source

```bash
# Clone the repository
git clone https://github.com/fmartingrmattermost-plugin-link-archiver.git
cd mattermost-plugin-link-archiver

# Build the plugin
make

# The plugin bundle will be created at:
# dist/com.mattermost.link-archiver.tar.gz
```

## Configuration

### System Console Settings

Navigate to **System Console > Plugins > Link Archiver** to configure the plugin.

#### Archival Rules

Configure archival rules that match on hostname and/or MIME type patterns. Rules are evaluated in order, and the first matching rule determines which archival tool to use.

**Hostname Patterns:**
- **Exact matches**: `example.com` → matches exactly `example.com`
- **Wildcards**: `*.example.com` → matches any subdomain (e.g., `www.example.com`, `api.example.com`)

**MIME Type Patterns:**
- **Exact matches**: `application/pdf` → matches exactly `application/pdf`
- **Wildcards**: `image/*` → matches all image types (e.g., `image/jpeg`, `image/png`)

**Rule Matching:**
- Rules are evaluated in order from top to bottom
- The first rule that matches (both hostname and MIME type patterns if specified) determines the archival tool
- At least one pattern (hostname or MIME type) must be specified per rule
- If both patterns are specified, both must match (AND logic)

#### Default Archival Tool

Set the default tool to use when no archival rule matches. This acts as the final fallback rule. Options:
- `direct_download`: Download files directly
- `obelisk`: Archive HTML pages as single files
- `do_nothing`: Skip archiving

### Example Configuration

**Archival Rules (evaluated in order):**
1. Hostname: `*.github.com`, MIME Type: `text/html` → `obelisk`
2. Hostname: `*.example.com`, MIME Type: (empty) → `direct_download`
3. Hostname: (empty), MIME Type: `application/pdf` → `direct_download`
4. Hostname: (empty), MIME Type: `image/*` → `direct_download`
5. Hostname: (empty), MIME Type: `text/html` → `obelisk`

**Default Tool:** `do_nothing`

This configuration will:
- Archive HTML pages from GitHub using Obelisk
- Archive any content from example.com subdomains using direct download
- Archive PDFs from any hostname using direct download
- Archive images from any hostname using direct download
- Archive HTML pages from other hostnames using Obelisk
- Skip archiving for all other content types

## Archival Tools

### Direct Download (`direct_download`)

Downloads files directly from URLs. Best for:
- PDFs
- Images (JPEG, PNG, GIF, WebP)
- Office documents (Word, Excel, PowerPoint)
- Archives (ZIP, RAR, 7Z)
- Other binary files

**Limitations:**
- Maximum file size: 100MB
- Timeout: 30 seconds

### Obelisk (`obelisk`)

Archives HTML pages as single, self-contained HTML files with all assets embedded. Uses [go-shiori/obelisk](https://github.com/go-shiori/obelisk) to:
- Embed CSS, JavaScript, images, and other assets
- Create standalone HTML files that work offline
- Handle DNS errors gracefully (skips failed resources)

**Features:**
- Files are saved with `.obelisk.html` extension
- Can be previewed directly in Mattermost UI
- Maximum file size: 50MB
- Timeout: 60 seconds

### Do Nothing (`do_nothing`)

Skips archiving for specific content types. Useful when you want to:
- Disable archiving for certain MIME types
- Use as a default to disable automatic archiving globally

## How It Works

1. **Link Detection**: When a message is posted, the plugin automatically extracts URLs from the message text
2. **Content Detection**: For each URL, the plugin:
   - Performs a HEAD request to detect MIME type
   - Falls back to GET request if HEAD fails
   - Retrieves ETag for deduplication
3. **Deduplication Check**:
   - Checks if URL was already archived in the current post
   - Checks global archive metadata for existing archives
   - Compares ETags to detect unchanged content
   - Compares content hashes (SHA256) for verification
4. **Archival**:
   - Extracts hostname from URL
   - Evaluates archival rules in order
   - Selects appropriate tool based on first matching rule (or default tool if no rule matches)
   - Downloads/archives the content
   - Uploads to Mattermost file storage
5. **Notification**:
   - Bot replies in thread with archived file attachment
   - Includes file information (name, size, type)
   - Links to original post if file was reused from previous archive
   - Shows error message if archival fails

## File Preview

Obelisk-archived HTML files (`.obelisk.html`) can be previewed directly in the Mattermost UI. The preview component:
- Displays the archived page in a sandboxed iframe
- Works offline (all assets are embedded)
- Responsive layout that adapts to available space

## API Endpoints

The plugin exposes the following API endpoints (admin only):

- `GET /plugins/com.mattermost.link-archiver/api/v1/config` - Get current configuration
- `POST /plugins/com.mattermost.link-archiver/api/v1/config` - Update configuration
- `GET /plugins/com.mattermost.link-archiver/api/v1/archival-tools` - Get list of available archival tools

## Development

### Prerequisites

- Go 1.24.3 or later
- Node.js 16+ and npm 8+
- Mattermost server 6.2.1 or later

### Development Setup

1. Clone the repository:
```bash
git clone https://github.com/fmartingrmattermost-plugin-link-archiver.git
cd mattermost-plugin-link-archiver
```

2. Install dependencies:
```bash
# Server dependencies (managed by Go modules)
go mod download

# Webapp dependencies
cd webapp
npm install
cd ..
```

3. Build the plugin:
```bash
make
```

### Development Workflow

**Watch mode for webapp development:**
```bash
export MM_SERVICESETTINGS_SITEURL=http://localhost:8065
export MM_ADMIN_TOKEN=your-admin-token
make watch
```

**Deploy with local mode:**
```bash
make deploy
```

**Run tests:**
```bash
make test
```

### Project Structure

```
.
├── server/                 # Server-side Go code
│   ├── archiver/          # Archival tool implementations
│   │   ├── archiver.go    # ArchivalTool interface
│   │   ├── direct_download.go
│   │   └── obelisk.go
│   ├── api.go             # HTTP API endpoints
│   ├── archive_processor.go  # Main archival orchestration
│   ├── bot.go             # Bot account management
│   ├── content_detector.go  # MIME type and metadata detection
│   ├── link_extractor.go  # URL extraction from messages
│   ├── storage.go         # File storage and deduplication
│   └── thread_reply.go    # Thread reply creation
├── webapp/                # Client-side React/TypeScript code
│   ├── src/
│   │   ├── components/
│   │   │   ├── admin_settings.tsx      # Admin configuration UI
│   │   │   ├── admin_settings_wrapper.tsx
│   │   │   └── obelisk_file_preview.tsx  # File preview component
│   │   └── index.tsx      # Plugin registration
└── plugin.json            # Plugin manifest
```

## Architecture

### Components

1. **Link Extractor**: Extracts URLs from post messages using regex
2. **Content Detector**: Detects MIME types and retrieves metadata (ETag, size)
3. **Archive Processor**: Orchestrates the archival workflow
4. **Storage Service**: Manages file storage and deduplication metadata
5. **Thread Reply Service**: Creates bot replies in threads
6. **Bot Service**: Manages the plugin's bot account

### Deduplication Strategy

The plugin uses a multi-layered deduplication approach:

1. **Per-Post Deduplication**: Prevents re-archiving the same URL multiple times in the same post
2. **ETag Comparison**: Compares ETags to detect unchanged content without downloading
3. **Content Hash Verification**: Uses SHA256 hashes to verify content matches
4. **Global Archive Metadata**: Stores metadata about the most recent archive for each URL

### Data Storage

- **Mattermost File Storage**: Archived files are stored using Mattermost's file storage API
- **KV Store**: Archive metadata is stored in Mattermost's KV store with hashed keys to stay within 150-character limit

## Troubleshooting

### Plugin Not Archiving Links

1. Check that the plugin is enabled in System Console > Plugins
2. Verify configuration has a default archival tool set or archival rules configured
3. Check plugin logs for errors: `System Console > Logs` or server logs
4. Ensure the bot account was created successfully

### Archival Failures

- **Timeout errors**: Increase timeout in archival tool configuration (requires code changes)
- **File too large**: Files exceeding size limits will fail (100MB for direct download, 50MB for obelisk)
- **DNS errors**: Obelisk tool is configured to skip DNS errors, but the main page must load successfully
- **Permission errors**: Ensure the bot account has permission to upload files to channels

### Preview Not Working

- Ensure files have `.obelisk.html` extension
- Check browser console for errors
- Verify file was archived using obelisk tool (check file extension)

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This plugin is licensed under the MIT License. See [LICENSE](LICENSE) for details.

## Support

- **Issues**: [Git Issues](https://github.com/fmartingrmattermost-plugin-link-archiver/issues)
- **Mattermost Community**: [Mattermost Forum](https://forum.mattermost.com)

## Acknowledgments

- Uses [go-shiori/obelisk](https://github.com/go-shiori/obelisk) for HTML page archival
- Built on the [Mattermost Plugin Starter Template](https://github.com/mattermost/mattermost-plugin-starter-template)
