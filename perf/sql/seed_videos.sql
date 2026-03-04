USE dancemirror;

SET @uid := 2;

DELETE FROM videos WHERE userId = @uid;

SET @row := 0;

INSERT INTO videos
  (userId, title, description, filePath, objectKey, fileName, fileSize, status, storagePath, createdAt, updatedAt)
SELECT
  @uid,
  CONCAT('perf_video_', t.n),
  'seeded for perf test',
  CONCAT('/uploads/perf/', @uid, '/', t.n, '_seed.mp4'),
  CONCAT('videos/', @uid, '/', t.n, '_seed.mp4'),
  CONCAT(t.n, '_seed.mp4'),
  123456,
  'ready',
  CONCAT('videos/', @uid, '/', t.n, '_seed.mp4'),
  NOW(3),
  NOW(3)
FROM (
  SELECT (@row := @row + 1) AS n
  FROM information_schema.COLUMNS c1
  CROSS JOIN information_schema.COLUMNS c2
  LIMIT 5000
) AS t;

SELECT COUNT(*) AS cnt FROM videos WHERE userId = @uid;