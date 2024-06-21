CREATE TABLE tbl_page_task (
    id      SERIAL  NOT NULL,
    pageUrl VARCHAR(255) NOT NULL,
    downloadStatus BIT(2) NOT NULL DEFAULT 0,
    storePath VARCHAR(255) NOT NULL,
);

CREATE TABLE tbl_comics (
    id          SERIAL       NOT NULL,
    comicUrl    VARCHAR(255) NOT NULL,
    title       VARCHAR(127),
    coverUrl    VARCHAR(255),
    desc        TEXT,
    isFinished  BOOLEAN NOT NULL DEFAULT 0,
    status      BIT(2)  NOT NULL DEFAULT 0,
    create_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    update_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);