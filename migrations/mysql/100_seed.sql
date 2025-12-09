INSERT IGNORE INTO users (id, email, name, password_hash, role)
VALUES ('00000000-0000-0000-0000-000000000001', 'admin@kidpech.app', 'Demo Admin', '$2a$12$LbhpmsYNIQP5CKEM5Qn2jOBYx8RJWb1x1My1t4bm/F/6HC8K3oprm', 'admin');

INSERT IGNORE INTO profiles (id, user_id, first_name, last_name, bio, created_at, updated_at)
VALUES ('10000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'Demo', 'Admin', 'System seeded profile', NOW(), NOW());
