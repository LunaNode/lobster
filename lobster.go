package lobster

import "github.com/gorilla/context"
import "github.com/gorilla/mux"
import "github.com/gorilla/schema"

import "github.com/LunaNode/lobster/i18n"
import "github.com/LunaNode/lobster/websockify"
import "github.com/LunaNode/lobster/wssh"

import crand "crypto/rand"
import "encoding/binary"
import "log"
import "math/rand"
import "net/http"
import "strings"
import "sync"
import "time"

var decoder *schema.Decoder
var cfg *Config
var L *i18n.Section
var LA i18n.SectionFunc

var router *mux.Router
var db *Database
var wsMutex sync.Mutex
var ws *websockify.Websockify
var ssh *wssh.Wssh

func LobsterHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer errorHandler(w, r, true)

		// replace proxy IP if set
		if cfg.Default.ProxyHeader != "" {
			actualIP := r.Header.Get(cfg.Default.ProxyHeader)
			if actualIP != "" {
				r.RemoteAddr = actualIP
			}
		}

		if cfg.Default.Debug {
			log.Printf("Request [%s] %s %s", r.RemoteAddr, r.Method, r.URL)
		}

		h.ServeHTTP(w, r)
	})
}

func RedirectHandler(target string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, 302)
	}
}

func LogAction(userId int, ip string, name string, details string) {
	db.Exec("INSERT INTO actions (user_id, ip, name, details) VALUES (?, ?, ?, ?)", userId, ip, name, details)
}

func RegisterSplashRoute(path string, template string) {
	router.HandleFunc(path, getSplashHandler(template))
}

func RegisterPanelHandler(path string, f PanelHandlerFunc, onlyPost bool) {
	result := router.HandleFunc(path, SessionWrap(panelWrap(f)))
	if onlyPost {
		result.Methods("POST")
	}
}

func RegisterAPIHandler(path string, f APIHandlerFunc, method string) {
	router.HandleFunc(path, apiWrap(f)).Methods(method)
}

func RegisterAdminHandler(path string, f AdminHandlerFunc, onlyPost bool) {
	result := router.HandleFunc(path, SessionWrap(adminWrap(f)))
	if onlyPost {
		result.Methods("POST")
	}
}

func RegisterHttpHandler(path string, f http.HandlerFunc, onlyPost bool) {
	result := router.HandleFunc(path, f)
	if onlyPost {
		result.Methods("POST")
	}
}

func RegisterVmInterface(region string, vmi VmInterface) {
	if regionInterfaces[region] != nil {
		log.Fatalf("Duplicate VM interface for region %s", region)
	}
	regionInterfaces[region] = vmi
}

func RegisterPaymentInterface(method string, payInterface PaymentInterface) {
	if paymentInterfaces[method] != nil {
		log.Fatalf("Duplicate payment interface for method %s", method)
	}
	paymentInterfaces[method] = payInterface
}

func GetConfig() *Config {
	return cfg
}

func GetDatabase() *Database {
	return db
}

func GetDecoder() *schema.Decoder {
	return decoder
}

// Creates websockify instance if not already setup, initializes token, and returns URL to redirect to
func HandleWebsockify(ipport string, password string) string {
	wsMutex.Lock()
	defer wsMutex.Unlock()

	if ws == nil {
		ws = &websockify.Websockify{
			Debug:  cfg.Default.Debug,
			Listen: cfg.Novnc.Listen,
		}
		ws.Run()
	}

	token := ws.Register(ipport)
	return strings.Replace(strings.Replace(cfg.Novnc.Url, "TOKEN", token, 1), "PASSWORD", password, 1)
}

// Creates wssh instance if not already setup, initializes token, and returns URL to redirect to
func HandleWssh(ipport string, username string, password string) string {
	wsMutex.Lock()
	defer wsMutex.Unlock()

	if ssh == nil {
		ssh = &wssh.Wssh{
			Debug:  cfg.Default.Debug,
			Listen: cfg.Wssh.Listen,
		}
		ssh.Run()
	}

	token := ssh.Register(ipport, username, password)
	return strings.Replace(cfg.Wssh.Url, "TOKEN", token, 1)
}

