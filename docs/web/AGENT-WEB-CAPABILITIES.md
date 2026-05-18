# PicoClaw Agent Web Capabilities - Implementation Guide

## Overview

This document describes the new web-based capabilities added to PicoClaw to enable agents to perform advanced operations through the web interface. All capabilities are now available in the web dashboard for seamless integration with agent operations.

## Implemented Features

### 1. File Management

#### Upload Files
- **Endpoint**: `POST /api/projects/{name}/upload?path={relPath}`
- **Frontend**: Drag-and-drop file upload UI in Projects page
- **Max Size**: 100MB per file
- **Features**:
  - Multipart form data upload
  - Upload to any directory in the project
  - Real-time feedback on upload success/failure
  - Multiple file upload support

#### Download Files
- **Endpoint**: `GET /api/projects/{name}/download?path={filePath}`
- **Frontend**: Download button on file viewer
- **Features**:
  - Direct file streaming to browser
  - Automatic filename detection
  - Works with any file type

#### File Browsing & Editing
- **Endpoints**: 
  - `GET /api/projects` - List all projects
  - `GET /api/projects/{name}/entries?path={relPath}` - Browse directory
  - `GET /api/projects/{name}/file?path={filePath}` - Read file
  - `PUT /api/projects/{name}/file?path={filePath}` - Update file
- **Frontend**: Full file tree with breadcrumb navigation
- **Features**:
  - Folder traversal
  - File preview and editing
  - Max 2MB for editable files
  - UTF-8 text validation

### 2. Container Execution

#### Create Container
- **Endpoint**: `POST /api/containers/create`
- **Request Body**:
  ```json
  {
    "name": "container-name",
    "image": "image:tag",
    "env": ["VAR=value"],
    "work_dir": "/app",
    "command": ["cmd", "arg"]
  }
  ```
- **Features**:
  - Docker image validation
  - Automatic image pull
  - Environment variable support

#### Run Container
- **Endpoint**: `POST /api/containers/run`
- **Request Body**:
  ```json
  {
    "project_name": "my-project",
    "project_path": "subdir/optional",
    "image": "alpine:latest",
    "command": ["ls", "-la"],
    "timeout": 30,
    "env": ["KEY=value"],
    "work_dir": "/workspace"
  }
  ```
- **Response**:
  ```json
  {
    "exit_code": 0,
    "stdout": "...",
    "stderr": "...",
    "duration_ms": 1500,
    "status": "completed"
  }
  ```
- **Features**:
  - Automatic project path mounting to `/workspace`
  - Configurable timeout (max 5 minutes)
  - Stdout/stderr capture
  - Exit code tracking

#### Docker Requirements
- Docker must be installed and available in PATH
- System checks Docker availability before running
- Returns 503 if Docker is unavailable

### 3. Test Execution

#### Run Tests
- **Endpoint**: `POST /api/tests/run`
- **Request Body**:
  ```json
  {
    "project_name": "my-project",
    "project_path": "subdir/optional",
    "test_path": "specific/test/pattern",
    "verbose": true,
    "timeout": 60
  }
  ```
- **Response**:
  ```json
  {
    "test_name": "my-project",
    "passed": 10,
    "failed": 2,
    "skipped": 1,
    "duration_ms": 5000,
    "stdout": "...",
    "stderr": "...",
    "status": "completed",
    "exit_code": 1
  }
  ```
- **Supported Frameworks**:
  - **Go**: `go test ./...` (via `go.mod` detection)
  - **Node.js**: `npm test` (via `package.json` detection)
  - **Python**: `pytest` (via `pytest.ini` detection)
  - **Make**: `make test` (via `Makefile` detection)

### 4. Script Execution

#### Run Scripts
- **Endpoint**: `POST /api/scripts/run`
- **Request Body**:
  ```json
  {
    "project_name": "my-project",
    "project_path": "subdir/optional",
    "script_path": "scripts/deploy.sh",
    "args": ["arg1", "arg2"],
    "env": ["ENV_VAR=value"],
    "timeout": 120
  }
  ```
- **Supported Script Types**:
  - Shell scripts (`.sh`) → runs with `sh`
  - Python scripts (`.py`) → runs with `python3`
  - Go scripts (`.go`) → runs with `go run`
  - Executable files → runs directly
- **Response**: Same structure as container run response

### 5. Audio Transcription (Existing, Enhanced)

The web interface now properly integrates with PicoClaw's audio transcription capabilities:
- Audio input from chat is automatically transcribed
- Transcription results are displayed in the chat
- Supports various audio formats (WAV, MP3, etc.)
- See [agent_transcribe.go](../../pkg/agent/agent_transcribe.go) for implementation

## Backend Files Modified/Created

