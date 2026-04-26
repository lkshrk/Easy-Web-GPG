package app

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/ProtonMail/gopenpgp/v3/crypto"

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

	encHandle, err := crypto.PGP().Encryption().Recipient(kp).New()
	if err != nil {
		slog.Error("encrypt: failed to build encryption handle", "key_id", keyID, "name", k.Name, "err", err)
		http.Error(w, "failed to prepare encryption: "+err.Error(), http.StatusInternalServerError)
		return
	}
	pgpMsg, err := encHandle.Encrypt([]byte(plaintext))
	if err != nil {
		slog.Error("PGP encryption failed", "key_id", keyID, "err", err)
		http.Error(w, "encryption failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	armored, err := pgpMsg.Armor()
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

	if !strings.HasPrefix(strings.TrimSpace(input), "-----BEGIN PGP MESSAGE-----") {
		http.Error(w, "invalid armored message", http.StatusUnprocessableEntity)
		return
	}

	decHandle, err := crypto.PGP().Decryption().DecryptionKey(keyToUse).New()
	if err != nil {
		slog.Error("decrypt: failed to build decryption handle", "key_id", keyID, "name", k.Name, "err", err)
		http.Error(w, "failed to prepare decryption: "+err.Error(), http.StatusInternalServerError)
		return
	}
	decResult, err := decHandle.Decrypt([]byte(input), crypto.Armor)
	if err != nil {
		slog.Error("PGP decryption failed", "key_id", keyID, "err", err)
		http.Error(w, "decryption failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(decResult.Bytes())
}
