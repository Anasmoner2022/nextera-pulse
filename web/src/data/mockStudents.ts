import type { StatusPolicy, StudentPaceRow, StudentsResponse } from '../domain';

// Synthetic demo rows only; replace with the agreed API adapter.
const demoContacts: Record<string, Pick<StudentPaceRow, 'email' | 'age' | 'phone'>> = {
  'nour.ahmed': { email: 'nour.ahmed@example.com', age: 19, phone: '+20 10 2456 9701' },
  'omar.samir': { email: 'omar.samir@example.com', age: 20, phone: '+20 11 3814 9702' },
  'malak.hassan': { email: 'malak.hassan@example.com', age: 18, phone: '+20 12 5273 9703' },
  'youssef.ali': { email: 'youssef.ali@example.com', age: 21, phone: '+20 10 6142 9704' },
  'jana.mostafa': { email: 'jana.mostafa@example.com', age: 17, phone: '+20 11 7295 9705' },
  'adam.khaled': { email: 'adam.khaled@example.com', age: 20, phone: '+20 12 8361 9706' },
  'farida.ashraf': { email: 'farida.ashraf@example.com', age: 19, phone: '+20 10 9428 9707' },
  'kareem.mahmoud': { email: 'kareem.mahmoud@example.com', age: 22, phone: '+20 11 1537 9708' },
  'salma.tarek': { email: 'salma.tarek@example.com', age: 18, phone: '+20 12 2684 9709' },
  'seif.hany': { email: 'seif.hany@example.com', age: 20, phone: '+20 10 3749 9710' },
  'laila.wael': { email: 'laila.wael@example.com', age: 19, phone: '+20 11 4852 9711' },
  'zain.mohamed': { email: 'zain.mohamed@example.com', age: 17, phone: '+20 12 5916 9712' },
  'mariam.fathy': { email: 'mariam.fathy@example.com', age: 21, phone: '+20 10 6473 9713' },
};

