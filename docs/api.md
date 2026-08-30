# API Contract

Base URL: `/api`. Semua response mengikuti bentuk `{ "success", "message", "data" }`; list menambahkan `meta`, dan error menambahkan `code`. Endpoint selain login/refresh membutuhkan Bearer access token.

## Authentication

| Method | Endpoint | Akses |
|---|---|---|
| POST | `/auth/login` | Public, rate limited |
| POST | `/auth/refresh` | Refresh cookie, rate limited |
| POST | `/auth/logout` | Refresh cookie, rate limited |
| GET | `/auth/me` | Authenticated |

Login menerima `{ "email", "password" }`. Refresh token tidak ada di JSON; browser mengelolanya sebagai cookie HttpOnly.

## Organization and attendance

| Method | Endpoint | Permission |
|---|---|---|
| GET/POST | `/employees` | `employee.read` / `employee.create` |
| GET/PATCH | `/employees/:id` | `employee.read` / `employee.update` |
| GET | `/departments` | `department.read` |
| POST/PATCH | `/departments[/:id]` | `department.manage` |
| GET | `/positions` | `position.read` |
| POST/PATCH | `/positions[/:id]` | `position.manage` |
| GET | `/attendance/me` | `attendance.self.read` |
| GET | `/attendance/team` | `attendance.team.read`, direct reports only |
| POST | `/attendance/clock-in`, `/attendance/clock-out` | `attendance.clock` |
| GET | `/attendance` | `attendance.all.read` |

List employees mendukung `page`, `limit`, `search`, `status`, dan `departmentId`. List attendance mendukung `page`, `limit`, `employeeId`, `startDate`, dan `endDate`.

## Leave

| Method | Endpoint | Permission |
|---|---|---|
| GET | `/leave-types` | Authenticated |
| POST/PATCH | `/leave-types[/:id]` | `leave.type.manage` |
| GET | `/leave-balances/me` | `leave.self.read` |
| PUT | `/leave-balances/entitlement` | `leave.balance.manage` |
| POST | `/leave-balances/adjust` | `leave.balance.manage` |
| GET | `/leave-requests/me` | `leave.self.read` |
| POST | `/leave-requests` | `leave.create` |
| GET | `/leave-requests/:id` | self/team/all read dengan scope check |
| POST | `/leave-requests/:id/cancel` | `leave.create`, request sendiri |
| GET | `/leave-approvals` | manager atau HR approve |
| POST | `/leave-requests/:id/manager-approve` | `leave.manager.approve`, direct report |
| POST | `/leave-requests/:id/hr-approve` | `leave.hr.approve` |
| POST | `/leave-requests/:id/reject` | manager/HR sesuai stage |

Submit body: `{ "leaveTypeId": 1, "startDate": "2026-09-01", "endDate": "2026-09-03", "reason": "..." }`. Reject body wajib `{ "reason": "..." }`. Adjustment body wajib menyertakan `employeeId`, `leaveTypeId`, `year`, `amount`, dan `reason`.

## Notification, audit, and administration

| Method | Endpoint | Permission |
|---|---|---|
| GET | `/notifications` | Authenticated, data sendiri |
| POST | `/notifications/:id/read` | Authenticated, data sendiri |
| GET | `/audit-logs` | `audit.read` |
| GET/POST | `/admin/users` | `role.manage` |
| PUT | `/admin/users/:id/roles` | `role.manage` |
| GET/POST | `/admin/roles` | `role.manage` |
| GET | `/admin/permissions` | `role.manage` |

List API menggunakan pagination server-side dengan limit maksimum 100.
