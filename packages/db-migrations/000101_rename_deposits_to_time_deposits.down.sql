-- 000101_rename_deposits_to_time_deposits.down.sql
ALTER TABLE IF EXISTS time_deposits RENAME TO deposits;
