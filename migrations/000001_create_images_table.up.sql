CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TYPE processing_status AS ENUM ('pending', 'processing', 'completed', 'failed');

CREATE TABLE IF NOT EXISTS images (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  original_name VARCHAR(255) NOT NULL,
  original_size BIGINT NOT NULL,
  original_width INTEGER NOT NULL,
  original_height INTEGER NOT NULL,
  original_format VARCHAR(50) NOT NULL,
  original_path TEXT NOT NULL,
  optimized_path TEXT,
  optimized_size BIGINT,
  optimized_width INTEGER,
  optimized_height INTEGER,
  status processing_status NOT NULL DEFAULT 'pending',
  error TEXT,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_images_status ON images (status);
CREATE INDEX idx_images_created_at ON images (created_at DESC);