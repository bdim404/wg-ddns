package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	
	_ "github.com/fernvenue/wg-ddns/docs"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var logLevelNames = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
}

type Logger struct {
	level LogLevel
}

func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}
	
	timestamp := time.Now().Format("2006/01/02 15:04:05")
	levelName := logLevelNames[level]
	message := fmt.Sprintf(format, args...)
	fmt.Printf("%s [%s] %s\n", timestamp, levelName, message)
}

func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

var logger *Logger

func parseLogLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn", "warning":
		return WARN
	case "error":
		return ERROR
	default:
		return INFO
	}
}

type Config struct {
	Interface string
	Endpoint  string
	Hostname  string
	LastIP    net.IP
}

type DDNSMonitor struct {
	configs         []Config
	conn            *dbus.Conn
	singleInterface string
	apiEnabled      bool
	listenAddress   string
	listenPort      string
	apiKey          string
	httpServer      *http.Server
	checkInterval   time.Duration
}

type RestartRequest struct {
	Interface string `json:"interface" binding:"required"`
}

type RestartResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type Args struct {
	singleInterface string
	listenAddress   string
	listenPort      string
	apiKey          string
	logLevel        string
	checkInterval   string
	help            bool
}

func parseArgs() *Args {
	args := &Args{}
	
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		
		if !strings.HasPrefix(arg, "--") {
			if arg == "-h" || arg == "-help" {
				args.help = true
				continue
			}
			fmt.Fprintf(os.Stderr, "Error: Invalid argument format '%s'. Only double-dash (--) options are supported.\n", arg)
			os.Exit(1)
		}
		
		if arg == "--help" {
			args.help = true
			continue
		}
		
		parts := strings.SplitN(arg, "=", 2)
		var key, value string
		
		if len(parts) == 2 {
			key = parts[0]
			value = parts[1]
		} else {
			key = arg
			if i+1 < len(os.Args) && !strings.HasPrefix(os.Args[i+1], "--") {
				i++
				value = os.Args[i]
			}
		}
		
		switch key {
		case "--single-interface":
			args.singleInterface = value
		case "--listen-address":
			args.listenAddress = value
		case "--listen-port":
			args.listenPort = value
		case "--api-key":
			args.apiKey = value
		case "--log-level":
			args.logLevel = value
		case "--check-interval":
			args.checkInterval = value
		default:
			fmt.Fprintf(os.Stderr, "Error: Unknown option '%s'\n", key)
			os.Exit(1)
		}
	}
	
	return args
}

func printUsage() {
	fmt.Printf("Usage: %s [OPTIONS]\n\n", os.Args[0])
	fmt.Println("OPTIONS:")
	fmt.Println("  --single-interface string    Monitor only the specified WireGuard interface")
	fmt.Println("  --listen-address string      HTTP API listen address")
	fmt.Println("  --listen-port string         HTTP API listen port")
	fmt.Println("  --api-key string             API key for authentication")
	fmt.Println("  --log-level string           Log level: debug, info, warn, error (default: info)")
	fmt.Println("  --check-interval string      DNS check interval (e.g., 10s, 1m, 5m) (default: 10s)")
	fmt.Println("  --help                       Show this help message")
	fmt.Println("")
	fmt.Println("NOTES:")
	fmt.Println("  - All three API options (--listen-address, --listen-port, --api-key) must be provided together to enable API functionality")
	fmt.Println("  - Use double-dash (--) format for all options")
}

// @title WireGuard DDNS API
// @version 1.0
// @description API for WireGuard DDNS monitor
// @host localhost:8080
// @BasePath /api/v1
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
func main() {
	args := parseArgs()
	
	if args.help {
		printUsage()
		os.Exit(0)
	}

	logLevel := INFO
	if args.logLevel != "" {
		logLevel = parseLogLevel(args.logLevel)
	}
	
	logger = &Logger{level: logLevel}
	
	log.SetOutput(io.Discard)
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard

	checkInterval := 10 * time.Second
	if args.checkInterval != "" {
		var err error
		checkInterval, err = time.ParseDuration(args.checkInterval)
		if err != nil {
			logger.Error("Invalid check interval format: %v", err)
			os.Exit(1)
		}
		if checkInterval < time.Second {
			logger.Error("Check interval must be at least 1 second")
			os.Exit(1)
		}
	}

	apiEnabled := args.listenAddress != "" && args.listenPort != "" && args.apiKey != ""

	monitor := &DDNSMonitor{
		singleInterface: args.singleInterface,
		apiEnabled:      apiEnabled,
		listenAddress:   args.listenAddress,
		listenPort:      args.listenPort,
		apiKey:          args.apiKey,
		checkInterval:   checkInterval,
	}
	
	if err := monitor.initialize(); err != nil {
		logger.Error("Failed to initialize monitor: %v", err)
		os.Exit(1)
	}
	defer monitor.cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Received shutdown signal")
		cancel()
	}()

	if monitor.apiEnabled {
		go monitor.startHTTPServer(ctx)
	}

	logger.Info("WireGuard DDNS monitor started")
	monitor.run(ctx)
}