func Setup(cfgPath string) {
	cfg = LoadConfig(cfgPath)
	router = mux.NewRouter()
	db = MakeDatabase()

	lang, err := i18n.LoadFile("language/" + cfg.Default.Language + ".json")
	checkErr(err)
	LA = lang.S
	L = LA("lobster")

	loadTemplates()
	loadEmail()
	loadPanelWidgets()

	decoder = schema.NewDecoder()
	decoder.IgnoreUnknownKeys(true)

	// splash/static routes
	router.HandleFunc("/message", getSplashHandler("splash_message"))
	router.HandleFunc("/login", SessionWrap(getSplashFormHandler("login")))
	router.HandleFunc("/create", SessionWrap(getSplashFormHandler("create")))
	router.HandleFunc("/pwreset", SessionWrap(authPwresetHandler))
	router.Handle("/assets/{path:.*}", http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets/"))))
	router.NotFoundHandler = http.HandlerFunc(splashNotFoundHandler)

	// auth routes
	router.HandleFunc("/auth/login", SessionWrap(authLoginHandler)).Methods("POST")
	router.HandleFunc("/auth/create", SessionWrap(authCreateHandler)).Methods("POST")
	router.HandleFunc("/auth/logout", SessionWrap(authLogoutHandler))
	router.HandleFunc("/auth/pwreset_request", SessionWrap(authPwresetRequestHandler)).Methods("POST")
	router.HandleFunc("/auth/pwreset_submit", SessionWrap(authPwresetSubmitHandler)).Methods("POST")

	// panel routes
	router.HandleFunc("/panel{slash:/*}", RedirectHandler("/panel/dashboard"))
	RegisterPanelHandler("/panel/dashboard", panelDashboard, false)
	RegisterPanelHandler("/panel/vms", panelVirtualMachines, false)
	RegisterPanelHandler("/panel/newvm", panelNewVM, false)
	RegisterPanelHandler("/panel/newvm/{region:[^/]+}", panelNewVMRegion, false)
	RegisterPanelHandler("/panel/vm/{id:[0-9]+}", panelVM, false)
	RegisterPanelHandler("/panel/vm/{id:[0-9]+}/start", panelVMStart, true)
	RegisterPanelHandler("/panel/vm/{id:[0-9]+}/stop", panelVMStop, true)
	RegisterPanelHandler("/panel/vm/{id:[0-9]+}/reboot", panelVMReboot, true)
	RegisterPanelHandler("/panel/vm/{id:[0-9]+}/delete", panelVMDelete, true)
	RegisterPanelHandler("/panel/vm/{id:[0-9]+}/action/{action:[^/]+}", panelVMAction, true)
	RegisterPanelHandler("/panel/vm/{id:[0-9]+}/vnc", panelVMVnc, false)
	RegisterPanelHandler("/panel/vm/{id:[0-9]+}/reimage", panelVMReimage, true)
	RegisterPanelHandler("/panel/vm/{id:[0-9]+}/rename", panelVMRename, true)
	RegisterPanelHandler("/panel/vm/{id:[0-9]+}/snapshot", panelVMSnapshot, true)
	RegisterPanelHandler("/panel/vm/{id:[0-9]+}/resize", panelVMResize, true)
	RegisterPanelHandler("/panel/billing", panelBilling, false)
	RegisterPanelHandler("/panel/pay", panelPay, false)
	RegisterPanelHandler("/panel/charges", panelCharges, false)
	RegisterPanelHandler("/panel/charges/{year:[0-9]+}/{month:[0-9]+}", panelCharges, false)
	RegisterPanelHandler("/panel/account", panelAccount, false)
	RegisterPanelHandler("/panel/account/passwd", panelAccountPassword, true)
	RegisterPanelHandler("/panel/api/add", panelApiAdd, true)
	RegisterPanelHandler("/panel/api/{id:[0-9]+}/remove", panelApiRemove, true)
	RegisterPanelHandler("/panel/images", panelImages, false)
	RegisterPanelHandler("/panel/images/add", panelImageAdd, true)
	RegisterPanelHandler("/panel/image/{id:[0-9]+}", panelImageDetails, false)
	RegisterPanelHandler("/panel/image/{id:[0-9]+}/remove", panelImageRemove, true)
	RegisterPanelHandler("/panel/csrftoken", panelToken, false)

	// api routes
	RegisterAPIHandler("/api/vms", apiVMList, "GET")
	RegisterAPIHandler("/api/vms", apiVMCreate, "POST")
	RegisterAPIHandler("/api/vms/{id:[0-9]+}", apiVMInfo, "GET")
	RegisterAPIHandler("/api/vms/{id:[0-9]+}/action", apiVMAction, "POST")
	RegisterAPIHandler("/api/vms/{id:[0-9]+}/reimage", apiVMReimage, "POST")
	RegisterAPIHandler("/api/vms/{id:[0-9]+}/resize", apiVMResize, "POST")
	RegisterAPIHandler("/api/vms/{id:[0-9]+}", apiVMDelete, "DELETE")
	RegisterAPIHandler("/api/vms/{id:[0-9]+}/ips", apiVMAddresses, "GET")
	RegisterAPIHandler("/api/vms/{id:[0-9]+}/ips/add", apiVMAddressAdd, "POST")
	RegisterAPIHandler("/api/vms/{id:[0-9]+}/ips/remove", apiVMAddressRemove, "POST") // use POST instead of DELETE since we need both public/private ip
	RegisterAPIHandler("/api/vms/{id:[0-9]+}/ips/{ip:[^/]+}/rdns", apiVMAddressRdns, "POST")
	RegisterAPIHandler("/api/images", apiImageList, "GET")
	RegisterAPIHandler("/api/images", apiImageFetch, "POST")
	RegisterAPIHandler("/api/images/{id:[0-9]+}", apiImageInfo, "GET")
	RegisterAPIHandler("/api/images/{id:[0-9]+}", apiImageDelete, "DELETE")
	RegisterAPIHandler("/api/plans", apiPlanList, "GET")

	// admin routes
	RegisterAdminHandler("/admin/dashboard", adminDashboard, false)
	RegisterAdminHandler("/admin/users", adminUsers, false)
	RegisterAdminHandler("/admin/user/{id:[0-9]+}", adminUser, false)
	RegisterAdminHandler("/admin/user/{id:[0-9]+}/login", adminUserLogin, true)
	RegisterAdminHandler("/admin/user/{id:[0-9]+}/credit", adminUserCredit, true)
	RegisterAdminHandler("/admin/user/{id:[0-9]+}/password", adminUserPassword, true)
	RegisterAdminHandler("/admin/user/{id:[0-9]+}/disable", adminUserDisable, true)
	RegisterAdminHandler("/admin/user/{id:[0-9]+}/enable", adminUserEnable, true)
	RegisterAdminHandler("/admin/vms", adminVirtualMachines, false)
	RegisterAdminHandler("/admin/vm/{id:[0-9]+}/suspend", adminVMSuspend, true)
	RegisterAdminHandler("/admin/vm/{id:[0-9]+}/unsuspend", adminVMUnsuspend, true)
	RegisterAdminHandler("/admin/plans", adminPlans, false)
	RegisterAdminHandler("/admin/plans/add", adminPlansAdd, true)
	RegisterAdminHandler("/admin/plans/autopopulate", adminPlansAutopopulate, true)
	RegisterAdminHandler("/admin/plan/{id:[0-9]+}", adminPlan, false)
	RegisterAdminHandler("/admin/plan/{id:[0-9]+}/delete", adminPlanDelete, true)
	RegisterAdminHandler("/admin/plan/{id:[0-9]+}/enable", adminPlanEnable, true)
	RegisterAdminHandler("/admin/plan/{id:[0-9]+}/disable", adminPlanDisable, true)
	RegisterAdminHandler("/admin/plan/{id:[0-9]+}/associate", adminPlanAssociateRegion, true)
	RegisterAdminHandler("/admin/plan/{id:[0-9]+}/deassociate/{region:[^/]+}", adminPlanDeassociateRegion, true)
	RegisterAdminHandler("/admin/regions", adminRegions, false)
	RegisterAdminHandler("/admin/region/{region:[^/]+}/enable", adminRegionEnable, true)
	RegisterAdminHandler("/admin/region/{region:[^/]+}/disable", adminRegionDisable, true)
	RegisterAdminHandler("/admin/images", adminImages, false)
	RegisterAdminHandler("/admin/images/add", adminImagesAdd, true)
	RegisterAdminHandler("/admin/image/{id:[0-9]+}/delete", adminImageDelete, true)
	RegisterAdminHandler("/admin/images/autopopulate", adminImagesAutopopulate, true)

	// seed math/rand via crypt/rand in case interfaces want to use it for non-secure randomness source
	// (math/rand is much faster)
	seedBytes := make([]byte, 8)
	_, err = crand.Read(seedBytes)
	if err != nil {
		log.Printf("Warning: failed to seed math/rand: %s", err.Error())
	} else {
		seed, _ := binary.Varint(seedBytes)
		rand.Seed(seed)
	}
}

func Run() {
	// fake cron routine
	go func() {
		for {
			cron()
			time.Sleep(time.Minute)
		}
	}()

	go func() {
		for {
			cached()
			time.Sleep(5 * time.Second)
		}
	}()

	httpServer := &http.Server{
		Addr:    cfg.Http.Addr,
		Handler: LobsterHandler(context.ClearHandler(router)),
	}
	log.Fatal(httpServer.ListenAndServe())
}

func cron() {
	defer errorHandler(nil, nil, true)
	vmRows := db.Query("SELECT id FROM vms WHERE time_billed < DATE_SUB(NOW(), INTERVAL ? HOUR)", BILLING_VM_FREQUENCY)
	defer vmRows.Close()
	for vmRows.Next() {
		var vmId int
		vmRows.Scan(&vmId)
		vmBilling(vmId, false)
	}

	userRows := db.Query("SELECT id FROM users WHERE last_billing_notify < DATE_SUB(NOW(), INTERVAL ? HOUR)", cfg.BillingNotifications.Frequency)
	defer userRows.Close()
	for userRows.Next() {
		var userId int
		userRows.Scan(&userId)
		userBilling(userId)
	}

	serviceBilling()

	// cleanup
	db.Exec("DELETE FROM form_tokens WHERE time < DATE_SUB(NOW(), INTERVAL 1 HOUR)")
	db.Exec("DELETE FROM sessions WHERE active_time < DATE_SUB(NOW(), INTERVAL 1 HOUR)")
	db.Exec("DELETE FROM antiflood WHERE time < DATE_SUB(NOW(), INTERVAL 2 HOUR)")
	db.Exec("DELETE FROM pwreset_tokens WHERE time < DATE_SUB(NOW(), INTERVAL ? MINUTE)", PWRESET_EXPIRE_MINUTES)
}

func cached() {
	defer errorHandler(nil, nil, true)
	rows := db.Query("SELECT id, user_id FROM images WHERE status = 'pending' ORDER BY RAND() LIMIT 3")
	defer rows.Close()
	for rows.Next() {
		var imageId, userId int
		rows.Scan(&imageId, &userId)
		imageInfo := imageInfo(userId, imageId)

		if imageInfo != nil {
			if imageInfo.Info.Status == ImageError {
				db.Exec("UPDATE images SET status = ? WHERE id = ?", "error", imageId)
			} else if imageInfo.Info.Status == ImageActive {
				db.Exec("UPDATE images SET status = ? WHERE id = ?", "active", imageId)
			}
		}
	}
}
