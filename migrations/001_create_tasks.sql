CREATE TABLE IF NOT EXISTS tasks (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 title VARCHAR(200) NOT NULL CHECK (length(trim(title)) > 0),
 description VARCHAR(5000) NOT NULL DEFAULT '',
 status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','in_progress','completed')),
 assignee VARCHAR(200) NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
