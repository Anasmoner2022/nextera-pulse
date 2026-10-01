# Contributing to Nextera Pulse

## Branches

Create short-lived branches from `main`.

Recommended naming:

```text
feat/XP-007-active-program-age
fix/XP-008-pace-boundary
chore/XP-001-ci
docs/kickoff-notes
```

Do not push feature work directly to `main`.

## Pull requests

Every PR should:

- reference the ticket ID
- explain what changed
- explain why
- include test evidence
- mention any migration or environment change
- call out business-rule changes explicitly

Keep PRs focused. Large unrelated changes should be split.

## Review ownership

### Shahd — Tech Lead / Architect

Required reviewer for:

- architecture changes
- Go package boundaries
- database design / migrations
- auth
- 01Edu integration
- sync reliability
- Pace Engine
- deployment-critical changes

### Anas — Product Owner

Required approval for:

- pace definitions
- academic-clock rules
- KPI inclusion/exclusion
- user-facing status wording
- Sprint acceptance criteria
- scope changes

### Sohila — Backend

Primary implementation ownership:

- PostgreSQL
- 01Edu client
- source sync
- pace calculations
- snapshots
- backend APIs

### Zeyad — Frontend

Primary implementation ownership:

- React application structure
- mentor experience
- student tables
- filters/states
- API integration

Backend contributions by Zeyad are welcome when scoped and reviewed.

## Interns

Interns should initially work only on scoped, reviewable tasks such as:

- React components
- loading / empty / error states
- fixtures and mock data
- frontend tests
- API integration tests
- documentation
- QA utilities

Interns should not initially own:

- architecture
- production secrets
- authentication
- core migrations
- 01Edu credential handling
- Pace Engine rules
- sync integrity

## Definition of Done

A task is not done until:

- code is reviewed
- tests pass
- error cases are handled
- docs/config are updated where needed
- no secret was committed
- acceptance criteria are satisfied

For pace-related work, manual comparison against 01Edu is part of acceptance.