func (m *DDNSMonitor) initialize() error {
	var err error
	m.conn, err = dbus.NewWithContext(context.Background())
	if err != nil {
		return fmt.Errorf("failed to connect to systemd: %w", err)
	}

	if m.singleInterface != "" {
		return m.parseSingleInterface()
	}
	return m.discoverWireGuardConfigs()
}

func (m *DDNSMonitor) parseSingleInterface() error {
	configPath := filepath.Join("/etc/wireguard", m.singleInterface+".conf")
	if err := m.parseWireGuardConfig(m.singleInterface, configPath); err != nil {
		return fmt.Errorf("failed to parse config for %s: %w", m.singleInterface, err)
	}
	
	logger.Info("Monitoring single interface: %s with %d domain endpoints", m.singleInterface, len(m.configs))
	return nil
}

func (m *DDNSMonitor) cleanup() {
	if m.httpServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.httpServer.Shutdown(shutdownCtx)
	}
	if m.conn != nil {
		m.conn.Close()
	}
}

func (m *DDNSMonitor) discoverWireGuardConfigs() error {
	units, err := m.conn.ListUnitsContext(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list systemd units: %w", err)
	}

	for _, unit := range units {
		if strings.HasPrefix(unit.Name, "wg-quick@") && strings.HasSuffix(unit.Name, ".service") && unit.ActiveState == "active" {
			interfaceName := strings.TrimPrefix(unit.Name, "wg-quick@")
			interfaceName = strings.TrimSuffix(interfaceName, ".service")
			
			configPath := filepath.Join("/etc/wireguard", interfaceName+".conf")
			if err := m.parseWireGuardConfig(interfaceName, configPath); err != nil {
				logger.Warn("Failed to parse config for %s: %v", interfaceName, err)
				continue
			}
		}
	}

	logger.Info("Discovered %d WireGuard interfaces with domain endpoints", len(m.configs))
	return nil
}

func (m *DDNSMonitor) parseWireGuardConfig(interfaceName, configPath string) error {
	file, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("failed to open config file %s: %w", configPath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	endpointRegex := regexp.MustCompile(`^\s*Endpoint\s*=\s*(.+)$`)
	ipRegex := regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		matches := endpointRegex.FindStringSubmatch(line)
		if len(matches) == 2 {
			endpoint := strings.TrimSpace(matches[1])
			
			host, _, err := net.SplitHostPort(endpoint)
			if err != nil {
				continue
			}

			if !ipRegex.MatchString(host) {
				config := Config{
					Interface: interfaceName,
					Endpoint:  endpoint,
					Hostname:  host,
				}
				
				if ip, err := net.ResolveIPAddr("ip4", host); err == nil {
					config.LastIP = ip.IP
				}
				
				m.configs = append(m.configs, config)
				logger.Debug("Found domain endpoint: %s -> %s (interface: %s)", host, config.LastIP, interfaceName)
			}
		}
	}

	return scanner.Err()
}

func (m *DDNSMonitor) checkEndpoints() {
	for i := range m.configs {
		config := &m.configs[i]
		
		logger.Debug("Resolving DNS for %s (interface: %s)", config.Hostname, config.Interface)
		currentIP, err := net.ResolveIPAddr("ip4", config.Hostname)
		if err != nil {
			logger.Warn("Failed to resolve %s: %v", config.Hostname, err)
			continue
		}

		logger.Debug("DNS resolution result for %s: %s (interface: %s)", config.Hostname, currentIP.IP, config.Interface)

		if !config.LastIP.Equal(currentIP.IP) {
			logger.Warn("IP change detected for %s: %s -> %s (interface: %s)", 
				config.Hostname, config.LastIP, currentIP.IP, config.Interface)
			
			config.LastIP = currentIP.IP
			
			if err := m.restartWireGuardService(config.Interface); err != nil {
				logger.Error("Failed to restart wg-quick@%s: %v", config.Interface, err)
			} else {
				logger.Warn("Successfully restarted wg-quick@%s.service", config.Interface)
			}
		}
	}
}

func (m *DDNSMonitor) restartWireGuardService(interfaceName string) error {
	serviceName := fmt.Sprintf("wg-quick@%s.service", interfaceName)
	
	reschan := make(chan string)
	_, err := m.conn.RestartUnitContext(context.Background(), serviceName, "replace", reschan)
	if err != nil {
		return fmt.Errorf("failed to restart service %s: %w", serviceName, err)
	}

	job := <-reschan
	if job != "done" {
		return fmt.Errorf("service restart job failed: %s", job)
	}

	return nil
}

