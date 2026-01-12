ALTER TABLE videos ADD COLUMN music VARCHAR(255) DEFAULT '';
ALTER TABLE videos ADD COLUMN tags VARCHAR(255) DEFAULT '';
CREATE INDEX idx_videos_music ON videos(music);
CREATE INDEX idx_videos_tags ON videos(tags);