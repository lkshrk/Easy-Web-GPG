package app

import (
	"log/slog"
	"net/http"

	"github.com/ProtonMail/gopenpgp/v2/crypto"

	mm "h-cloud.io/web-gpg/internal/models"
)

// EncryptHandler encrypts plaintext using the selected PGP key.
func (a *App) EncryptHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	keyID := r.FormValue("key")
	plaintext := r.FormValue("input")

	var k mm.Key
	q := a.DB.Rebind("SELECT id, name, armored, is_private, encrypted_password FROM keys WHERE id = ?")
	if err := a.DB.GetContext(r.Context(), &k, q, keyID); err != nil {
		slog.Warn("encrypt: key not found", "key_id", keyID, "err", err)
		http.Error(w, "key not found", http.StatusUnprocessableEntity)
		return
	}

	kp, err := crypto.NewKeyFromArmored(k.Armored)
	if err != nil {
		slog.Error("encrypt: failed to parse stored key", "key_id", keyID, "name", k.Name, "err", err)
		http.Error(w, "stored key is invalid: "+err.Error(), http.StatusInternalServerError)
		return
	}
	pub, err := kp.GetArmoredPublicKey()
	if err != nil {
		slog.Error("encrypt: failed to extract public key", "key_id", keyID, "name", k.Name, "err", err)
		http.Error(w, "failed to get public key: "+err.Error(), http.StatusInternalServerError)
		return
	}
	pubKeyObj, err := crypto.NewKeyFromArmored(pub)
	if err != nil {
		slog.Error("encrypt: failed to re-parse extracted public key", "key_id", keyID, "name", k.Name, "err", err)
		http.Error(w, "failed to parse public key: "+err.Error(), http.StatusInternalServerError)
		return
	}

	recipientKR, err := crypto.NewKeyRing(pubKeyObj)
	if err != nil {
		slog.Error("encrypt: failed to build keyring", "key_id", keyID, "name", k.Name, "err", err)
		http.Error(w, "failed to create keyring: "+err.Error(), http.StatusInternalServerError)
		return
	}

	message := crypto.NewPlainMessageFromString(plaintext)
	pgpMsg, err := recipientKR.Encrypt(message, nil)
	if err != nil {
		slog.Error("PGP encryption failed", "key_id", keyID, "err", err)
		http.Error(w, "encryption failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	armored, err := pgpMsg.GetArmored()
	if err != nil {
		slog.Error("encrypt: failed to armor ciphertext", "key_id", keyID, "err", err)
		http.Error(w, "failed to armor message: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(armored))
}

// DecryptHandler decrypts a PGP message using the selected private key.
func (a *App) DecryptHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	keyID := r.FormValue("key")
	input := r.FormValue("input")

	var k mm.Key
	q := a.DB.Rebind("SELECT id, name, armored, is_private, encrypted_password FROM keys WHERE id = ?")
	if err := a.DB.GetContext(r.Context(), &k, q, keyID); err != nil {
		slog.Warn("decrypt: key not found", "key_id", keyID, "err", err)
		http.Error(w, "key not found", http.StatusUnprocessableEntity)
		return
	}

	if !k.IsPrivate {
		http.Error(w, "selected key is not a private key", http.StatusUnprocessableEntity)
		return
	}

	priv, err := crypto.NewKeyFromArmored(k.Armored)
	if err != nil {
		slog.Error("decrypt: failed to parse stored private key", "key_id", keyID, "name", k.Name, "err", err)
		http.Error(w, "stored private key is invalid: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locked, err := priv.IsLocked()
	if err != nil {
		slog.Error("decrypt: failed to inspect private key lock state", "key_id", keyID, "name", k.Name, "err", err)
		http.Error(w, "failed to inspect private key: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var keyToUse *crypto.Key
	if locked {
		if k.EncryptedPasshex != nil && *k.EncryptedPasshex != "" {
			pwBytes, err := a.Crypto.Decrypt(*k.EncryptedPasshex)
			if err != nil {
				slog.Error("decrypt: failed to decrypt stored passphrase", "key_id", keyID, "name", k.Name, "err", err)
				http.Error(w, "failed to decrypt stored passphrase: "+err.Error(), http.StatusInternalServerError)
				return
			}
			unlocked, err := priv.Unlock(pwBytes)
			if err != nil {
				slog.Warn("decrypt: stored passphrase did not unlock private key", "key_id", keyID, "name", k.Name, "err", err)
				http.Error(w, "stored passphrase is wrong for this key: "+err.Error(), http.StatusUnprocessableEntity)
				return
			}
			keyToUse = unlocked
		} else {
			http.Error(w, "private key is passphrase-protected but no passphrase was stored", http.StatusUnprocessableEntity)
			return
		}
	} else {
		keyToUse = priv
	}

	kr, err := crypto.NewKeyRing(keyToUse)
	if err != nil {
		slog.Error("decrypt: failed to build keyring", "key_id", keyID, "name", k.Name, "err", err)
		http.Error(w, "failed to create keyring: "+err.Error(), http.StatusInternalServerError)
		return
	}

	encMessage, err := crypto.NewPGPMessageFromArmored(input)
	if err != nil {
		slog.Warn("decrypt: invalid armored message from user", "key_id", keyID, "err", err)
		http.Error(w, "invalid armored message: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}

	decrypted, err := kr.Decrypt(encMessage, nil, 0)
	if err != nil {
		slog.Error("PGP decryption failed", "key_id", keyID, "err", err)
		http.Error(w, "decryption failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(decrypted.GetString()))
}
