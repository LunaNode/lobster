package lobster

import "github.com/LunaNode/lobster/utils"

import "golang.org/x/crypto/pbkdf2"

import "crypto/rand"
import "crypto/sha512"
import "crypto/subtle"
import "encoding/hex"
import "fmt"
import "log"
import "net/http"
import "strings"

func authMakePassword(password string) string {
	salt := make([]byte, 16)
	_, err := rand.Read(salt)
	checkErr(err)
	passwordHash := pbkdf2.Key([]byte(password), salt, 8192, 64, sha512.New)
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(passwordHash)
}

func authCreate(db *Database, ip string, username string, password string, email string) (int, error) {
	if !AntifloodCheck(db, ip, "authCreate", 3) {
		return 0, L.Error("try_again_later")
	}

	userId, err := UserCreate(db, username, password, email)
	if err != nil {
		return 0, err
	}

	LogAction(db, userId, ip, "Registered account", "")
	AntifloodAction(db, ip, "authCreate")
	MailWrap(db, -1, "accountCreated", AccountCreatedEmail{UserId: int(userId), Username: username, Email: email}, false)
	return userId, nil
}

func authCheckPassword(password string, actualPasswordCombined string) bool {
	// grab salt/password parts
	passwordParts := strings.Split(actualPasswordCombined, ":")
	salt, _ := hex.DecodeString(passwordParts[0])
	actualPasswordHash, _ := hex.DecodeString(passwordParts[1])

	// derive key from provided password and match
	providedPasswordHash := pbkdf2.Key([]byte(password), salt, 8192, 64, sha512.New)
	return subtle.ConstantTimeCompare(actualPasswordHash, providedPasswordHash) == 1
}

func authLogin(db *Database, ip string, username string, password string) (int, error) {
	if len(password) > MAX_PASSWORD_LENGTH {
		return 0, L.Error("incorrect_username_or_password")
	} else if !AntifloodCheck(db, ip, "authCheck", 12) {
		return 0, L.Error("try_again_later")
	}

	rows := db.Query("SELECT id, password FROM users WHERE username = ? AND status != 'disabled'", username)
	if !rows.Next() {
		log.Printf("Authentication failure on user=%s: bad username (%s)", username, ip)
		AntifloodAction(db, ip, "authCheck")
		return 0, L.Error("incorrect_username_or_password")
	}
	var userId int
	var actualPasswordCombined string
	rows.Scan(&userId, &actualPasswordCombined)
	rows.Close()

	if authCheckPassword(password, actualPasswordCombined) {
		log.Printf("Authentication successful for user=%s (%s)", username, ip)
		LogAction(db, userId, ip, "Logged in", "")
		return userId, nil
	} else {
		AntifloodAction(db, ip, "authCheck")
		log.Printf("Authentication failure on user=%s: bad password (%s)", username, ip)
		return 0, L.Error("incorrect_username_or_password")
	}
}

func authChangePassword(db *Database, ip string, userId int, oldPassword string, newPassword string) error {
	if len(newPassword) < MIN_PASSWORD_LENGTH || len(newPassword) > MAX_PASSWORD_LENGTH {
		return L.Errorf("password_length", MIN_PASSWORD_LENGTH, MAX_PASSWORD_LENGTH)
	} else if !AntifloodCheck(db, ip, "authCheck", 12) {
		return L.Error("try_again_later")
	}

	rows := db.Query("SELECT password FROM users WHERE id = ?", userId)
	if !rows.Next() {
		AntifloodAction(db, ip, "authCheck")
		log.Printf("Error changing password: bad user ID?! (%d/%s)", userId ,ip)
		return L.Error("invalid_account")
	}
	var actualPasswordCombined string
	rows.Scan(&actualPasswordCombined)
	rows.Close()

	if authCheckPassword(oldPassword, actualPasswordCombined) {
		db.Exec("UPDATE users SET password = ? WHERE id = ?", authMakePassword(newPassword), userId)
		log.Printf("Successful password change for user_id=%d (%s)", userId, ip)
		LogAction(db, userId, ip, "Change password", "")
		MailWrap(db, userId, "authChangePassword", nil, false)
		return nil
	} else {
		AntifloodAction(db, ip, "authCheck")
		log.Printf("Change password authentication failure for user_id=%d (%s)", userId, ip)
		return L.Error("incorrect_password")
	}
}

