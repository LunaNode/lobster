package lobster

import "github.com/asaskevich/govalidator"
import "golang.org/x/crypto/pbkdf2"

import "crypto/rand"
import "crypto/sha512"
import "crypto/subtle"
import "encoding/hex"
import "errors"
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
	if email != "" && !govalidator.IsEmail(email) {
		return 0, errors.New("invalid email address")
	}

	if len(username) < MIN_USERNAME_LENGTH || len(username) > MAX_USERNAME_LENGTH {
		return 0, errors.New(fmt.Sprintf("usernames must be between %d and %d characters long", MIN_USERNAME_LENGTH, MAX_USERNAME_LENGTH))
	}

	if !isPrintable(username) {
		return 0, errors.New(fmt.Sprintf("provided username contains invalid characters"))
	}

	if len(password) < MIN_PASSWORD_LENGTH || len(password) > MAX_PASSWORD_LENGTH {
		return 0, errors.New(fmt.Sprintf("passwords must be between %d and %d characters long", MIN_PASSWORD_LENGTH, MAX_PASSWORD_LENGTH))
	}

	// ensure username not taken already
	var userCount int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", username).Scan(&userCount)
	if userCount > 0 {
		return 0, errors.New("username is already taken")
	}

	// ensure email not taken already
	if email != "" {
		db.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", email).Scan(&userCount)
		if userCount > 0 {
			return 0, errors.New("email address is already in use")
		}
	}

	// antiflood
	if !antifloodCheck(db, ip, "authCreate", 3) {
		return 0, errors.New("try again later")
	}

	// generate salt and hash password
	result := db.Exec("INSERT INTO users (username, password, email) VALUES (?, ?, ?)", username, authMakePassword(password), email)
	userId, _ := result.LastInsertId()
	LogAction(db, int(userId), ip, "Registered account", "")
	antifloodAction(db, ip, "authCreate")
	mailWrap(db, -1, "accountCreated", AccountCreatedEmail{UserId: int(userId), Username: username, Email: email}, false)
	return int(userId), nil
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
		return 0, errors.New("invalid username or password")
	} else if !antifloodCheck(db, ip, "authCheck", 12) {
		return 0, errors.New("try again later")
	}

	rows := db.Query("SELECT id, password FROM users WHERE username = ? AND status != 'disabled'", username)
	if !rows.Next() {
		log.Printf("Authentication failure on user=%s: bad username (%s)", username, ip)
		antifloodAction(db, ip, "authCheck")
		return 0, errors.New("invalid username or password")
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
		antifloodAction(db, ip, "authCheck")
		log.Printf("Authentication failure on user=%s: bad password (%s)", username, ip)
		return 0, errors.New("invalid username or password")
	}
}

func authChangePassword(db *Database, ip string, userId int, oldPassword string, newPassword string) error {
	if len(newPassword) < MIN_PASSWORD_LENGTH || len(newPassword) > MAX_PASSWORD_LENGTH {
		return errors.New(fmt.Sprintf("passwords must be between %d and %d characters long", MIN_PASSWORD_LENGTH, MAX_PASSWORD_LENGTH))
	} else if !antifloodCheck(db, ip, "authCheck", 12) {
		return errors.New("try again later")
	}

	rows := db.Query("SELECT password FROM users WHERE id = ?", userId)
	if !rows.Next() {
		antifloodAction(db, ip, "authCheck")
		log.Printf("Error changing password: bad user ID?! (%d/%s)", userId ,ip)
		return errors.New("invalid account")
	}
	var actualPasswordCombined string
	rows.Scan(&actualPasswordCombined)
	rows.Close()

	if authCheckPassword(oldPassword, actualPasswordCombined) {
		db.Exec("UPDATE users SET password = ? WHERE id = ?", authMakePassword(newPassword), userId)
		log.Printf("Successful password change for user_id=%d (%s)", userId, ip)
		LogAction(db, userId, ip, "Change password", "")
		mailWrap(db, userId, "authChangePassword", nil, false)
		return nil
	} else {
		antifloodAction(db, ip, "authCheck")
		log.Printf("Change password authentication failure for user_id=%d (%s)", userId, ip)
		return errors.New("incorrect password provided")
	}
}

func authForceChangePassword(db *Database, userId int, password string) {
	db.Exec("UPDATE users SET password = ? WHERE id = ?", authMakePassword(password), userId)
}

type AuthLoginForm struct {
	Username string `schema:"username"`
	Password string `schema:"password"`
}

func authLoginHandler(w http.ResponseWriter, r *http.Request, db *Database, session *Session) {
	if session.IsLoggedIn() {
		redirectMessage(w, r, "/panel/dashboard", "You are already logged in.")
		return
	}

	form := new(AuthLoginForm)
	err := decoder.Decode(form, r.PostForm)
	if err != nil {
		http.Redirect(w, r, "/login", 303)
		return
	}

	userId, err := authLogin(db, extractIP(r.RemoteAddr), form.Username, form.Password)
	if err != nil {
		errorMessage := err.Error()
		prettyError := strings.ToUpper(errorMessage[:1]) + errorMessage[1:] + "."
		redirectMessage(w, r, "/login", prettyError)
		return
	} else {
		session.UserId = userId
		http.Redirect(w, r, "/panel/dashboard", 303)

		user := userDetails(db, userId)
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
		redirectMessage(w, r, "/panel/dashboard", "You are already logged in.")
		return
	}

	form := new(AuthCreateForm)
	err := decoder.Decode(form, r.PostForm)
	if err != nil {
		http.Redirect(w, r, "/create", 303)
		return
	}

	if form.AcceptTerms != "yes" {
		redirectMessage(w, r, "/create", "You must agree to the terms of service to register an account.")
		return
	}

	userId, err := authCreate(db, extractIP(r.RemoteAddr), form.Username, form.Password, form.Email)
	if err != nil {
		redirectMessage(w, r, "/create", "Error: " + err.Error() + ".")
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
