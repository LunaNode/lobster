package lobster

import "github.com/gorilla/context"
import "github.com/gorilla/mux"
import "github.com/gorilla/schema"

import "github.com/LunaNode/lobster/websockify"

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

type Lobster struct {
	router *mux.Router
	db *Database

	wsMutex sync.Mutex
	ws *websockify.Websockify
}

func LobsterHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func LogAction(db *Database, userId int, ip string, name string, details string) {
	db.Exec("INSERT INTO actions (user_id, ip, name, details) VALUES (?, ?, ?, ?)", userId, ip, name, details)
}

func MakeLobster(cfgPath string) *Lobster {
	this := new(Lobster)

	cfg = LoadConfig(cfgPath)
	this.router = mux.NewRouter()
	this.db = MakeDatabase()

	return this
}

func (this *Lobster) RegisterPanelHandler(path string, f PanelHandlerFunc, onlyPost bool) {
	result := this.router.HandleFunc(path, this.db.wrapHandler(sessionWrap(panelWrap(f))))
	if onlyPost {
		result.Methods("POST")
	}
}

func (this *Lobster) RegisterAPIHandler(path string, f APIHandlerFunc, method string) {
	this.router.HandleFunc(path, this.db.wrapHandler(apiWrap(f))).Methods(method)
}

func (this *Lobster) RegisterAdminHandler(path string, f AdminHandlerFunc, onlyPost bool) {
	result := this.router.HandleFunc(path, this.db.wrapHandler(sessionWrap(adminWrap(f))))
	if onlyPost {
		result.Methods("POST")
	}
}

func (this *Lobster) RegisterHttpHandler(path string, f http.HandlerFunc, onlyPost bool) {
	result := this.router.HandleFunc(path, f)
	if onlyPost {
		result.Methods("POST")
	}
}

func (this *Lobster) RegisterVmInterface(region string, vmi VmInterface) {
	if regionInterfaces[region] != nil {
		log.Fatalf("Duplicate VM interface for region %s", region)
	}
	regionInterfaces[region] = vmi
}

func (this *Lobster) RegisterPaymentInterface(method string, payInterface PaymentInterface) {
	if paymentInterfaces[method] != nil {
		log.Fatalf("Duplicate payment interface for method %s", method)
	}
	paymentInterfaces[method] = payInterface
}

func (this *Lobster) GetConfig() *Config {
	return cfg
}

func (this *Lobster) GetDatabase() *Database {
	return this.db
}

// Creates websockify instance if not already setup, initializes token, and returns URL to redirect to
func (this *Lobster) HandleWebsockify(ipport string, password string) string {
	this.wsMutex.Lock()
	defer this.wsMutex.Unlock()

	if this.ws == nil {
		this.ws = &websockify.Websockify{
			Listen: cfg.Novnc.Listen,
		}
		this.ws.Run()
	}

	token := this.ws.Register(ipport)
	return strings.Replace(strings.Replace(cfg.Novnc.Url, "TOKEN", token, 1), "PASSWORD", password, 1)
}

