package service

import (
	"encoding/json"
	"strings"
)

var pythonModulePackageAliases = map[string]string{
	"crypto":   "pycryptodome",
	"execjs":   "pyexecjs",
	"socks":    "pysocks",
	"cv2":      "opencv-python",
	"bs4":      "beautifulsoup4",
	"pil":      "pillow",
	"yaml":     "pyyaml",
	"serial":   "pyserial",
	"dateutil": "python-dateutil",
	"dotenv":   "python-dotenv",
	"jwt":      "pyjwt",
	"sklearn":  "scikit-learn",
	"openssl":  "pyopenssl",
	"nacl":     "pynacl",
}

func ResolvePythonAutoInstallPackage(moduleName string) string {
	moduleName = strings.TrimSpace(moduleName)
	if moduleName == "" {
		return ""
	}

	if mapped, exists := pythonModulePackageAliases[strings.ToLower(moduleName)]; exists {
		return mapped
	}

	return moduleName
}

func PythonAutoInstallAliases() map[string]string {
	aliases := make(map[string]string, len(pythonModulePackageAliases))
	for key, value := range pythonModulePackageAliases {
		aliases[key] = value
	}
	return aliases
}

func EncodePythonAutoInstallAliases() string {
	data, err := json.Marshal(PythonAutoInstallAliases())
	if err != nil {
		return "{}"
	}
	return string(data)
}
