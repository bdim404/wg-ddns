package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
)

type Config struct {
	Interface string
	Endpoint  string
	Hostname  string
	LastIP    net.IP
}

type DDNSMonitor struct {
	configs []Config
	conn    *dbus.Conn
}

func main() {
	monitor := &DDNSMonitor{}
	
	if err := monitor.initialize(); err != nil {
		log.Fatalf("Failed to initialize monitor: %v", err)
	}
	defer monitor.cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal")
		cancel()
	}()

	log.Println("WireGuard DDNS monitor started")
	monitor.run(ctx)
}

func (m *DDNSMonitor) initialize() error {
	var err error
	m.conn, err = dbus.New()
	if err != nil {
		return fmt.Errorf("failed to connect to systemd: %w", err)
	}

	return m.discoverWireGuardConfigs()
}

func (m *DDNSMonitor) cleanup() {
	if m.conn != nil {
		m.conn.Close()
	}
}

func (m *DDNSMonitor) discoverWireGuardConfigs() error {
	units, err := m.conn.ListUnits()
	if err != nil {
		return fmt.Errorf("failed to list systemd units: %w", err)
	}

	for _, unit := range units {
		if strings.HasPrefix(unit.Name, "wg-quick@") && strings.HasSuffix(unit.Name, ".service") && unit.ActiveState == "active" {
			interfaceName := strings.TrimPrefix(unit.Name, "wg-quick@")
			interfaceName = strings.TrimSuffix(interfaceName, ".service")
			
			configPath := filepath.Join("/etc/wireguard", interfaceName+".conf")
			if err := m.parseWireGuardConfig(interfaceName, configPath); err != nil {
				log.Printf("Failed to parse config for %s: %v", interfaceName, err)
				continue
			}
		}
	}

	log.Printf("Discovered %d WireGuard interfaces with domain endpoints", len(m.configs))
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
				log.Printf("Found domain endpoint: %s -> %s (interface: %s)", host, config.LastIP, interfaceName)
			}
		}
	}

	return scanner.Err()
}

func (m *DDNSMonitor) checkEndpoints() {
	for i := range m.configs {
		config := &m.configs[i]
		
		currentIP, err := net.ResolveIPAddr("ip4", config.Hostname)
		if err != nil {
			log.Printf("Failed to resolve %s: %v", config.Hostname, err)
			continue
		}

		if !config.LastIP.Equal(currentIP.IP) {
			log.Printf("IP change detected for %s: %s -> %s (interface: %s)", 
				config.Hostname, config.LastIP, currentIP.IP, config.Interface)
			
			config.LastIP = currentIP.IP
			
			if err := m.restartWireGuardService(config.Interface); err != nil {
				log.Printf("Failed to restart wg-quick@%s: %v", config.Interface, err)
			} else {
				log.Printf("Successfully restarted wg-quick@%s.service", config.Interface)
			}
		}
	}
}

func (m *DDNSMonitor) restartWireGuardService(interfaceName string) error {
	serviceName := fmt.Sprintf("wg-quick@%s.service", interfaceName)
	
	reschan := make(chan string)
	_, err := m.conn.RestartUnit(serviceName, "replace", reschan)
	if err != nil {
		return fmt.Errorf("failed to restart service %s: %w", serviceName, err)
	}

	job := <-reschan
	if job != "done" {
		return fmt.Errorf("service restart job failed: %s", job)
	}

	return nil
}

func (m *DDNSMonitor) run(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down monitor")
			return
		case <-ticker.C:
			m.checkEndpoints()
		}
	}
}