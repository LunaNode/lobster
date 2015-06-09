package lobster

import "crypto/rand"
import "encoding/hex"
import "net/http"
import "log"

type Session struct {
	Id string
	UserId int
	Admin bool
	OriginalId int // user id prior to logging in as another user
	Regenerate bool
}

func (this *Session) clone() *Session {
	return &Session{
		Id: this.Id,
		UserId: this.UserId,
		Admin: this.Admin,
		OriginalId: this.OriginalId,
		Regenerate: this.Regenerate,
	}
}
func (this *Session) IsLoggedIn() bool {
	return this.UserId != 0
}
func (this *Session) Reset() {
	this.UserId = 0
	this.Admin = false
	this.OriginalId = 0
}

func sessionWrap(handler func(w http.ResponseWriter, r *http.Request, db *Database, session *Session)) func(w http.ResponseWriter, r *http.Request, db *Database) {
	return func(w http.ResponseWriter, r *http.Request, db *Database) {
		r.ParseForm()

		// determine session identifier and grab session; or generate a new one
		var session *Session
		sessionNew := false
		sessionCookie, err := r.Cookie(SESSION_COOKIE_NAME)
		if err == nil {
			sessionIdentifier := sessionCookie.Value
			rows := db.Query("SELECT user_id, admin, original_id, regenerate FROM sessions WHERE uid = ? AND active_time > DATE_SUB(NOW(), INTERVAL 1 HOUR)", sessionIdentifier)
			if rows.Next() {
				session = &Session{Id: sessionIdentifier}
				rows.Scan(&session.UserId, &session.Admin, &session.OriginalId, &session.Regenerate)
			} else {
				// invalid session identifier! need to generate new session
				log.Printf("Invalid session identifier from %s", r.RemoteAddr)
				session = makeSession(w, db)
				sessionNew = true
			}
		} else {
			session = makeSession(w, db)
			sessionNew = true
		}

		// regenerate session if needed
		if session.Regenerate {
			newSessionIdentifier := generateSessionIdentifier(w)
			db.Exec("UPDATE sessions SET uid = ?, regenerate = 0 WHERE uid = ?", newSessionIdentifier, session.Id)
			session.Id = newSessionIdentifier
			session.Regenerate = false
		}

		// CSRF protection
		if r.Method == "POST" && !csrfCheck(db, session, r.PostForm.Get("token")) {
			log.Printf("Invalid CSRF token from %s", r.RemoteAddr)
			http.Redirect(w, r, "/panel/dashboard", 303)
			return
		}

		// call handler but remember current session
		originalSession := session.clone()
		handler(w, r, db, session)

		// writeback session
		if originalSession.UserId == 0 && session.UserId != 0 && !sessionNew {
			// we just logged in on an old session, regenerate the session ID first chance we get
			// note that we can't do it immediately since template has been written already
			session.Regenerate = true
		}
		db.Exec("UPDATE sessions SET user_id = ?, admin = ?, original_id = ?, regenerate = ?, active_time = NOW() WHERE uid = ?", session.UserId, session.Admin, session.OriginalId, session.Regenerate, session.Id)
	}
}

func makeSession(w http.ResponseWriter, db *Database) *Session {
	newSession := &Session{Id: generateSessionIdentifier(w)}
	db.Exec("INSERT INTO sessions (uid, user_id, admin, original_id, regenerate) VALUES (?, ?, ?, ?, ?)", newSession.Id, newSession.UserId, newSession.Admin, newSession.OriginalId, newSession.Regenerate)
	return newSession
}

func generateSessionIdentifier(w http.ResponseWriter) string {
	r := make([]byte, SESSION_UID_LENGTH / 2)
	_, err := rand.Read(r)
	checkErr(err)
	sessionIdentifier := hex.EncodeToString(r)
	http.SetCookie(w, &http.Cookie{
		Name: SESSION_COOKIE_NAME,
		Value: sessionIdentifier,
		Path: "/",
		Domain: cfg.Session.Domain,
		Secure: cfg.Session.Secure,
	})
	return sessionIdentifier
}

func csrfGenerate(db *Database, session *Session) string {
	tokenBytes := make([]byte, 32)
	_, err := rand.Read(tokenBytes)
	checkErr(err)
	token := hex.EncodeToString(tokenBytes)
	db.Exec("INSERT INTO form_tokens (session_uid, token) VALUES (?, ?)", session.Id, token)
	return token
}

func csrfCheck(db *Database, session *Session, token string) bool {
	var numMatch int
	db.QueryRow("SELECT COUNT(*) FROM form_tokens WHERE session_uid = ? AND token = ?", session.Id, token).Scan(&numMatch)

	if numMatch == 0 {
		return false
	} else {
		db.Exec("DELETE FROM form_tokens WHERE token = ?", token)
		return true
	}
}
