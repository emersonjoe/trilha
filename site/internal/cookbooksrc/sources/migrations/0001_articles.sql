CREATE TABLE articles (
	id           BIGSERIAL PRIMARY KEY,
	slug         TEXT NOT NULL UNIQUE,
	title        TEXT NOT NULL,
	published_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX articles_published_idx ON articles (published_at DESC, id DESC);
