package model

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	panelcron "daidai-panel/pkg/cron"
	"daidai-panel/pkg/netutil"
)

type SystemConfigValueType string

const (
	SystemConfigTypeString SystemConfigValueType = "string"
	SystemConfigTypeInt    SystemConfigValueType = "int"
	SystemConfigTypeBool   SystemConfigValueType = "bool"
	SystemConfigTypeEnum   SystemConfigValueType = "enum"
)

type SystemConfigOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type SystemConfigDefinition struct {
	Key          string                `json:"key"`
	DefaultValue string                `json:"default_value"`
	Description  string                `json:"description"`
	ValueType    SystemConfigValueType `json:"value_type"`
	Group        string                `json:"group"`
	Options      []SystemConfigOption  `json:"options,omitempty"`
}

type systemConfigSpec struct {
	def       SystemConfigDefinition
	normalize func(string) (string, error)
}

var registeredSystemConfigSpecs = []systemConfigSpec{
	newIntConfig("max_concurrent_tasks", "5", "定时任务最大并发数", "tasks", 1, 128),
	newIntConfig("command_timeout", "86400", "全局默认超时（秒）", "tasks", 1, 604800),
	newIntConfig("log_retention_days", "7", "日志保留天数", "tasks", 1, 3650),
	newIntConfig("max_log_content_size", "102400000", "任务日志内容最大保留字节数", "tasks", 1024, 524288000),
	newBoolConfig("auto_update_enabled", "false", "静默更新开关（每 24 小时自动检查并在有新版本时尝试更新）", "network"),
	newIntConfig("random_delay", "0", "任务执行前随机延迟最大秒数", "tasks", 0, 86400),
	newTrimmedStringConfig("random_delay_extensions", "", "随机延迟仅对指定脚本后缀生效", "tasks"),
	newBoolConfig("auto_install_deps", "true", "脚本缺依赖时自动尝试安装", "tasks"),
	newIntConfig("cpu_warn", "80", "CPU 告警阈值（%）", "alerts", 1, 100),
	newIntConfig("memory_warn", "80", "内存告警阈值（%）", "alerts", 1, 100),
	newIntConfig("disk_warn", "90", "磁盘告警阈值（%）", "alerts", 1, 100),
	newBoolConfig("auto_add_cron", "true", "自动添加定时任务", "subscription"),
	newBoolConfig("auto_del_cron", "true", "自动删除失效任务", "subscription"),
	newValidatedStringConfig("default_cron_rule", "", "订阅脚本未声明 cron 时使用的默认规则", "subscription", normalizeDefaultCronRule),
	newTrimmedStringConfig("repo_file_extensions", "py js sh ts", "订阅自动识别任务时扫描的脚本后缀", "subscription"),
	newBoolConfig("notify_on_resource_warn", "false", "资源超限发送通知", "alerts"),
	newBoolConfig("notify_on_login", "false", "登录成功发送通知", "security"),
	newValidatedStringConfig("proxy_url", "", "出站请求代理地址", "network", normalizeProxyURL),
	newValidatedStringConfig("update_image_mirror", "", "系统更新拉取镜像时使用的可选镜像源（留空直连 Docker Hub）", "network", normalizeUpdateImageMirror),
	newValidatedStringConfig(
		"trusted_proxy_cidrs",
		strings.Join(netutil.DefaultTrustedProxyCIDRs(), "\n"),
		"可信代理 CIDR/IP 列表（逗号、空格或换行分隔）",
		"network",
		normalizeTrustedProxyCIDRs,
	),
	newTrimmedStringConfig("panel_title", "呆呆面板", "面板标题", "branding"),
	newTrimmedStringConfig("panel_icon", "", "面板图标（SVG data URL）", "branding"),
	newTrimmedStringConfig("editor_background_color", "", "脚本编辑器背景颜色（留空使用默认样式）", "branding"),
	newTrimmedStringConfig("log_background_color", "#0f172a", "日志视图背景颜色（建议使用深色）", "branding"),
	newTrimmedStringConfig("log_background_image", "", "日志视图背景图片（data URL）", "branding"),
	newBoolConfig("captcha_enabled", "false", "极验验证码开关（连续失败 3 次后触发）", "security"),
	newTrimmedStringConfig("captcha_id", "", "验证码平台 ID", "security"),
	newTrimmedStringConfig("captcha_key", "", "验证码平台密钥（服务端 Key）", "security"),
	newEnumConfig(
		"captcha_fail_mode",
		"open",
		"验证码上游异常策略：open=放行，strict=严格拦截",
		"security",
		[]SystemConfigOption{
			{Value: "open", Label: "宽松放行"},
			{Value: "strict", Label: "严格拦截"},
		},
	),
}

