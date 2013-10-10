-- I know it is not normalized. But it is easier.
CREATE TABLE IF NOT EXISTS messages
(
	mid CHAR(255) NOT NULL PRIMARY KEY,

	owner_service CHAR(255) NOT NULL,
	owner_name CHAR(255) NOT NULL,

	sender_service CHAR(255),
	sender_name CHAR(255),

	create_time BIGINT,
	deadline BIGINT,
	content BLOB
);

CREATE INDEX idx_owner_time ON messages (owner_service, owner_name, create_time, deadline);

-- SELECT * FROM messages
-- WHERE mid = ?

-- SELECT * FROM messages
-- WHERE owner_service = ?
-- AND owner_name = ?
-- AND create_time > ?
-- AND deadline > NOW();
