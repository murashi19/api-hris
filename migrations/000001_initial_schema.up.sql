CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_users_deleted_at ON users(deleted_at);

CREATE TABLE roles (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(80) NOT NULL UNIQUE,
    description VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE permissions (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(100) NOT NULL UNIQUE,
    description VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE user_roles (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY(user_id, role_id)
);
CREATE TABLE role_permissions (
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY(role_id, permission_id)
);
CREATE TABLE refresh_sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_refresh_sessions_user_id ON refresh_sessions(user_id);
CREATE INDEX idx_refresh_sessions_expires_at ON refresh_sessions(expires_at);
CREATE INDEX idx_refresh_sessions_revoked_at ON refresh_sessions(revoked_at);

CREATE TABLE departments (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL UNIQUE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE positions (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    department_id BIGINT REFERENCES departments(id) ON DELETE RESTRICT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_positions_name_department ON positions(name, department_id) NULLS NOT DISTINCT;

CREATE TABLE employees (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT UNIQUE REFERENCES users(id) ON DELETE SET NULL,
    employee_number VARCHAR(30) NOT NULL UNIQUE,
    full_name VARCHAR(180) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    phone VARCHAR(40) NOT NULL DEFAULT '',
    birth_date DATE,
    join_date DATE NOT NULL,
    employment_type VARCHAR(30) NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('ACTIVE','INACTIVE','TERMINATED')),
    department_id BIGINT REFERENCES departments(id) ON DELETE RESTRICT,
    position_id BIGINT REFERENCES positions(id) ON DELETE RESTRICT,
    manager_id BIGINT REFERENCES employees(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT employees_not_own_manager CHECK (manager_id IS NULL OR manager_id <> id)
);
CREATE INDEX idx_employees_full_name ON employees(full_name);
CREATE INDEX idx_employees_status ON employees(status);
CREATE INDEX idx_employees_department_id ON employees(department_id);
CREATE INDEX idx_employees_position_id ON employees(position_id);
CREATE INDEX idx_employees_manager_id ON employees(manager_id);
CREATE INDEX idx_employees_deleted_at ON employees(deleted_at);

CREATE TABLE attendance_records (
    id BIGSERIAL PRIMARY KEY,
    employee_id BIGINT NOT NULL REFERENCES employees(id) ON DELETE RESTRICT,
    work_date DATE NOT NULL,
    clock_in_at TIMESTAMPTZ,
    clock_out_at TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL CHECK (status IN ('PRESENT','LATE','ABSENT','LEAVE')),
    notes VARCHAR(500) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT attendance_clock_order CHECK (clock_out_at IS NULL OR (clock_in_at IS NOT NULL AND clock_out_at >= clock_in_at)),
    UNIQUE(employee_id, work_date)
);
CREATE INDEX idx_attendance_records_work_date ON attendance_records(work_date);

CREATE TABLE leave_types (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL UNIQUE,
    code VARCHAR(30) NOT NULL UNIQUE,
    requires_balance BOOLEAN NOT NULL DEFAULT TRUE,
    requires_attachment BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE leave_balances (
    id BIGSERIAL PRIMARY KEY,
    employee_id BIGINT NOT NULL REFERENCES employees(id) ON DELETE RESTRICT,
    leave_type_id BIGINT NOT NULL REFERENCES leave_types(id) ON DELETE RESTRICT,
    year INTEGER NOT NULL CHECK (year BETWEEN 2000 AND 2200),
    entitled NUMERIC(7,2) NOT NULL DEFAULT 0 CHECK (entitled >= 0),
    used NUMERIC(7,2) NOT NULL DEFAULT 0 CHECK (used >= 0),
    adjustment NUMERIC(7,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(employee_id, leave_type_id, year),
    CONSTRAINT leave_balance_available_nonnegative CHECK (entitled + adjustment - used >= 0)
);
CREATE TABLE leave_requests (
    id BIGSERIAL PRIMARY KEY,
    employee_id BIGINT NOT NULL REFERENCES employees(id) ON DELETE RESTRICT,
    leave_type_id BIGINT NOT NULL REFERENCES leave_types(id) ON DELETE RESTRICT,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    duration NUMERIC(7,2) NOT NULL CHECK (duration > 0),
    reason VARCHAR(1000) NOT NULL,
    status VARCHAR(30) NOT NULL CHECK (status IN ('PENDING_MANAGER','PENDING_HR','APPROVED','REJECTED','CANCELLED')),
    submitted_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT leave_request_date_order CHECK (end_date >= start_date)
);
CREATE INDEX idx_leave_requests_employee_id ON leave_requests(employee_id);
CREATE INDEX idx_leave_requests_leave_type_id ON leave_requests(leave_type_id);
CREATE INDEX idx_leave_requests_status ON leave_requests(status);
CREATE INDEX idx_leave_requests_dates ON leave_requests(start_date, end_date);
CREATE TABLE leave_approvals (
    id BIGSERIAL PRIMARY KEY,
    leave_request_id BIGINT NOT NULL REFERENCES leave_requests(id) ON DELETE RESTRICT,
    stage VARCHAR(20) NOT NULL CHECK (stage IN ('MANAGER','HR')),
    status VARCHAR(20) NOT NULL CHECK (status IN ('PENDING','APPROVED','REJECTED','CANCELLED')),
    actor_user_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    reason VARCHAR(1000) NOT NULL DEFAULT '',
    acted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(leave_request_id, stage)
);
CREATE INDEX idx_leave_approvals_request_id ON leave_approvals(leave_request_id);

CREATE TABLE notifications (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(60) NOT NULL,
    title VARCHAR(180) NOT NULL,
    message VARCHAR(1000) NOT NULL,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_notifications_user_id ON notifications(user_id);
CREATE INDEX idx_notifications_read_at ON notifications(read_at);
CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(80) NOT NULL,
    entity_id VARCHAR(80) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_audit_logs_actor_user_id ON audit_logs(actor_user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