var registeredSystemConfigMap = buildSystemConfigSpecMap(registeredSystemConfigSpecs)

func buildSystemConfigSpecMap(specs []systemConfigSpec) map[string]systemConfigSpec {
	result := make(map[string]systemConfigSpec, len(specs))
	for _, spec := range specs {
		result[spec.def.Key] = spec
	}
	return result
}

func newTrimmedStringConfig(key, defaultValue, description, group string) systemConfigSpec {
	return systemConfigSpec{
		def: SystemConfigDefinition{
			Key:          key,
			DefaultValue: defaultValue,
			Description:  description,
			ValueType:    SystemConfigTypeString,
			Group:        group,
		},
		normalize: func(value string) (string, error) {
			value = strings.TrimSpace(value)
			if value == "" {
				return strings.TrimSpace(defaultValue), nil
			}
			return value, nil
		},
	}
}

func newValidatedStringConfig(key, defaultValue, description, group string, normalize func(string) (string, error)) systemConfigSpec {
	return systemConfigSpec{
		def: SystemConfigDefinition{
			Key:          key,
			DefaultValue: defaultValue,
			Description:  description,
			ValueType:    SystemConfigTypeString,
			Group:        group,
		},
		normalize: normalize,
	}
}

func newBoolConfig(key, defaultValue, description, group string) systemConfigSpec {
	return systemConfigSpec{
		def: SystemConfigDefinition{
			Key:          key,
			DefaultValue: normalizeBoolDefault(defaultValue),
			Description:  description,
			ValueType:    SystemConfigTypeBool,
			Group:        group,
		},
		normalize: func(value string) (string, error) {
			if strings.TrimSpace(value) == "" {
				return normalizeBoolDefault(defaultValue), nil
			}

			parsed, ok := parseBoolString(value)
			if !ok {
				return "", fmt.Errorf("配置 %s 需要布尔值", key)
			}
			return strconv.FormatBool(parsed), nil
		},
	}
}

func newIntConfig(key, defaultValue, description, group string, minValue, maxValue int) systemConfigSpec {
	return systemConfigSpec{
		def: SystemConfigDefinition{
			Key:          key,
			DefaultValue: defaultValue,
			Description:  description,
			ValueType:    SystemConfigTypeInt,
			Group:        group,
		},
		normalize: func(value string) (string, error) {
			value = strings.TrimSpace(value)
			if value == "" {
				return defaultValue, nil
			}

			parsed, err := strconv.Atoi(value)
			if err != nil {
				return "", fmt.Errorf("配置 %s 需要整数值", key)
			}
			if parsed < minValue || parsed > maxValue {
				return "", fmt.Errorf("配置 %s 需在 %d-%d 之间", key, minValue, maxValue)
			}
			return strconv.Itoa(parsed), nil
		},
	}
}