func (this *Lobster) Init() {
	loadTemplates()
	loadEmail()

	decoder = schema.NewDecoder()
	decoder.IgnoreUnknownKeys(true)

	// splash/static routes
	this.router.HandleFunc("/", getSplashHandler("index"))
	this.router.HandleFunc("/about", getSplashHandler("about"))
	this.router.HandleFunc("/pricing", getSplashHandler("pricing"))
	this.router.HandleFunc("/contact", getSplashHandler("contact"))
	this.router.HandleFunc("/terms", getSplashHandler("terms"))
	this.router.HandleFunc("/privacy", getSplashHandler("privacy"))
	this.router.HandleFunc("/login", this.db.wrapHandler(sessionWrap(getSplashFormHandler("login"))))
	this.router.HandleFunc("/create", this.db.wrapHandler(sessionWrap(getSplashFormHandler("create"))))
	this.router.Handle("/assets/{path:.*}", http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets/"))))
	this.router.NotFoundHandler = http.HandlerFunc(splashNotFoundHandler)

	// auth routes
	this.router.HandleFunc("/auth/login", this.db.wrapHandler(sessionWrap(authLoginHandler))).Methods("POST")
	this.router.HandleFunc("/auth/create", this.db.wrapHandler(sessionWrap(authCreateHandler))).Methods("POST")
	this.router.HandleFunc("/auth/logout", this.db.wrapHandler(sessionWrap(authLogoutHandler)))

	// panel routes
	this.router.HandleFunc("/panel{slash:/*}", RedirectHandler("/panel/dashboard"))
	this.RegisterPanelHandler("/panel/dashboard", panelDashboard, false)
	this.RegisterPanelHandler("/panel/vms", panelVirtualMachines, false)
	this.RegisterPanelHandler("/panel/newvm", panelNewVM, false)
	this.RegisterPanelHandler("/panel/newvm/{region:[^/]+}", panelNewVMRegion, false)
	this.RegisterPanelHandler("/panel/vm/{id:[0-9]+}", panelVM, false)
	this.RegisterPanelHandler("/panel/vm/{id:[0-9]+}/start", panelVMStart, true)
	this.RegisterPanelHandler("/panel/vm/{id:[0-9]+}/stop", panelVMStop, true)
	this.RegisterPanelHandler("/panel/vm/{id:[0-9]+}/reboot", panelVMReboot, true)
	this.RegisterPanelHandler("/panel/vm/{id:[0-9]+}/delete", panelVMDelete, true)
	this.RegisterPanelHandler("/panel/vm/{id:[0-9]+}/action/{action:[^/]+}", panelVMAction, true)
	this.RegisterPanelHandler("/panel/vm/{id:[0-9]+}/vnc", panelVMVnc, false)
	this.RegisterPanelHandler("/panel/vm/{id:[0-9]+}/reimage", panelVMReimage, true)
	this.RegisterPanelHandler("/panel/vm/{id:[0-9]+}/rename", panelVMRename, true)
	this.RegisterPanelHandler("/panel/vm/{id:[0-9]+}/snapshot", panelVMSnapshot, true)
	this.RegisterPanelHandler("/panel/billing", panelBilling, false)
	this.RegisterPanelHandler("/panel/pay", panelPay, false)
	this.RegisterPanelHandler("/panel/charges", panelCharges, false)
	this.RegisterPanelHandler("/panel/charges/{year:[0-9]+}/{month:[0-9]+}", panelCharges, false)
	this.RegisterPanelHandler("/panel/account", panelAccount, false)
	this.RegisterPanelHandler("/panel/account/passwd", panelAccountPassword, true)
	this.RegisterPanelHandler("/panel/api/add", panelApiAdd, true)
	this.RegisterPanelHandler("/panel/api/{id:[0-9]+}/remove", panelApiRemove, true)
	this.RegisterPanelHandler("/panel/images", panelImages, false)
	this.RegisterPanelHandler("/panel/images/add", panelImageAdd, true)
	this.RegisterPanelHandler("/panel/image/{id:[0-9]+}", panelImageDetails, false)
	this.RegisterPanelHandler("/panel/image/{id:[0-9]+}/remove", panelImageRemove, true)
	this.RegisterPanelHandler("/panel/support", panelSupport, false)
	this.RegisterPanelHandler("/panel/support/open", panelSupportOpen, false)
	this.RegisterPanelHandler("/panel/support/{id:[0-9]+}", panelSupportTicket, false)
	this.RegisterPanelHandler("/panel/support/{id:[0-9]+}/reply", panelSupportTicketReply, true)
	this.RegisterPanelHandler("/panel/support/{id:[0-9]+}/close", panelSupportTicketClose, true)
	this.RegisterPanelHandler("/panel/csrftoken", panelToken, false)

	// api routes
	this.RegisterAPIHandler("/api/vms", apiVMList, "GET")
	this.RegisterAPIHandler("/api/vms", apiVMCreate, "POST")
	this.RegisterAPIHandler("/api/vms/{id:[0-9]+}", apiVMInfo, "GET")
	this.RegisterAPIHandler("/api/vms/{id:[0-9]+}/action", apiVMAction, "POST")
	this.RegisterAPIHandler("/api/vms/{id:[0-9]+}/reimage", apiVMReimage, "POST")
	this.RegisterAPIHandler("/api/vms/{id:[0-9]+}", apiVMDelete, "DELETE")
	this.RegisterAPIHandler("/api/vms/{id:[0-9]+}/ips", apiVMAddresses, "GET")
	this.RegisterAPIHandler("/api/vms/{id:[0-9]+}/ips/add", apiVMAddressAdd, "POST")
	this.RegisterAPIHandler("/api/vms/{id:[0-9]+}/ips/remove", apiVMAddressRemove, "POST") // use POST instead of DELETE since we need both public/private ip
	this.RegisterAPIHandler("/api/vms/{id:[0-9]+}/ips/{ip:[^/]+}/rdns", apiVMAddressRdns, "POST")
	this.RegisterAPIHandler("/api/images", apiImageList, "GET")
	this.RegisterAPIHandler("/api/images", apiImageFetch, "POST")
	this.RegisterAPIHandler("/api/images/{id:[0-9]+}", apiImageInfo, "GET")
	this.RegisterAPIHandler("/api/images/{id:[0-9]+}", apiImageDelete, "DELETE")
	this.RegisterAPIHandler("/api/plans", apiPlanList, "GET")

	// admin routes
	this.RegisterAdminHandler("/admin/dashboard", adminDashboard, false)
	this.RegisterAdminHandler("/admin/users", adminUsers, false)
	this.RegisterAdminHandler("/admin/user/{id:[0-9]+}", adminUser, false)
	this.RegisterAdminHandler("/admin/user/{id:[0-9]+}/login", adminUserLogin, true)
	this.RegisterAdminHandler("/admin/user/{id:[0-9]+}/credit", adminUserCredit, true)
	this.RegisterAdminHandler("/admin/user/{id:[0-9]+}/password", adminUserPassword, true)
	this.RegisterAdminHandler("/admin/user/{id:[0-9]+}/disable", adminUserDisable, true)
	this.RegisterAdminHandler("/admin/support", adminSupport, false)
	this.RegisterAdminHandler("/admin/support/open/{id:[0-9]+}", adminSupportOpen, false)
	this.RegisterAdminHandler("/admin/support/{id:[0-9]+}", adminSupportTicket, false)
	this.RegisterAdminHandler("/admin/support/{id:[0-9]+}/reply", adminSupportTicketReply, true)
	this.RegisterAdminHandler("/admin/support/{id:[0-9]+}/close", adminSupportTicketClose, true)
	this.RegisterAdminHandler("/admin/plans", adminPlans, false)
	this.RegisterAdminHandler("/admin/plans/add", adminPlansAdd, false)
	this.RegisterAdminHandler("/admin/plan/{id:[0-9]+}/delete", adminPlanDelete, true)
	this.RegisterAdminHandler("/admin/images", adminImages, false)
	this.RegisterAdminHandler("/admin/images/add", adminImagesAdd, false)
	this.RegisterAdminHandler("/admin/image/{id:[0-9]+}/delete", adminImageDelete, true)

	// seed math/rand via crypt/rand in case interfaces want to use it for non-secure randomness source
	// (math/rand is much faster)
	seedBytes := make([]byte, 8)
	_, err := crand.Read(seedBytes)
	if err != nil {
		log.Printf("Warning: failed to seed math/rand: %s", err.Error())
	} else {
		seed, _ := binary.Varint(seedBytes)
		rand.Seed(seed)
	}
}

func (this *Lobster) Run() {
	// fake cron routine
	go func() {
		for {
			this.cron()
			time.Sleep(time.Minute)
		}
	}()

	go func() {
		for {
			this.cached()
			time.Sleep(5 * time.Second)
		}
	}()

	httpServer := &http.Server{
		Addr: cfg.Http.Addr,
		Handler: LobsterHandler(context.ClearHandler(this.router)),
	}
	log.Fatal(httpServer.ListenAndServe())
}

func (this *Lobster) cron() {
	defer errorHandler(nil, nil, true)
	vmRows := this.db.Query("SELECT id FROM vms WHERE time_billed < DATE_SUB(NOW(), INTERVAL ? HOUR)", BILLING_VM_FREQUENCY)
	defer vmRows.Close()
	for vmRows.Next() {
		var vmId int
		vmRows.Scan(&vmId)
		vmBilling(this.db, vmId, false)
	}

	userRows := this.db.Query("SELECT id FROM users WHERE last_billing_notify < DATE_SUB(NOW(), INTERVAL 24 HOUR)")
	defer userRows.Close()
	for userRows.Next() {
		var userId int
		userRows.Scan(&userId)
		userBilling(this.db, userId)
	}

	serviceBilling(this.db)

	// cleanup
	this.db.Exec("DELETE FROM form_tokens WHERE time < DATE_SUB(NOW(), INTERVAL 1 HOUR)")
	this.db.Exec("DELETE FROM sessions WHERE active_time < DATE_SUB(NOW(), INTERVAL 1 HOUR)")
	this.db.Exec("DELETE FROM antiflood WHERE time < DATE_SUB(NOW(), INTERVAL 2 HOUR)")
}

func (this *Lobster) cached() {
	defer errorHandler(nil, nil, true)
	rows := this.db.Query("SELECT id, user_id FROM images WHERE status = 'pending' ORDER BY RAND() LIMIT 3")
	defer rows.Close()
	for rows.Next() {
		var imageId, userId int
		rows.Scan(&imageId, &userId)
		imageInfo := imageInfo(this.db, userId, imageId)

		if imageInfo != nil {
			if imageInfo.Info.Status == ImageError {
				this.db.Exec("UPDATE images SET status = ? WHERE id = ?", "error", imageId)
			} else if imageInfo.Info.Status == ImageActive {
				this.db.Exec("UPDATE images SET status = ? WHERE id = ?", "active", imageId)
			}
		}
	}
}
