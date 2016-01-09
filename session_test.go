package lobster

import "bytes"
import "net/http"
import "net/http/httptest"
import "net/url"
import "testing"

func TestSessionBasic(t *testing.T) {
	var seenUserId int
	fakeHandler := func(w http.ResponseWriter, r *http.Request, session *Session) {
		seenUserId = session.UserId
	}

	TestReset()
	userId := TestUser()
	w := httptest.NewRecorder()
	session := makeSession(w)
	req, _ := http.NewRequest("GET", "http://example.com/", nil)
	req.AddCookie(&http.Cookie{Name: SESSION_COOKIE_NAME, Value: session.Id})
	db.Exec("UPDATE sessions SET user_id = ?", userId)
	SessionWrap(fakeHandler)(w, req)

	if seenUserId != userId {
		t.Fatalf("Expected session user id %d but got %d", userId, seenUserId)
	}
}

func findResponseCookie(response *http.Response, name string) string {
	for _, cookie := range response.Cookies() {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func TestSessionLogin(t *testing.T) {
	// create session, apply on fake handler, then login
	// verify session ID changes after first request after login
	//   (check both cookie and that new handler on old session doesn't get logged in)
	// then verify in next handler that session preserves user ID
	// note: we use server for this test since it makes parsing Set-Cookie easier
	TestReset()
	userId := TestUser()

	var seenUserId int
	fakeHandler := func(w http.ResponseWriter, r *http.Request, session *Session) {
		seenUserId = session.UserId
	}

	loginHandler := func(w http.ResponseWriter, r *http.Request, session *Session) {
		session.UserId = userId
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			SessionWrap(fakeHandler)(w, r)
		} else if r.URL.Path == "/login" {
			SessionWrap(loginHandler)(w, r)
		} else {
			t.Errorf("Unexpected request path %s", r.URL.Path)
		}
	})
	server := httptest.NewServer(handler)

	// initial request
	response, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	} else if seenUserId != 0 {
		t.Fatal("Initial request already shows logged in user")
	}
	initialSessionId := findResponseCookie(response, SESSION_COOKIE_NAME)
	if initialSessionId == "" {
		t.Fatal("No session cookie provided")
	}

	// login
	request, _ := http.NewRequest("GET", server.URL+"/login", nil)
	request.AddCookie(&http.Cookie{Name: SESSION_COOKIE_NAME, Value: initialSessionId})
	response, err = new(http.Client).Do(request)
	if err != nil {
		t.Fatal(err)
	}

	// get arbitrary page, expect both to be logged in and for server to regenerate session id
	request, _ = http.NewRequest("GET", server.URL, nil)
	request.AddCookie(&http.Cookie{Name: SESSION_COOKIE_NAME, Value: initialSessionId})
	response, err = new(http.Client).Do(request)
	if err != nil {
		t.Fatal(err)
	} else if seenUserId != userId {
		t.Fatal("First page after login with initial session should be logged in, but isn't")
	}
	loginSessionId := findResponseCookie(response, SESSION_COOKIE_NAME)
	if loginSessionId == "" {
		t.Fatal("No session cookie provided on first request after login")
	} else if loginSessionId == initialSessionId {
		t.Fatal("Session cookie remains the same on first request after login")
	}

	// verify old session not logged in
	request, _ = http.NewRequest("GET", server.URL, nil)
	request.AddCookie(&http.Cookie{Name: SESSION_COOKIE_NAME, Value: initialSessionId})
	response, err = new(http.Client).Do(request)
	if err != nil {
		t.Fatal(err)
	} else if seenUserId != 0 {
		t.Fatal("Session from before login is logged in")
	}

	// verify new session is logged in
	request, _ = http.NewRequest("GET", server.URL, nil)
	request.AddCookie(&http.Cookie{Name: SESSION_COOKIE_NAME, Value: loginSessionId})
	response, err = new(http.Client).Do(request)
	if err != nil {
		t.Fatal(err)
	} else if seenUserId != userId {
		t.Fatal("Session from after login is not logged in correctly")
	}
}

func TestSessionCSRF(t *testing.T) {
	// try no token, valid token, reuse token, and other session token
	// only valid token should work
	// on fail we expect 303 redirect
	TestReset()
	w := httptest.NewRecorder()
	session := makeSession(w)
	fakeHandler := func(w http.ResponseWriter, r *http.Request, session *Session) {}

	// no token
	req, _ := http.NewRequest("POST", "http://example.com/", nil)
	req.AddCookie(&http.Cookie{Name: SESSION_COOKIE_NAME, Value: session.Id})
	w = httptest.NewRecorder()
	SessionWrap(fakeHandler)(w, req)

	if w.Code != 303 {
		t.Error("CSRF protection allowed no token")
	}

	// valid token
	v := url.Values{}
	v.Add("token", CSRFGenerate(session))
	req, _ = http.NewRequest("POST", "http://example.com/", bytes.NewBufferString(v.Encode()))
	req.AddCookie(&http.Cookie{Name: SESSION_COOKIE_NAME, Value: session.Id})
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	SessionWrap(fakeHandler)(w, req)

	if w.Code != 200 {
		t.Error("CSRF protection disallowed valid token")
	}

	// reuse token
	req, _ = http.NewRequest("POST", "http://example.com/", bytes.NewBufferString(v.Encode()))
	req.AddCookie(&http.Cookie{Name: SESSION_COOKIE_NAME, Value: session.Id})
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	SessionWrap(fakeHandler)(w, req)

	if w.Code != 303 {
		t.Error("CSRF protection allowed reused token")
	}

	// other session token
	session2 := makeSession(w)
	v = url.Values{}
	v.Add("token", CSRFGenerate(session2))
	req, _ = http.NewRequest("POST", "http://example.com/", bytes.NewBufferString(v.Encode()))
	req.AddCookie(&http.Cookie{Name: SESSION_COOKIE_NAME, Value: session.Id})
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	SessionWrap(fakeHandler)(w, req)

	if w.Code != 303 {
		t.Error("CSRF protection allowed token from another session")
	}
}
