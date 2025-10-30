-- Init script for Backup Database
USE backup_db;

-- Tabel-tabel akan dibuat otomatis oleh schema sync service
-- File ini hanya untuk inisialisasi database kosong

-- Log
SELECT 'Backup database initialized successfully (empty - tables will be synced automatically)' AS message;
