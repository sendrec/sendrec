CREATE TABLE segment_engagement (
    video_id UUID NOT NULL REFERENCES videos(id),
    segment_index SMALLINT NOT NULL CHECK (segment_index >= 0 AND segment_index < 50),
    watch_count INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (video_id, segment_index)
);
