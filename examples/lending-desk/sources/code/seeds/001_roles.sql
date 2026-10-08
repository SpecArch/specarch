INSERT INTO roles (name) VALUES ('desk-staff');

INSERT INTO role_permissions (role, permission) VALUES
    ('desk-staff', 'members.write'),
    ('desk-staff', 'loans.write'),
    ('desk-staff', 'loans.read');