func (m *DDNSMonitor) startHTTPServer(ctx context.Context) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(m.loggingMiddleware())
	
	v1 := router.Group("/api/v1")
	v1.Use(m.authMiddleware())
	{
		v1.POST("/restart", m.handleRestart)
		v1.GET("/interfaces", m.handleListInterfaces)
	}
	
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	
	addr := fmt.Sprintf("%s:%s", m.listenAddress, m.listenPort)
	m.httpServer = &http.Server{
		Addr:    addr,
		Handler: router,
	}
	
	logger.Info("HTTP API server started on %s", addr)
	logger.Info("Swagger UI available at http://%s/swagger/index.html", addr)
	
	go func() {
		if err := m.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error: %v", err)
		}
	}()
	
	<-ctx.Done()
}

func (m *DDNSMonitor) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		
		c.Next()
		
		duration := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		path := c.Request.URL.Path
		statusCode := c.Writer.Status()
		
		logger.Info("API %s %s - %d - %v - %s", method, path, statusCode, duration, clientIP)
	}
}

func (m *DDNSMonitor) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey != m.apiKey {
			logger.Warn("API authentication failed from %s", c.ClientIP())
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// @Summary Restart WireGuard interface
// @Description Restart a specific WireGuard interface
// @Tags interfaces
// @Accept json
// @Produce json
// @Param X-API-Key header string true "API Key"
// @Param request body RestartRequest true "Interface to restart"
// @Success 200 {object} RestartResponse
// @Failure 400 {object} RestartResponse
// @Failure 401 {object} RestartResponse
// @Failure 404 {object} RestartResponse
// @Failure 500 {object} RestartResponse
// @Router /restart [post]
func (m *DDNSMonitor) handleRestart(c *gin.Context) {
	var req RestartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Debug("API restart request - invalid JSON from %s", c.ClientIP())
		c.JSON(http.StatusBadRequest, RestartResponse{
			Success: false,
			Message: "Invalid request format",
		})
		return
	}
	
	logger.Info("API restart request for interface '%s' from %s", req.Interface, c.ClientIP())
	
	if m.singleInterface != "" && req.Interface != m.singleInterface {
		logger.Warn("API restart request denied - interface '%s' not allowed (single-interface mode: %s)", req.Interface, m.singleInterface)
		c.JSON(http.StatusBadRequest, RestartResponse{
			Success: false,
			Message: fmt.Sprintf("Only interface '%s' is monitored", m.singleInterface),
		})
		return
	}
	
	found := false
	for _, config := range m.configs {
		if config.Interface == req.Interface {
			found = true
			break
		}
	}
	
	if !found {
		logger.Warn("API restart request denied - interface '%s' not found in monitored interfaces", req.Interface)
		c.JSON(http.StatusNotFound, RestartResponse{
			Success: false,
			Message: fmt.Sprintf("Interface '%s' not found in monitored interfaces", req.Interface),
		})
		return
	}
	
	if err := m.restartWireGuardService(req.Interface); err != nil {
		logger.Error("API restart request failed for interface '%s': %v", req.Interface, err)
		c.JSON(http.StatusInternalServerError, RestartResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to restart interface: %v", err),
		})
		return
	}
	
	logger.Info("API restart request completed successfully for interface '%s'", req.Interface)
	c.JSON(http.StatusOK, RestartResponse{
		Success: true,
		Message: fmt.Sprintf("Interface '%s' restarted successfully", req.Interface),
	})
}

// @Summary List monitored interfaces
// @Description Get list of all monitored WireGuard interfaces
// @Tags interfaces
// @Produce json
// @Param X-API-Key header string true "API Key"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /interfaces [get]
func (m *DDNSMonitor) handleListInterfaces(c *gin.Context) {
	logger.Debug("API interfaces request from %s", c.ClientIP())
	
	interfaces := make([]map[string]interface{}, 0, len(m.configs))
	for _, config := range m.configs {
		interfaces = append(interfaces, map[string]interface{}{
			"interface": config.Interface,
			"endpoint":  config.Endpoint,
			"hostname":  config.Hostname,
			"last_ip":   config.LastIP.String(),
		})
	}
	
	response := map[string]interface{}{
		"single_interface_mode": m.singleInterface != "",
		"monitored_interface":   m.singleInterface,
		"interfaces":           interfaces,
		"total_count":          len(interfaces),
	}
	
	c.JSON(http.StatusOK, response)
}

func (m *DDNSMonitor) run(ctx context.Context) {
	logger.Info("DNS check interval: %v", m.checkInterval)
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Shutting down monitor")
			return
		case <-ticker.C:
			logger.Debug("Starting scheduled endpoint check")
			m.checkEndpoints()
			logger.Debug("Completed scheduled endpoint check")
		}
	}
}