# Nextera Pulse web

React, Vite, and TypeScript frontend for the Nextera Pulse demo workspace.

## Run locally

```powershell
cd web
npm install
npm run dev
```

Vite prints the local URL (usually `http://localhost:5173`). `npm run build` type-checks and produces the production bundle in `dist/`.

## Current scope

- Role-based demo workspaces for program managers, mentors, and students.
- Student roster, follow-up history, mentor assignment and capacity controls, and account/access management.
- Student profile pages with shared avatars and quotes; profile photo uploads can be cropped before saving.
- Notifications, pace analysis, light/dark theme, and responsive layouts for desktop and mobile.
- Profile, account, assignment, follow-up, theme, and notification demo state is stored locally in the browser.
- Mentor cohort dashboard and student pace table.
- Thirteen clearly synthetic student examples covering all six pace statuses and the 01Edu statuses used by the active KPI rules; selecting a row opens the student's metric and reason-code details.
- Demo API mode returns an API-shaped cohort response and simulates network latency. The sample data is synthetic and must not be presented as real student information.
- Search by student name/login; filters by server-provided pace status and curriculum month.
- English/Arabic layout direction and light/dark theme, saved in local browser storage.
- Loading, empty, and API error states.
- Pace status, gaps, timeline values and reason codes are displayed from the data source. The frontend does not calculate pace.
- The default source is synthetic demo data, explicitly labelled in the UI. It is not pilot/student data.

## Backend integration still needs a contract

`src/data/students.ts` is the adapter boundary. `VITE_STUDENTS_SOURCE=mock` (default) uses the demo API response; `VITE_STUDENTS_SOURCE=api` calls `GET /api/v1/cohorts/97/students` on `VITE_API_BASE_URL` and sends cookies for a same-site session. The first API contract represented here is `{ cohortName, eventId, snapshotDate, students }`. Each student includes pace metrics from the server plus `sourceStatus` from 01Edu. Confirm pagination, filters, exact DTO names, session behavior, and error format with the backend before production integration. Never put 01Edu credentials in this app; access 01Edu through the backend only.

The demo includes fake source statuses and applies the active KPI rules: `restricted`, `blocked-long`, `ban`, and `left-school` are excluded; `away`, `temporary-notice`, and `observation` remain included. Login and role authorization still need to follow the backend's Google OIDC/RBAC implementation. This UI is not an authentication or access-control boundary.