func authForceChangePassword(db *Database, userId int, password string) {
	db.Exec("UPDATE users SET password = ? WHERE id = ?", authMakePassword(password), userId)
}

func authPwresetRequest(db *Database, ip string, username string, email string) error {
	if email == "" {
		return L.Error("pwreset_email_required")
	} else if !AntifloodCheck(db, ip, "pwresetRequest", 10) {
		return L.Error("try_again_later")
	}
	AntifloodAction(db, ip, "pwresetRequest") // mark antiflood regardless of whether success/failure

	rows := db.Query("SELECT id FROM users WHERE username = ? AND email = ?", username, email)
	if !rows.Next() {
		return L.Error("incorrect_username_email")
	}
	var userId int
	rows.Scan(&userId)
	rows.Close()

	// make sure not already active pwreset for this user
	var count int
	db.QueryRow("SELECT COUNT(*) FROM pwreset_tokens WHERE user_id = ?", userId).Scan(&count)
	if count > 0 {
		return L.Error("pwreset_outstanding")
	}

	token := utils.Uid(32)
	db.Exec("INSERT INTO pwreset_tokens (user_id, token) VALUES (?, ?)", userId, token)
	MailWrap(db, userId, "pwresetRequest", token, false)
	return nil
}

func authPwresetSubmit(db *Database, ip string, userId int, token string, password string) error {
	if !AntifloodCheck(db, ip, "pwresetSubmit", 10) {
		return L.Error("try_again_later")
	} else if len(password) < MIN_PASSWORD_LENGTH || len(password) > MAX_PASSWORD_LENGTH {
		return L.Errorf("password_length", MIN_PASSWORD_LENGTH, MAX_PASSWORD_LENGTH)
	}
	AntifloodAction(db, ip, "pwresetSubmit") // mark antiflood regardless of whether success/failure

	rows := db.Query("SELECT id FROM pwreset_tokens WHERE user_id = ? AND token = ? AND time > DATE_SUB(NOW(), INTERVAL ? MINUTE)", userId, token, PWRESET_EXPIRE_MINUTES)
	if !rows.Next() {
		return L.Error("incorrect_token")
	}
	var tokenId int
	rows.Scan(&tokenId)
	rows.Close()

	db.Exec("DELETE FROM pwreset_tokens WHERE id = ?", tokenId)
	db.Exec("UPDATE users SET password = ? WHERE id = ?", authMakePassword(password), userId)
	log.Printf("Successful password reset for user_id=%d (%s)", userId, ip)
	LogAction(db, userId, ip, "Reset password", "")
	MailWrap(db, userId, "authChangePassword", nil, false)
	return nil
}

type AuthLoginForm struct {
	Username string `schema:"username"`
	Password string `schema:"password"`
}

func authLoginHandler(w http.ResponseWriter, r *http.Request, db *Database, session *Session) {
	if session.IsLoggedIn() {
		RedirectMessage(w, r, "/panel/dashboard", L.Info("already_logged_in"))
		return
	}

	form := new(AuthLoginForm)
	err := decoder.Decode(form, r.PostForm)
	if err != nil {
		http.Redirect(w, r, "/login", 303)
		return
	}

	userId, err := authLogin(db, ExtractIP(r.RemoteAddr), form.Username, form.Password)
	if err != nil {
		RedirectMessage(w, r, "/login", L.FormatError(err))
		return
	} else {
		session.UserId = userId
		http.Redirect(w, r, "/panel/dashboard", 303)

		user := UserDetails(db, userId)
		if user != nil && user.Admin {
			session.Admin = true
		}
	}
}

type AuthCreateForm struct {
	Username string `schema:"username"`
	Password string `schema:"password"`
	Email string `schema:"email"`
	AcceptTerms string `schema:"acceptTermsOfService"`
}

