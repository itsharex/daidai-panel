package service

import (
	"encoding/json"
	"strings"
)

var pythonModulePackageAliases = map[string]string{
	"crypto": "pycryptodome",
	"execjs": "pyexecjs",
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
