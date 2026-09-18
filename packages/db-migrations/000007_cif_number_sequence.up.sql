-- CBS Migration 000007: Sequence nomor CIF
-- Run after: 000006_customer_encryption_and_schedule.up.sql
--
-- Nomor CIF sebelumnya memakai timestamp/nanosecond yang bisa bertabrakan pada
-- pendaftaran bersamaan. Sequence database menjamin keunikan dan urutan.

CREATE SEQUENCE IF NOT EXISTS cif_number_seq START 1 INCREMENT 1;