func authCreateHandler(w http.ResponseWriter, r *http.Request, db *Database, session *Session) {
	if session.IsLoggedIn() {
		RedirectMessage(w, r, "/panel/dashboard", L.Info("already_logged_in"))
		return
	}

	form := new(AuthCreateForm)
	err := decoder.Decode(form, r.PostForm)
	if err != nil {
		http.Redirect(w, r, "/create", 303)
		return
	}

	if form.AcceptTerms != "yes" {
		RedirectMessage(w, r, "/create", L.FormattedError("must_terms"))
		return
	}

	userId, err := authCreate(db, ExtractIP(r.RemoteAddr), form.Username, form.Password, form.Email)
	if err != nil {
		RedirectMessage(w, r, "/create", L.FormatError(err))
		return
	} else {
		session.UserId = userId
		http.Redirect(w, r, "/panel/dashboard", 303)
	}
}

func authLogoutHandler(w http.ResponseWriter, r *http.Request, db *Database, session *Session) {
	session.Reset()
	http.Redirect(w, r, "/login", 303)
}

type AuthPwresetParams struct {
	Title string
	Message string
	Token string
	PwresetUserId string
	PwresetToken string
}

func authPwresetHandler(w http.ResponseWriter, r *http.Request, db *Database, session *Session) {
	message := ""
	if r.URL.Query()["message"] != nil {
		message = r.URL.Query()["message"][0]
	}
	params := AuthPwresetParams{
		Message: message,
		Token: CSRFGenerate(db, session),
	}

	if r.URL.Query().Get("user_id") != "" && r.URL.Query().Get("token") != "" {
		params.PwresetUserId = r.URL.Query().Get("user_id")
		params.PwresetToken = r.URL.Query().Get("token")
		RenderTemplate(w, "splash", "pwreset_submit", params)
	} else {
		RenderTemplate(w, "splash", "pwreset_request", params)
	}
}

type AuthPwresetRequestForm struct {
	Username string `schema:"username"`
	Email string `schema:"email"`
}

func authPwresetRequestHandler(w http.ResponseWriter, r *http.Request, db *Database, session *Session) {
	if session.IsLoggedIn() {
		RedirectMessage(w, r, "/panel/dashboard", L.Info("already_logged_in"))
		return
	}

	form := new(AuthPwresetRequestForm)
	err := decoder.Decode(form, r.PostForm)
	if err != nil {
		http.Redirect(w, r, "/pwreset", 303)
		return
	}

	err = authPwresetRequest(db, ExtractIP(r.RemoteAddr), form.Username, form.Email)
	if err != nil {
		RedirectMessage(w, r, "/pwreset", L.FormatError(err))
		return
	} else {
		RedirectMessage(w, r, "/message", L.Success("pwreset_requested"))
	}
}

type AuthPwresetSubmitForm struct {
	UserId int `schema:"pwreset_user_id"`
	Token string `schema:"pwreset_token"`
	Password string `schema:"password"`
	PasswordConfirm string `schema:"password_confirm"`
}

func authPwresetSubmitHandler(w http.ResponseWriter, r *http.Request, db *Database, session *Session) {
	if session.IsLoggedIn() {
		RedirectMessage(w, r, "/panel/dashboard", L.Info("already_logged_in"))
		return
	}

	form := new(AuthPwresetSubmitForm)
	err := decoder.Decode(form, r.PostForm)
	if err != nil {
		http.Redirect(w, r, "/pwreset", 303)
		return
	} else if form.Password != form.PasswordConfirm {
		RedirectMessageExtra(w, r, "/pwreset", L.FormattedError("password_mismatch"), map[string]string{"user_id": fmt.Sprintf("%d", form.UserId), "token": form.Token})
	}

	err = authPwresetSubmit(db, ExtractIP(r.RemoteAddr), form.UserId, form.Token, form.Password)
	if err != nil {
		RedirectMessageExtra(w, r, "/pwreset", L.FormatError(err), map[string]string{"user_id": fmt.Sprintf("%d", form.UserId), "token": form.Token})
		return
	} else {
		RedirectMessage(w, r, "/message", L.Success("pwreset_completed"))
	}
}
