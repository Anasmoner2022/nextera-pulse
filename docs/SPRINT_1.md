# Sprint 1 — Trusted Pace Foundation

## Duration

2 weeks.

## Sprint goal

For every active student in the Second Cohort, Nextera Pulse can reliably determine:

- enrollment age
- curriculum month
- actual level
- official minimum level
- official expected level
- checkpoint target
- gap
- pace state

## Pilot

01Edu label:

```text
01 Coding - Second Cohort
```

Event:

```text
97
```

## Backlog

| ID | Task | Owner | Review |
|---|---|---|---|
| XP-001 | Repository bootstrap + CI | Shahd | Anas |
| XP-002 | PostgreSQL schema + migrations | Sohila | Shahd |
| XP-003 | Secure 01Edu GraphQL client | Sohila | Shahd |
| XP-004 | Event 97 config + timeline sync | Sohila | Shahd |
| XP-005 | Second Cohort label sync | Sohila | Shahd |
| XP-006 | Status-policy seed | Sohila | Anas |
| XP-007 | Active academic age calculation | Sohila | Shahd + Anas |
| XP-008 | Pace Engine | Sohila | Shahd + Anas |
| XP-009 | Daily metric snapshots | Sohila | Shahd |
| XP-010 | Students API | Sohila | Shahd |
| XP-011 | React/Vite/TS foundation | Zeyad | Shahd |
| XP-012 | Mentor students table + filters | Zeyad | Anas |
| XP-013 | Manual 01Edu verification | Anas | Sohila |

## Pace rules

```text
if left-school:
    INACTIVE
else if currently paused:
    PAUSED
else if no applicable timeline:
    NOT_MEASURABLE
else if actual_level >= expected_level:
    GREEN
else if actual_level >= minimum_level:
    YELLOW
else:
    RED
```

Initial month calculation:

```text
active_program_days =
  current_time
  - event_user.createdAt
  - approved_pause_days

curriculum_month =
  floor(active_program_days / 30) + 1
```

The rule must be centralized and versioned.

## Sprint acceptance gate

Sprint 1 is accepted only if:

1. the Second Cohort is identified correctly;
2. current student level matches 01Edu;
3. official timeline thresholds come from Event 97 configuration;
4. pause rules match Product decisions;
5. pace status is deterministic and explainable;
6. representative students are manually validated against 01Edu;
7. no unresolved pace mismatch remains.
