package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"daidai-panel/database"
	"daidai-panel/model"
)

const (
	machineCodeConfigKey     = "machine_code"
	machineCodeDescription   = "面板机器码（首次启动自动生成，重启/升级不会改变）"
	machineCodePrintableBits = 128
)

var (
	cachedMachineCode string
	machineCodeMu     sync.Mutex
)

// EnsureMachineCode returns the panel's stable installation identifier.
// The first invocation generates a random value and persists it into the
// system_configs table; subsequent calls (including across process restarts)
// read the stored value, so the code never changes unless the database row
// is explicitly cleared.
func EnsureMachineCode() string {
	machineCodeMu.Lock()
	defer machineCodeMu.Unlock()

	if cachedMachineCode != "" {
		return cachedMachineCode
	}

	if database.DB == nil {
		return ""
	}

	var existing model.SystemConfig
	err := database.DB.Where("`key` = ?", machineCodeConfigKey).First(&existing).Error
	if err == nil {
		if v := strings.TrimSpace(existing.Value); v != "" {
			cachedMachineCode = v
			return cachedMachineCode
		}
	}

	code := generateMachineCode()
	record := model.SystemConfig{
		Key:         machineCodeConfigKey,
		Value:       code,
		Description: machineCodeDescription,
	}

	if err == nil {
		// Row exists with empty value — fill it in without replacing anything.
		if updateErr := database.DB.Model(&existing).Updates(map[string]interface{}{
			"value":       code,
			"description": machineCodeDescription,
		}).Error; updateErr != nil {
			log.Printf("machine code: update empty row failed: %v", updateErr)
		}
	} else if createErr := database.DB.Create(&record).Error; createErr != nil {
		log.Printf("machine code: create row failed: %v", createErr)
		// Fall back to in-memory only so callers still see a stable value
		// for this process lifetime.
	}

	cachedMachineCode = code
	return cachedMachineCode
}

// ResetMachineCodeCacheForTest clears the in-memory cache so tests can
// simulate a fresh process reading an existing database row.
func ResetMachineCodeCacheForTest() {
	machineCodeMu.Lock()
	defer machineCodeMu.Unlock()
	cachedMachineCode = ""
}

func generateMachineCode() string {
	buf := make([]byte, machineCodePrintableBits/8)
	if _, err := rand.Read(buf); err == nil {
		return formatMachineCode(buf)
	}

	seed := strings.Join([]string{
		runtime.GOOS,
		runtime.GOARCH,
		hostnameOrUnknown(),
		strconv.Itoa(os.Getpid()),
		time.Now().UTC().Format(time.RFC3339Nano),
	}, "|")
	sum := sha256.Sum256([]byte(seed))
	return formatMachineCode(sum[:machineCodePrintableBits/8])
}

func formatMachineCode(raw []byte) string {
	hexStr := strings.ToUpper(hex.EncodeToString(raw))
	var sb strings.Builder
	for i := 0; i < len(hexStr); i += 4 {
		if i > 0 {
			sb.WriteByte('-')
		}
		end := i + 4
		if end > len(hexStr) {
			end = len(hexStr)
		}
		sb.WriteString(hexStr[i:end])
	}
	return sb.String()
}

func hostnameOrUnknown() string {
	if h, err := os.Hostname(); err == nil && strings.TrimSpace(h) != "" {
		return h
	}
	return "unknown"
}
