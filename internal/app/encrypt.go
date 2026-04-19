package app

import (
	"log"
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
		http.Error(w, "key not found", http.StatusUnprocessableEntity)
		return
	}

	kp, err := crypto.NewKeyFromArmored(k.Armored)
	if err != nil {
		http.Error(w, "invalid key", http.StatusUnprocessableEntity)
		return
	}
	pub, err := kp.GetArmoredPublicKey()
	if err != nil {
		http.Error(w, "failed to get public key", http.StatusInternalServerError)
		return
	}
	pubKeyObj, err := crypto.NewKeyFromArmored(pub)
	if err != nil {
		http.Error(w, "failed to parse public key", http.StatusInternalServerError)
		return
	}

	recipientKR, err := crypto.NewKeyRing(pubKeyObj)
	if err != nil {
		http.Error(w, "failed to create keyring", http.StatusInternalServerError)
		return
	}

	message := crypto.NewPlainMessageFromString(plaintext)
	pgpMsg, err := recipientKR.Encrypt(message, nil)
	if err != nil {
		log.Printf("encrypt error: %v", err)
		http.Error(w, "encryption failed", http.StatusInternalServerError)
		return
	}

	armored, err := pgpMsg.GetArmored()
	if err != nil {
		http.Error(w, "failed to armor message", http.StatusInternalServerError)
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
		http.Error(w, "key not found", http.StatusUnprocessableEntity)
		return
	}

	if !k.IsPrivate {
		http.Error(w, "selected key is not a private key", http.StatusUnprocessableEntity)
		return
	}

	priv, err := crypto.NewKeyFromArmored(k.Armored)
	if err != nil {
		http.Error(w, "invalid private key", http.StatusUnprocessableEntity)
		return
	}

	locked, err := priv.IsLocked()
	if err != nil {
		http.Error(w, "failed to inspect private key", http.StatusInternalServerError)
		return
	}

	var keyToUse *crypto.Key
	if locked {
		if k.EncryptedPasshex != nil && *k.EncryptedPasshex != "" {
			pwBytes, err := a.Crypto.Decrypt(*k.EncryptedPasshex)
			if err != nil {
				http.Error(w, "failed to decrypt stored password", http.StatusInternalServerError)
				return
			}
			unlocked, err := priv.Unlock(pwBytes)
			if err != nil {
				http.Error(w, "failed to unlock private key with stored password", http.StatusUnprocessableEntity)
				return
			}
			keyToUse = unlocked
		} else {
			http.Error(w, "private key is password protected; no stored password", http.StatusUnprocessableEntity)
			return
		}
	} else {
		keyToUse = priv
	}

	kr, err := crypto.NewKeyRing(keyToUse)
	if err != nil {
		http.Error(w, "failed to create keyring", http.StatusInternalServerError)
		return
	}

	encMessage, err := crypto.NewPGPMessageFromArmored(input)
	if err != nil {
		http.Error(w, "invalid armored message", http.StatusUnprocessableEntity)
		return
	}

	decrypted, err := kr.Decrypt(encMessage, nil, 0)
	if err != nil {
		log.Printf("decrypt error: %v", err)
		http.Error(w, "decryption failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(decrypted.GetString()))
}
