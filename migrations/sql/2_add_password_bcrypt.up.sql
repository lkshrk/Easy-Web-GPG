-- migration: add password_bcrypt column to keys table
ALTER TABLE keys ADD COLUMN password_bcrypt TEXT;
