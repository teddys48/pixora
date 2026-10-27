# AGENTS.md

## Project Overview

**Project name:** Pixora  
**Description:** A simple web-based photo gallery for uploading photos, organizing them into folders, and browsing photos by timeline.

### Core Features
- Upload single or multiple photos.
- Browse photos in a responsive gallery.
- Organize photos using folders.
- View photos grouped by capture date.
- Open photos in a lightbox viewer.
- Generate and serve thumbnails.
- Support light and dark themes.
- Provide reliable local storage and backup capabilities.

## Technology Stack

Use the following stack unless an existing, documented project decision specifies otherwise:

- **Frontend:** React + TypeScript + Vite
- **Styling:** Tailwind CSS + shadcn/ui
- **Icons:** Lucide
- **Backend:** Go + Fiber
- **Database:** SQLite
- **File storage:** Local filesystem
- **Deployment:** Docker Compose or Podman Compose

Avoid introducing additional frameworks, libraries, databases, or infrastructure without a clear requirement.

## General Engineering Principles

1. Inspect the existing codebase before making changes.
2. Follow established project conventions and architecture.
3. Prefer simple, maintainable, and well-tested solutions.
4. Keep changes focused on the current task or development phase.
5. Avoid unnecessary abstractions, dependencies, and premature optimization.
6. Do not rewrite working code without a clear technical reason.
7. Preserve backward compatibility unless a breaking change is explicitly required.
8. Never claim a feature or test works without verifying it.
9. Do not introduce unrelated features or perform unrelated refactoring.
10. Ask for clarification only when a missing decision materially affects correctness or architecture.

## Git Rules

- Inspect `git status` and relevant diffs before modifying files.
- Preserve existing user changes.
- Never discard, overwrite, reset, or clean up unrelated modifications.
- Do not run destructive Git commands.
- Do not create commits, push changes, or rewrite Git history unless explicitly requested.
- Review the final diff for accidental changes.

## Database Rules

**Never execute database commands or modify a live database directly.**

- Do not connect to production databases.
- Do not inspect or modify live database contents.
- Do not run destructive SQL against an existing database.
- Implement schema changes through version-controlled migration files.
- Database migrations must be reviewed and documented.
- Prefer additive migrations when possible.
- Include appropriate indexes, foreign keys, and constraints.
- Enable SQLite foreign-key enforcement where appropriate.
- Use transactions for operations that require atomic database updates.
- Never store image binary data directly in SQLite unless explicitly required.

Application code and automated tests may use isolated test databases. Do not point tests at a real user or production database.

## Docker and Infrastructure Rules

- Do not execute Docker, Docker Compose, Podman, or Podman Compose commands.
- Do not start, stop, restart, rebuild, or remove containers.
- Do not modify running infrastructure.
- You may inspect and edit Dockerfiles and Compose configuration as part of the requested implementation.
- Provide suggested commands for the user to run manually when runtime verification requires containers.

## File Storage and Upload Security

- Store original photos and thumbnails outside the frontend's public directory.
- Keep the storage root configurable through environment variables.
- Store relative storage paths in the database, not machine-specific absolute paths.
- Generate unique internal filenames; never use user-supplied filenames directly as filesystem paths.
- Preserve the original filename separately as metadata when needed.
- Validate file size, allowed formats, MIME type, and actual image content.
- Do not trust file extensions or client-provided MIME types alone.
- Reject unsupported formats and malformed images safely.
- Prevent path traversal and unauthorized file access.
- Set reasonable upload limits and resource limits.
- Avoid loading entire large files into memory unnecessarily.
- Clean up temporary files and handle partial failures.
- Prevent accidental overwriting of existing files.
- Do not expose internal filesystem paths or sensitive server errors through API responses.

## Photo Metadata and Timeline

