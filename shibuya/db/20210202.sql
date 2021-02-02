use shibuya;

CREATE TABLE IF NOT EXISTS project_context
(
    project_id INT UNSIGNED NOT NULL,
    context varchar(100),
    cluster_id varchar(100),
    key(project_id)
)CHARSET=utf8mb4;