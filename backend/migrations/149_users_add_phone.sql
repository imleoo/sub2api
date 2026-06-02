-- 手机号注册：users 表新增 phone 列和唯一索引
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone VARCHAR(20);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone ON users (phone) WHERE phone IS NOT NULL;
