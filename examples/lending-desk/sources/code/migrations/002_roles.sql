CREATE TABLE roles (
    name VARCHAR(40) PRIMARY KEY
);

CREATE TABLE role_permissions (
    role       VARCHAR(40) NOT NULL REFERENCES roles (name),
    permission VARCHAR(80) NOT NULL,
    PRIMARY KEY (role, permission)
);
