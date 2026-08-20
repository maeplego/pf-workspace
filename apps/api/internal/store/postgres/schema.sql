-- Local / overlay demo schema. ULID ids (TEXT), timestamptz clocks.

CREATE TABLE IF NOT EXISTS workspaces (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  org_id TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS org_id TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS members (
  workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
  sub TEXT NOT NULL,
  role TEXT NOT NULL,
  display_name TEXT NOT NULL DEFAULT '',
  joined_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (workspace_id, sub)
);

ALTER TABLE members ADD COLUMN IF NOT EXISTS display_name TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS members_sub_idx ON members (sub);

CREATE TABLE IF NOT EXISTS invitations (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  role TEXT NOT NULL,
  max_uses INTEGER NOT NULL,
  use_count INTEGER NOT NULL DEFAULT 0,
  expires_at TIMESTAMPTZ NOT NULL,
  invited_by TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ
);

ALTER TABLE invitations ADD COLUMN IF NOT EXISTS invited_email TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS invitations_workspace_idx ON invitations (workspace_id, created_at DESC);
CREATE INDEX IF NOT EXISTS invitations_expires_idx ON invitations (expires_at);

CREATE TABLE IF NOT EXISTS audit_events (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
  actor_sub TEXT NOT NULL,
  type TEXT NOT NULL,
  target_sub TEXT NOT NULL DEFAULT '',
  invite_id TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS audit_events_workspace_idx ON audit_events (workspace_id, created_at DESC);

CREATE TABLE IF NOT EXISTS boards (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE boards ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS boards_workspace_id_idx ON boards (workspace_id);

CREATE TABLE IF NOT EXISTS columns (
  id TEXT PRIMARY KEY,
  board_id TEXT NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  position INTEGER NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS columns_board_id_idx ON columns (board_id);

CREATE TABLE IF NOT EXISTS sprints (
  id TEXT PRIMARY KEY,
  board_id TEXT NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
  workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  start_at TIMESTAMPTZ NOT NULL,
  end_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS sprints_board_id_idx ON sprints (board_id);

CREATE TABLE IF NOT EXISTS cards (
  id TEXT PRIMARY KEY,
  column_id TEXT NOT NULL REFERENCES columns (id) ON DELETE CASCADE,
  board_id TEXT NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  position INTEGER NOT NULL,
  version INTEGER NOT NULL,
  sprint_id TEXT NOT NULL DEFAULT '',
  completed_at TIMESTAMPTZ,
  assignee_sub TEXT NOT NULL DEFAULT '',
  priority TEXT NOT NULL DEFAULT '',
  due_date TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS cards_column_id_idx ON cards (column_id);
CREATE INDEX IF NOT EXISTS cards_board_id_idx ON cards (board_id);

CREATE TABLE IF NOT EXISTS pages (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
  parent_id TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL,
  body TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  position INTEGER NOT NULL,
  version INTEGER NOT NULL,
  collab_document_id TEXT NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS pages_workspace_id_idx ON pages (workspace_id);

ALTER TABLE pages ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS documents (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  body TEXT NOT NULL DEFAULT '',
  collab_document_id TEXT NOT NULL UNIQUE,
  last_editor_sub TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS documents_workspace_id_idx ON documents (workspace_id);

CREATE TABLE IF NOT EXISTS collab_tickets (
  id TEXT PRIMARY KEY,
  sub TEXT NOT NULL,
  collab_document_id TEXT NOT NULL,
  read_only BOOLEAN NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS channels (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS channels_workspace_id_idx ON channels (workspace_id);

CREATE TABLE IF NOT EXISTS chat_messages (
  id TEXT PRIMARY KEY,
  channel_id TEXT NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
  sub TEXT NOT NULL,
  body TEXT NOT NULL,
  mentions TEXT[] NOT NULL DEFAULT '{}',
  attachment_file_id TEXT NOT NULL DEFAULT '',
  seq INTEGER NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE (channel_id, seq)
);

CREATE INDEX IF NOT EXISTS chat_messages_channel_id_idx ON chat_messages (channel_id);

CREATE TABLE IF NOT EXISTS chat_tickets (
  id TEXT PRIMARY KEY,
  sub TEXT NOT NULL,
  channel_id TEXT NOT NULL,
  read_only BOOLEAN NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS stored_files (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
  uploader_sub TEXT NOT NULL,
  purpose TEXT NOT NULL,
  provider TEXT NOT NULL,
  content_type TEXT NOT NULL DEFAULT '',
  size BIGINT NOT NULL DEFAULT 0,
  name TEXT NOT NULL DEFAULT '',
  view_token TEXT NOT NULL DEFAULT '',
  path TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS page_files (
  page_id TEXT NOT NULL REFERENCES pages (id) ON DELETE CASCADE,
  file_id TEXT NOT NULL REFERENCES stored_files (id) ON DELETE CASCADE,
  PRIMARY KEY (page_id, file_id)
);

CREATE TABLE IF NOT EXISTS page_versions (
  page_id TEXT NOT NULL REFERENCES pages (id) ON DELETE CASCADE,
  number INTEGER NOT NULL,
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT '',
  sub TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (page_id, number)
);

ALTER TABLE page_versions ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN IF NOT EXISTS last_editor_sub TEXT NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- Row level security: app.tenant_id (SET LOCAL) mirrors workspaces.org_id from IdP.
-- When app.tenant_id is unset (NULL), policies allow all rows (migration / Unscoped store).

CREATE OR REPLACE FUNCTION app_tenant_matches(org_id TEXT) RETURNS BOOLEAN AS $$
  SELECT current_setting('app.tenant_id', true) IS NULL
      OR org_id = current_setting('app.tenant_id', true);
$$ LANGUAGE sql STABLE;

CREATE OR REPLACE FUNCTION app_workspace_tenant_matches(ws_id TEXT) RETURNS BOOLEAN AS $$
  SELECT current_setting('app.tenant_id', true) IS NULL
      OR EXISTS (
        SELECT 1 FROM workspaces w
        WHERE w.id = ws_id AND app_tenant_matches(w.org_id)
      );
$$ LANGUAGE sql STABLE;

ALTER TABLE workspaces ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS workspaces_tenant ON workspaces;
CREATE POLICY workspaces_tenant ON workspaces
  FOR ALL
  USING (app_tenant_matches(org_id))
  WITH CHECK (app_tenant_matches(org_id));

ALTER TABLE members ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS members_tenant ON members;
CREATE POLICY members_tenant ON members
  FOR ALL
  USING (app_workspace_tenant_matches(workspace_id))
  WITH CHECK (app_workspace_tenant_matches(workspace_id));

ALTER TABLE invitations ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS invitations_tenant ON invitations;
CREATE POLICY invitations_tenant ON invitations
  FOR ALL
  USING (app_workspace_tenant_matches(workspace_id))
  WITH CHECK (app_workspace_tenant_matches(workspace_id));

ALTER TABLE audit_events ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS audit_events_tenant ON audit_events;
CREATE POLICY audit_events_tenant ON audit_events
  FOR ALL
  USING (app_workspace_tenant_matches(workspace_id))
  WITH CHECK (app_workspace_tenant_matches(workspace_id));

ALTER TABLE boards ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS boards_tenant ON boards;
CREATE POLICY boards_tenant ON boards
  FOR ALL
  USING (app_workspace_tenant_matches(workspace_id))
  WITH CHECK (app_workspace_tenant_matches(workspace_id));

ALTER TABLE columns ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS columns_tenant ON columns;
CREATE POLICY columns_tenant ON columns
  FOR ALL
  USING (
    current_setting('app.tenant_id', true) IS NULL
    OR EXISTS (
      SELECT 1 FROM boards b
      JOIN workspaces w ON w.id = b.workspace_id
      WHERE b.id = columns.board_id AND app_tenant_matches(w.org_id)
    )
  )
  WITH CHECK (
    current_setting('app.tenant_id', true) IS NULL
    OR EXISTS (
      SELECT 1 FROM boards b
      JOIN workspaces w ON w.id = b.workspace_id
      WHERE b.id = columns.board_id AND app_tenant_matches(w.org_id)
    )
  );

ALTER TABLE sprints ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS sprints_tenant ON sprints;
CREATE POLICY sprints_tenant ON sprints
  FOR ALL
  USING (app_workspace_tenant_matches(workspace_id))
  WITH CHECK (app_workspace_tenant_matches(workspace_id));

ALTER TABLE cards ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS cards_tenant ON cards;
CREATE POLICY cards_tenant ON cards
  FOR ALL
  USING (
    current_setting('app.tenant_id', true) IS NULL
    OR EXISTS (
      SELECT 1 FROM boards b
      JOIN workspaces w ON w.id = b.workspace_id
      WHERE b.id = cards.board_id AND app_tenant_matches(w.org_id)
    )
  )
  WITH CHECK (
    current_setting('app.tenant_id', true) IS NULL
    OR EXISTS (
      SELECT 1 FROM boards b
      JOIN workspaces w ON w.id = b.workspace_id
      WHERE b.id = cards.board_id AND app_tenant_matches(w.org_id)
    )
  );

ALTER TABLE pages ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS pages_tenant ON pages;
CREATE POLICY pages_tenant ON pages
  FOR ALL
  USING (app_workspace_tenant_matches(workspace_id))
  WITH CHECK (app_workspace_tenant_matches(workspace_id));

ALTER TABLE documents ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS documents_tenant ON documents;
CREATE POLICY documents_tenant ON documents
  FOR ALL
  USING (app_workspace_tenant_matches(workspace_id))
  WITH CHECK (app_workspace_tenant_matches(workspace_id));

ALTER TABLE channels ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS channels_tenant ON channels;
CREATE POLICY channels_tenant ON channels
  FOR ALL
  USING (app_workspace_tenant_matches(workspace_id))
  WITH CHECK (app_workspace_tenant_matches(workspace_id));

ALTER TABLE chat_messages ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS chat_messages_tenant ON chat_messages;
CREATE POLICY chat_messages_tenant ON chat_messages
  FOR ALL
  USING (
    current_setting('app.tenant_id', true) IS NULL
    OR EXISTS (
      SELECT 1 FROM channels c
      JOIN workspaces w ON w.id = c.workspace_id
      WHERE c.id = chat_messages.channel_id AND app_tenant_matches(w.org_id)
    )
  )
  WITH CHECK (
    current_setting('app.tenant_id', true) IS NULL
    OR EXISTS (
      SELECT 1 FROM channels c
      JOIN workspaces w ON w.id = c.workspace_id
      WHERE c.id = chat_messages.channel_id AND app_tenant_matches(w.org_id)
    )
  );

ALTER TABLE stored_files ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS stored_files_tenant ON stored_files;
CREATE POLICY stored_files_tenant ON stored_files
  FOR ALL
  USING (app_workspace_tenant_matches(workspace_id))
  WITH CHECK (app_workspace_tenant_matches(workspace_id));

ALTER TABLE page_files ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS page_files_tenant ON page_files;
CREATE POLICY page_files_tenant ON page_files
  FOR ALL
  USING (
    current_setting('app.tenant_id', true) IS NULL
    OR EXISTS (
      SELECT 1 FROM pages p
      JOIN workspaces w ON w.id = p.workspace_id
      WHERE p.id = page_files.page_id AND app_tenant_matches(w.org_id)
    )
  )
  WITH CHECK (
    current_setting('app.tenant_id', true) IS NULL
    OR EXISTS (
      SELECT 1 FROM pages p
      JOIN workspaces w ON w.id = p.workspace_id
      WHERE p.id = page_files.page_id AND app_tenant_matches(w.org_id)
    )
  );

ALTER TABLE page_versions ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS page_versions_tenant ON page_versions;
CREATE POLICY page_versions_tenant ON page_versions
  FOR ALL
  USING (
    current_setting('app.tenant_id', true) IS NULL
    OR EXISTS (
      SELECT 1 FROM pages p
      JOIN workspaces w ON w.id = p.workspace_id
      WHERE p.id = page_versions.page_id AND app_tenant_matches(w.org_id)
    )
  )
  WITH CHECK (
    current_setting('app.tenant_id', true) IS NULL
    OR EXISTS (
      SELECT 1 FROM pages p
      JOIN workspaces w ON w.id = p.workspace_id
      WHERE p.id = page_versions.page_id AND app_tenant_matches(w.org_id)
    )
  );
