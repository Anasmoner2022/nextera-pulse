export type PulseRole = 'manager' | 'mentor' | 'student';

export type AccessKey = 'viewProgress' | 'viewCohort' | 'manageStudents' | 'manageAccounts' | 'manageInterventions' | 'viewAllStudents' | 'viewMentorPerformance';

export type PulseAccount = { id: string; name: string; email: string; role: PulseRole; permissions: AccessKey[]; passwordHash?: string; studentLogin?: string; mentorId?: string; assignedStudentLogins?: string[]; capacityLimit?: number; avatar?: string; quote?: string; };

export type PulseSession = Omit<PulseAccount, 'passwordHash'>;
