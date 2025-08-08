package models

import "time"

type Key struct {
	ID               int64     `db:"id" json:"id"`
	Name             string    `db:"name" json:"name"`
	Armored          string    `db:"armored" json:"armored"`
	IsPrivate        bool      `db:"is_private" json:"is_private"`
	EncryptedPasshex *string   `db:"encrypted_password" json:"encrypted_password"`
	PasswordBcrypt   *string   `db:"password_bcrypt" json:"password_bcrypt"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
}