export const mockStudents: StudentsResponse = {
  cohortName: '01 Coding - Second Cohort', eventId: 97, snapshotDate: '2026-10-06',
  students: ([
    { studentId: 9701, login: 'nour.ahmed', displayName: 'Nour Ahmed', enrolledAt: '2026-06-01', activeAcademicDays: 82, curriculumMonth: 4, curriculumRound: 2, actualLevel: 8, minimumLevel: 7, expectedLevel: 8, checkpointTarget: 8, gapToMinimum: 1, gapToExpected: 0, paceStatus: 'GREEN', sourceStatus: 'active', healthReasonCodes: [] },
    { studentId: 9702, login: 'omar.samir', displayName: 'Omar Samir', enrolledAt: '2026-06-03', activeAcademicDays: 80, curriculumMonth: 4, curriculumRound: 1, actualLevel: 6, minimumLevel: 7, expectedLevel: 8, checkpointTarget: 8, gapToMinimum: -1, gapToExpected: -2, paceStatus: 'YELLOW', sourceStatus: 'away', healthReasonCodes: ['BELOW_EXPECTED_LEVEL'] },
    { studentId: 9703, login: 'malak.hassan', displayName: 'Malak Hassan', enrolledAt: '2026-06-05', activeAcademicDays: 79, curriculumMonth: 4, curriculumRound: 3, actualLevel: 4, minimumLevel: 7, expectedLevel: 8, checkpointTarget: 8, gapToMinimum: -3, gapToExpected: -4, paceStatus: 'RED', sourceStatus: 'temporary-notice', healthReasonCodes: ['BELOW_MINIMUM_LEVEL'] },
    { studentId: 9704, login: 'youssef.ali', displayName: 'Youssef Ali', enrolledAt: '2026-06-09', activeAcademicDays: 73, curriculumMonth: 4, curriculumRound: 2, actualLevel: 5, minimumLevel: 7, expectedLevel: 8, checkpointTarget: 8, gapToMinimum: -2, gapToExpected: -3, paceStatus: 'PAUSED', sourceStatus: 'restricted', healthReasonCodes: ['CURRENTLY_PAUSED'] },
    { studentId: 9705, login: 'jana.mostafa', displayName: 'Jana Mostafa', enrolledAt: '2026-06-12', activeAcademicDays: 0, curriculumMonth: 0, curriculumRound: null, actualLevel: 0, minimumLevel: null, expectedLevel: null, checkpointTarget: null, gapToMinimum: null, gapToExpected: null, paceStatus: 'NOT_MEASURABLE', sourceStatus: 'observation', healthReasonCodes: ['NO_TIMELINE'] },
    { studentId: 9706, login: 'adam.khaled', displayName: 'Adam Khaled', enrolledAt: '2026-05-22', activeAcademicDays: 0, curriculumMonth: 0, curriculumRound: null, actualLevel: 3, minimumLevel: null, expectedLevel: null, checkpointTarget: null, gapToMinimum: null, gapToExpected: null, paceStatus: 'INACTIVE', sourceStatus: 'left-school', healthReasonCodes: ['LEFT_SCHOOL'] },
    { studentId: 9707, login: 'farida.ashraf', displayName: 'Farida Ashraf', enrolledAt: '2026-06-18', activeAcademicDays: 68, curriculumMonth: 3, curriculumRound: 2, actualLevel: 7, minimumLevel: 6, expectedLevel: 7, checkpointTarget: 7, gapToMinimum: 1, gapToExpected: 0, paceStatus: 'GREEN', sourceStatus: 'active', healthReasonCodes: [] },
    { studentId: 9708, login: 'kareem.mahmoud', displayName: 'Kareem Mahmoud', enrolledAt: '2026-06-20', activeAcademicDays: 64, curriculumMonth: 3, curriculumRound: 1, actualLevel: 5, minimumLevel: 6, expectedLevel: 7, checkpointTarget: 7, gapToMinimum: -1, gapToExpected: -2, paceStatus: 'YELLOW', sourceStatus: 'active', healthReasonCodes: ['BELOW_EXPECTED_LEVEL'] },
    { studentId: 9709, login: 'salma.tarek', displayName: 'Salma Tarek', enrolledAt: '2026-06-24', activeAcademicDays: 61, curriculumMonth: 3, curriculumRound: 3, actualLevel: 3, minimumLevel: 6, expectedLevel: 7, checkpointTarget: 7, gapToMinimum: -3, gapToExpected: -4, paceStatus: 'RED', sourceStatus: 'active', healthReasonCodes: ['BELOW_MINIMUM_LEVEL'] },
    { studentId: 9710, login: 'seif.hany', displayName: 'Seif Hany', enrolledAt: '2026-07-01', activeAcademicDays: 55, curriculumMonth: 3, curriculumRound: 2, actualLevel: 5, minimumLevel: 6, expectedLevel: 7, checkpointTarget: 7, gapToMinimum: -1, gapToExpected: -2, paceStatus: 'PAUSED', sourceStatus: 'blocked-long', healthReasonCodes: ['CURRENTLY_PAUSED'] },
    { studentId: 9711, login: 'laila.wael', displayName: 'Laila Wael', enrolledAt: '2026-07-04', activeAcademicDays: 52, curriculumMonth: 3, curriculumRound: 1, actualLevel: 6, minimumLevel: 6, expectedLevel: 7, checkpointTarget: 7, gapToMinimum: 0, gapToExpected: -1, paceStatus: 'YELLOW', sourceStatus: 'active', healthReasonCodes: ['BELOW_EXPECTED_LEVEL'] },
    { studentId: 9712, login: 'zain.mohamed', displayName: 'Zain Mohamed', enrolledAt: '2026-07-09', activeAcademicDays: 47, curriculumMonth: 2, curriculumRound: 2, actualLevel: 4, minimumLevel: 4, expectedLevel: 5, checkpointTarget: 5, gapToMinimum: 0, gapToExpected: -1, paceStatus: 'GREEN', sourceStatus: 'active', healthReasonCodes: [] },
    { studentId: 9713, login: 'mariam.fathy', displayName: 'Mariam Fathy', enrolledAt: '2026-07-12', activeAcademicDays: 44, curriculumMonth: 2, curriculumRound: 1, actualLevel: 3, minimumLevel: 4, expectedLevel: 5, checkpointTarget: 5, gapToMinimum: -1, gapToExpected: -2, paceStatus: 'PAUSED', sourceStatus: 'ban', healthReasonCodes: ['CURRENTLY_PAUSED'] },
  ] satisfies StudentPaceRow[]).map(student => ({ ...student, ...demoContacts[student.login] })),
};

/**
 * Demo copy of the backend's status_policy seed (internal/store/migrations.go),
 * used only to compute the on-track rate on mock data. Live mode reads the
 * real table on the server.
 */
export const mockStatusPolicy: StatusPolicy[] = [
  { recordType: 'away', excludeFromActiveKpis: false },
  { recordType: 'restricted', excludeFromActiveKpis: true },
  { recordType: 'blocked-long', excludeFromActiveKpis: true },
  { recordType: 'temporary-notice', excludeFromActiveKpis: false },
  { recordType: 'left-school', excludeFromActiveKpis: true },
  { recordType: 'observation', excludeFromActiveKpis: false },
  { recordType: 'ban', excludeFromActiveKpis: true },
];