- Prefer EXIF `DateTimeOriginal` for the photo capture date when available and valid.
- Fall back to a documented timestamp policy when EXIF data is missing or invalid.
- Keep capture time, upload time, and record creation time conceptually separate.
- Use a consistent timestamp format, preferably UTC for stored instants.
- Handle timezone information and EXIF timestamps carefully; do not invent a timezone when none is available.
- Make timeline sorting deterministic when multiple photos have the same timestamp.
- Store only metadata that provides clear product value.
- Treat EXIF data as untrusted input and handle malformed metadata safely.

## Backend Standards

- Follow idiomatic Go conventions and keep packages focused.
- Separate HTTP handlers, business logic, storage operations, and database access where appropriate.
- Use request-scoped contexts and timeouts.
- Validate incoming requests at the API boundary.
- Return consistent JSON response and error formats.
- Use appropriate HTTP status codes.
- Avoid leaking stack traces, SQL details, or filesystem paths.
- Implement pagination and deterministic sorting for photo listing endpoints.
- Avoid N+1 queries when retrieving gallery metadata.
- Use parameterized SQL queries.
- Handle file and database failures explicitly.
- Add unit and integration tests for important business logic.

## Frontend Standards

- Use TypeScript with strict type checking.
- Reuse existing UI components and established design tokens.
- Keep components focused and reusable without unnecessary abstraction.
- Separate API communication from presentation components.
- Provide loading, empty, error, and success states.
- Implement responsive layouts for desktop, tablet, and mobile.
- Support keyboard navigation and accessible controls.
- Use semantic HTML and appropriate accessible labels.
- Avoid unnecessary network requests and excessive component re-rendering.
- Use thumbnail URLs in gallery grids and original images in the viewer.
- Handle upload progress and per-file failures clearly.
- Do not hardcode API URLs, credentials, or environment-specific values.

## API and Architecture

- Keep the frontend independent of internal backend implementation details.
- Define and maintain a consistent REST API contract.
- Document endpoints, request parameters, response schemas, and errors.
- Use pagination for collections.
- Keep file storage logic separate from metadata persistence.
- Make storage and thumbnail paths configurable.
- Design storage interfaces so object storage can be introduced later without requiring a complete rewrite.
- Avoid adding authentication, multi-user support, or external services unless the scope requires them.

## Testing and Quality Gates

Before declaring a task complete:

1. Run relevant tests when the environment permits.
2. Run frontend linting and type checking when configured.
3. Run Go formatting and relevant Go tests when configured.
4. Check error handling and boundary cases.
5. Review security implications of file handling and API changes.
6. Review the Git diff for unintended changes.
7. Update `PROJECT_STATUS.md` when scope, progress, architecture, or decisions change.

If a command cannot be executed, explain why and provide the exact verification step the user can run manually.

Never claim tests passed when they were not executed.

## Documentation

- Keep setup instructions accurate.
- Document required environment variables in `.env.example`.
- Never commit secrets, API keys, passwords, or real credentials.
- Keep API documentation synchronized with implementation.
- Document database migrations and storage layout.
- Update `PROJECT_STATUS.md` after completing a phase or making a material architectural decision.

## Development Workflow

For each task:

1. Read this file and `PROJECT_STATUS.md`.
2. Inspect the current repository and relevant implementation.
3. Identify the current phase, scope, dependencies, and acceptance criteria.
4. Implement only the requested work.
5. Add or update tests and documentation.
6. Run available quality checks without executing prohibited infrastructure commands.
7. Review the final diff.
8. Update `PROJECT_STATUS.md` with verified results and remaining work.
9. Summarize changes, tests performed, known limitations, and manual verification steps.

## Phase Discipline

Pixora development is divided into six phases:

1. Foundation and UI.
2. Backend and data model.
3. Upload and photo management.
4. Folder management.
5. Timeline and photo viewer.
6. Hardening and deployment.

Do not implement future-phase features prematurely unless they are required to support the current phase and are explicitly documented.

## Definition of Done

A task is complete only when:

- The requested scope is implemented.
- Relevant tests and checks have been run where possible.
- Errors and edge cases are handled appropriately.
- No unrelated changes or prohibited operations were introduced.
- Documentation and `PROJECT_STATUS.md` reflect the actual state.
- Remaining limitations and unverified behavior are explicitly reported.
