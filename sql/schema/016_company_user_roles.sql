-- +goose Up
CREATE TABLE IF NOT EXISTS company_user_roles (
    company_id uuid NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (company_id, user_id),
    CONSTRAINT chk_company_user_roles_role
        CHECK (role IN ('admin', 'member', 'viewer'))
);

CREATE INDEX IF NOT EXISTS idx_company_user_roles_user
    ON company_user_roles (user_id);

CREATE INDEX IF NOT EXISTS idx_company_user_roles_company
    ON company_user_roles (company_id);

CREATE TRIGGER update_company_user_roles_updated_at
    BEFORE UPDATE ON company_user_roles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assign_initial_company_role()
RETURNS TRIGGER AS $$
DECLARE
    assigned_role text;
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM company_user_roles
        WHERE company_id = NEW.company_id
    ) THEN
        assigned_role := 'admin';
    ELSE
        assigned_role := 'member';
    END IF;

    INSERT INTO company_user_roles (company_id, user_id, role)
    VALUES (NEW.company_id, NEW.id, assigned_role)
    ON CONFLICT (company_id, user_id) DO NOTHING;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER assign_initial_company_role_trigger
    AFTER INSERT ON users
    FOR EACH ROW EXECUTE FUNCTION assign_initial_company_role();

INSERT INTO company_user_roles (company_id, user_id, role)
SELECT company_id, id, 'admin'
FROM (
    SELECT
        u.id,
        u.company_id,
        ROW_NUMBER() OVER (PARTITION BY u.company_id ORDER BY u.created_at) AS row_number
    FROM users u
    WHERE u.is_active = TRUE
) ranked
WHERE ranked.row_number = 1
ON CONFLICT (company_id, user_id) DO NOTHING;

-- +goose Down
DROP TRIGGER IF EXISTS assign_initial_company_role_trigger ON users;
DROP FUNCTION IF EXISTS assign_initial_company_role();
DROP TRIGGER IF EXISTS update_company_user_roles_updated_at ON company_user_roles;
DROP INDEX IF EXISTS idx_company_user_roles_company;
DROP INDEX IF EXISTS idx_company_user_roles_user;
DROP TABLE IF EXISTS company_user_roles;
