// Package pace contains the Pace Engine — the core business logic that
// determines each student's pace status (GREEN, YELLOW, RED, PAUSED,
// INACTIVE, NOT_MEASURABLE).
package pace

// EngineVersion identifies the pace rules in this package. Snapshots record it
// together with the timeline version they used; bump it when a rule changes.
const EngineVersion = "v0.2"
