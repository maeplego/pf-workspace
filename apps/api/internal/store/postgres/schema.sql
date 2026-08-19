-- Local / overlay demo schema. ULID ids (TEXT), timestamptz clocks.

CREATE TABLE IF NOT EXISTS workspaces (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS members (
  workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
  sub TEXT NOT NULL,
  role TEXT NOT NULL,
  joined_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (workspace_id, sub)
);

CREATE INDEX IF NOT EXISTS members_sub_idx ON members (sub);

CREATE TABLE IF NOT EXISTS boards (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

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

CREATE TABLE IF NOT EXISTS documents (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  body TEXT NOT NULL DEFAULT '',
  collab_document_id TEXT NOT NULL UNIQUE,
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
  sub TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (page_id, number)
);