### New Files
- **`web/backend/api/execution.go`** - Container, test, and script execution APIs
  - ~400 lines of code
  - Handles Docker operations, test framework detection
  - Manages command execution with timeouts

### Modified Files
- **`web/backend/api/projects.go`**
  - Added file upload endpoint
  - Added file download endpoint
  - Added helper types for upload/download operations
  - ~100 lines added

- **`web/backend/api/router.go`**
  - Registered execution routes
  - Added call to `registerExecutionRoutes()`

## Frontend Files Modified/Created

### Modified Files
- **`web/frontend/src/api/projects.ts`** (~100 lines added)
  - Added TypeScript interfaces for container/test/script operations
  - Added API client functions:
    - `uploadProjectFile()`
    - `downloadProjectFile()`
    - `runContainer()`
    - `runTest()`
    - `runScript()`

- **`web/frontend/src/components/projects/projects-page.tsx`** (~150 lines added)
  - Added upload UI with drag-and-drop
  - Added download button
  - Added test runner UI
  - Added container runner UI
  - Added result display panels
  - Integrated all new features into Projects page

## Security Considerations

1. **Path Validation**: All file paths are validated with `safeRelPath()` to prevent directory traversal
2. **File Size Limits**: 
   - Upload: 100MB max per file
   - Edit: 2MB max for browser-based editing
3. **Container Sandboxing**: Containers run with `--rm` flag and have restricted access via volume mounts
4. **Timeout Protection**: All operations have configurable timeouts (max 5 minutes)
5. **Project Scope**: All operations are confined to the project workspace directory

## API Endpoints Summary

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/projects` | List all projects |
| GET | `/api/projects/{name}/entries` | List directory contents |
| GET | `/api/projects/{name}/file` | Read file content |
| PUT | `/api/projects/{name}/file` | Update file content |
| POST | `/api/projects/{name}/upload` | Upload files *(NEW)* |
| GET | `/api/projects/{name}/download` | Download file *(NEW)* |
| POST | `/api/containers/create` | Create container *(NEW)* |
| POST | `/api/containers/run` | Run container command *(NEW)* |
| POST | `/api/tests/run` | Run project tests *(NEW)* |
| POST | `/api/scripts/run` | Execute script *(NEW)* |

## Usage Examples

### Frontend - Upload Files
```typescript
import { uploadProjectFile } from "@/api/projects"

const files = [new File(...), ...]
await uploadProjectFile("my-project", files, "path/to/upload")
```

### Frontend - Run Tests
```typescript
import { runTest } from "@/api/projects"

const result = await runTest({
  project_name: "my-project",
  verbose: true,
  timeout: 60
})
console.log(`${result.passed} passed, ${result.failed} failed`)
```

### Frontend - Run Container
```typescript
import { runContainer } from "@/api/projects"

const result = await runContainer({
  project_name: "my-project",
  image: "node:20-alpine",
  command: ["npm", "build"],
  timeout: 120
})
```

## Integration with Chat

The web interface now enables:
1. **Audio Input**: Users can send voice messages in chat
2. **Transcription**: Audio is automatically converted to text
3. **File Context**: Users can upload project files for the agent to analyze
4. **Live Execution**: Users can run tests and containers directly from the UI
5. **Result Download**: Users can download test reports and build artifacts

## Testing the Features

### Test File Upload
1. Navigate to Projects page
2. Select a project
3. Click "Upload Files" button
4. Select one or more files
5. Verify files appear in file tree

### Test File Download
1. Navigate to Projects page
2. Select a file from the tree
3. Click "Download" button
4. Verify file downloads to Downloads folder

### Test Container Execution
1. Ensure Docker is installed (`docker version`)
2. Click "Run Container" button
3. Verify output appears in Container Output panel

### Test Script Execution
1. Create a test script in project
2. Use `POST /api/scripts/run` to execute
3. Verify output in response

## Future Enhancements

- [ ] WebSocket support for live streaming of test/container output
- [ ] Support for custom test frameworks
- [ ] Build artifact caching
- [ ] Container logs persistence
- [ ] Test result history tracking
- [ ] Script scheduling via cron
- [ ] GPU support for container execution

## Troubleshooting

### Docker Not Found
- Error: "Docker is not available on this system"
- Solution: Ensure Docker is installed and accessible in PATH

### File Upload Fails
- Check file size (max 100MB)
- Verify project directory is writable
- Check available disk space

### Tests Not Detected
- Verify project has compatible test framework
- Check for `go.mod`, `package.json`, `pytest.ini`, or `Makefile`
- See `detectTestCommand()` in `execution.go`

### Container Execution Timeout
- Increase timeout parameter (max 5 minutes)
- Optimize container image or commands
- Check container logs for errors
