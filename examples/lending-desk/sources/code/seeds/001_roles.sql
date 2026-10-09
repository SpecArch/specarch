INSERT INTO roles (name) VALUES ('desk-staff'), ('desk-supervisor');

INSERT INTO role_permissions (role, permission) VALUES
    ('desk-staff', 'members.write'),
    ('desk-staff', 'loans.write'),
    ('desk-staff', 'loans.read'),
    ('desk-supervisor', 'loans.read'),
    ('desk-supervisor', 'loans.writeoff');