func newEnumConfig(key, defaultValue, description, group string, options []SystemConfigOption) systemConfigSpec {
	allowed := make(map[string]bool, len(options))
	normalizedOptions := make([]SystemConfigOption, len(options))
	for i, option := range options {
		value := strings.ToLower(strings.TrimSpace(option.Value))
		normalizedOptions[i] = SystemConfigOption{
			Value: value,
			Label: option.Label,
		}
		allowed[value] = true
	}

	defaultValue = strings.ToLower(strings.TrimSpace(defaultValue))

	return systemConfigSpec{
		def: SystemConfigDefinition{
			Key:          key,
			DefaultValue: defaultValue,
			Description:  description,
			ValueType:    SystemConfigTypeEnum,
			Group:        group,
			Options:      normalizedOptions,
		},
		normalize: func(value string) (string, error) {
			value = strings.ToLower(strings.TrimSpace(value))
			if value == "" {
				return defaultValue, nil
			}
			if !allowed[value] {
				return "", fmt.Errorf("配置 %s 的值无效", key)
			}
			return value, nil
		},
	}
}

func normalizeDefaultCronRule(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !panelcron.Parse(value).Valid {
		return "", fmt.Errorf("默认 Cron 规则无效")
	}
	return value, nil
}

func normalizeProxyURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("代理地址格式无效")
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "socks5", "socks5h":
		return value, nil
	default:
		return "", fmt.Errorf("代理地址仅支持 http/https/socks5/socks5h")
	}
}

func normalizeUpdateImageMirror(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	if !strings.Contains(value, "://") {
		value = "https://" + value
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("系统更新镜像源格式无效")
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return "", fmt.Errorf("系统更新镜像源仅支持 http/https")
	}

	if path := strings.Trim(parsed.Path, "/"); path != "" {
		return "", fmt.Errorf("系统更新镜像源暂不支持附带路径，请只填写主机名")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("系统更新镜像源不能带查询参数或片段")
	}

	return parsed.Host, nil
}

func normalizeTrustedProxyCIDRs(value string) (string, error) {
	return netutil.NormalizeTrustedProxyCIDRs(value)
}

func normalizeBoolDefault(value string) string {
	parsed, ok := parseBoolString(value)
	if !ok {
		return "false"
	}
	return strconv.FormatBool(parsed)
}

func parseBoolString(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}

func SystemConfigDefinitions() []SystemConfigDefinition {
	result := make([]SystemConfigDefinition, 0, len(registeredSystemConfigSpecs))
	for _, spec := range registeredSystemConfigSpecs {
		result = append(result, spec.def)
	}
	return result
}

func GetSystemConfigDefinition(key string) (SystemConfigDefinition, bool) {
	spec, exists := registeredSystemConfigMap[key]
	if !exists {
		return SystemConfigDefinition{}, false
	}
	return spec.def, true
}

func NormalizeSystemConfigValue(key, value string) (string, error) {
	spec, exists := registeredSystemConfigMap[key]
	if !exists {
		return value, nil
	}
	return spec.normalize(value)
}

func GetRegisteredConfig(key string) string {
	def, exists := GetSystemConfigDefinition(key)
	if !exists {
		return GetConfig(key, "")
	}
	return GetConfig(key, def.DefaultValue)
}

func GetRegisteredConfigInt(key string) int {
	def, exists := GetSystemConfigDefinition(key)
	if !exists {
		return GetConfigInt(key, 0)
	}

	defaultValue, err := strconv.Atoi(def.DefaultValue)
	if err != nil {
		defaultValue = 0
	}
	return GetConfigInt(key, defaultValue)
}

func GetRegisteredConfigBool(key string) bool {
	def, exists := GetSystemConfigDefinition(key)
	if !exists {
		return GetConfigBool(key, false)
	}

	defaultValue, _ := parseBoolString(def.DefaultValue)
	return GetConfigBool(key, defaultValue)
}

func SortedSystemConfigKeys() []string {
	keys := make([]string, 0, len(registeredSystemConfigSpecs))
	for _, spec := range registeredSystemConfigSpecs {
		keys = append(keys, spec.def.Key)
	}
	return keys
}
