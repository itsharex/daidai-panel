package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"daidai-panel/config"
	"daidai-panel/model"
)

var (
	autoInstallNodeModuleRe    = regexp.MustCompile(`(?:Cannot find module|Error \[ERR_MODULE_NOT_FOUND\].*)\s*'([^']+)'`)
	autoInstallNodeHintRe      = regexp.MustCompile(`npm\s+install\s+([a-zA-Z@][a-zA-Z0-9_./@-]*)`)
	autoInstallPyModuleRe      = regexp.MustCompile(`(?:ModuleNotFoundError|ImportError):\s*No module named\s+'([^']+)'`)
	autoInstallPyHintRe        = regexp.MustCompile(`pip3?\s+install\s+([a-zA-Z][a-zA-Z0-9_.@-]*)`)
	autoInstallGoModuleRe      = regexp.MustCompile(`(?:no required module provides package|missing go\.sum entry for module providing package)\s+([^\s:;]+)`)

	thirdPartyExcludedModules = map[string]bool{
		"sendNotify":           true,
		"notify":               true,
		"CryptoJS":             true,
		"ql":                   true,
		"qlApi":                true,
		"jdCookie":             true,
		"JD_DmFruitShareCodes": true,
	}
)

type AutoInstallCandidate struct {
	Manager       string
	RequestedName string
	PackageName   string
	DisplayName   string
	WorkDir       string
	RecordType    string
	RecordName    string
}

type AutoInstallResult struct {
	Success bool
	Log     string
	Error   string
}

func DetectAutoInstallCandidate(ext, output, workDir string) *AutoInstallCandidate {
	ext = strings.ToLower(strings.TrimSpace(ext))

	switch ext {
	case ".py":
		if matches := autoInstallPyModuleRe.FindStringSubmatch(output); len(matches) > 1 {
			requested := strings.Split(matches[1], ".")[0]
			if isPythonStdlib(requested) || thirdPartyExcludedModules[requested] {
				return nil
			}
			if isLocalPythonModule(requested, workDir) {
				return nil
			}
			packageName := ResolvePythonAutoInstallPackage(requested)
			return &AutoInstallCandidate{
				Manager:       "python",
				RequestedName: requested,
				PackageName:   packageName,
				DisplayName:   formatAutoInstallDisplayName(requested, packageName),
				WorkDir:       workDir,
				RecordType:    model.DepTypePython,
				RecordName:    packageName,
			}
		}
		if matches := autoInstallPyHintRe.FindStringSubmatch(output); len(matches) > 1 {
			requested := strings.TrimSpace(matches[1])
			if requested == "" || isPythonStdlib(requested) || thirdPartyExcludedModules[requested] {
				return nil
			}
			return &AutoInstallCandidate{
				Manager:       "python",
				RequestedName: requested,
				PackageName:   requested,
				DisplayName:   requested,
				WorkDir:       workDir,
				RecordType:    model.DepTypePython,
				RecordName:    requested,
			}
		}
	case ".js", ".mjs", ".ts":
		if matches := autoInstallNodeModuleRe.FindStringSubmatch(output); len(matches) > 1 {
			requested := strings.TrimSpace(matches[1])
			if requested == "" || strings.HasPrefix(requested, ".") || strings.HasPrefix(requested, "/") || thirdPartyExcludedModules[requested] {
				return nil
			}
			return &AutoInstallCandidate{
				Manager:       "nodejs",
				RequestedName: requested,
				PackageName:   requested,
				DisplayName:   requested,
				WorkDir:       workDir,
				RecordType:    model.DepTypeNodeJS,
				RecordName:    requested,
			}
		}
		if matches := autoInstallNodeHintRe.FindStringSubmatch(output); len(matches) > 1 {
			requested := strings.TrimSpace(matches[1])
			if requested == "" || thirdPartyExcludedModules[requested] {
				return nil
			}
			return &AutoInstallCandidate{
				Manager:       "nodejs",
				RequestedName: requested,
				PackageName:   requested,
				DisplayName:   requested,
				WorkDir:       workDir,
				RecordType:    model.DepTypeNodeJS,
				RecordName:    requested,
			}
		}
	case ".go":
		moduleRoot := findNearestAncestorWithFile(workDir, "go.mod")
		if moduleRoot == "" {
			return nil
		}
		if matches := autoInstallGoModuleRe.FindStringSubmatch(output); len(matches) > 1 {
			moduleName := strings.TrimSpace(matches[1])
			if moduleName == "" {
				return nil
			}
			return &AutoInstallCandidate{
				Manager:       "go",
				RequestedName: moduleName,
				PackageName:   moduleName,
				DisplayName:   moduleName,
				WorkDir:       moduleRoot,
			}
		}
	}

	return nil
}

func InstallAutoDependency(candidate *AutoInstallCandidate, envVars map[string]string) AutoInstallResult {
	if candidate == nil {
		return AutoInstallResult{Error: "未找到可自动安装的依赖"}
	}

	baseEnv := buildEnvSlice(envVars)
	depsDir := filepath.Join(config.C.Data.Dir, "deps")

	switch candidate.Manager {
	case "python":
		venvPip := ResolveManagedPipBinary()
		if strings.TrimSpace(venvPip) == "" {
			venvPip = "pip3"
		}
		cmd := exec.Command(venvPip, "install", candidate.PackageName)
		cmd.Env = PipInstallEnv(baseEnv, CurrentPipMirror())
		out, err := cmd.CombinedOutput()
		return completeAutoInstall(candidate, out, err)
	case "nodejs":
		nodeDir := filepath.Join(depsDir, "nodejs")
		_ = os.MkdirAll(nodeDir, 0755)
		cmd := exec.Command("npm", "install", candidate.PackageName, "--prefix", nodeDir)
		cmd.Env = NpmInstallEnv(baseEnv, CurrentNpmMirror())
		out, err := cmd.CombinedOutput()
		return completeAutoInstall(candidate, out, err)
	case "go":
		cmd := exec.Command("go", "get", candidate.PackageName)
		cmd.Dir = candidate.WorkDir
		cmd.Env = baseEnv
		out, err := cmd.CombinedOutput()
		return completeAutoInstall(candidate, out, err)
	default:
		return AutoInstallResult{Error: fmt.Sprintf("不支持的自动安装类型: %s", candidate.Manager)}
	}
}

func completeAutoInstall(candidate *AutoInstallCandidate, out []byte, err error) AutoInstallResult {
	logText := string(out)
	if err != nil {
		return AutoInstallResult{
			Success: false,
			Log:     logText,
			Error:   strings.TrimSpace(logText),
		}
	}

	if candidate.RecordType != "" && candidate.RecordName != "" {
		RecordAutoInstalledDep(candidate.RecordType, candidate.RecordName, logText)
	}

	return AutoInstallResult{
		Success: true,
		Log:     logText,
	}
}

func formatAutoInstallDisplayName(requested, packageName string) string {
	requested = strings.TrimSpace(requested)
	packageName = strings.TrimSpace(packageName)
	if requested == "" {
		return packageName
	}
	if packageName == "" || strings.EqualFold(requested, packageName) {
		return requested
	}
	return requested + " -> " + packageName
}

var pythonStdlibModules = map[string]bool{
	"__future__": true, "_thread": true, "_winapi": true, "abc": true, "aifc": true,
	"argparse": true, "array": true, "ast": true, "asynchat": true, "asyncio": true,
	"asyncore": true, "atexit": true, "audioop": true, "base64": true, "bdb": true,
	"binascii": true, "binhex": true, "bisect": true, "builtins": true, "bz2": true,
	"calendar": true, "cgi": true, "cgitb": true, "chunk": true, "cmath": true,
	"cmd": true, "code": true, "codecs": true, "codeop": true, "collections": true,
	"colorsys": true, "compileall": true, "concurrent": true, "configparser": true,
	"contextlib": true, "contextvars": true, "copy": true, "copyreg": true, "cProfile": true,
	"crypt": true, "csv": true, "ctypes": true, "curses": true, "dataclasses": true,
	"datetime": true, "dbm": true, "decimal": true, "difflib": true, "dis": true,
	"distutils": true, "doctest": true, "email": true, "encodings": true,
	"enum": true, "errno": true, "faulthandler": true, "fcntl": true, "filecmp": true,
	"fileinput": true, "fnmatch": true, "fractions": true, "ftplib": true,
	"functools": true, "gc": true, "getopt": true, "getpass": true, "gettext": true,
	"glob": true, "graphlib": true, "grp": true, "gzip": true, "hashlib": true,
	"heapq": true, "hmac": true, "html": true, "http": true, "idlelib": true,
	"imaplib": true, "imghdr": true, "imp": true, "importlib": true, "inspect": true,
	"io": true, "ipaddress": true, "itertools": true, "json": true, "keyword": true,
	"lib2to3": true, "linecache": true, "locale": true, "logging": true, "lzma": true,
	"mailbox": true, "mailcap": true, "marshal": true, "math": true, "mimetypes": true,
	"mmap": true, "modulefinder": true, "msvcrt": true, "multiprocessing": true,
	"netrc": true, "nis": true, "nntplib": true, "nt": true, "numbers": true,
	"operator": true, "optparse": true, "os": true, "ossaudiodev": true,
	"pathlib": true, "pdb": true, "pickle": true, "pickletools": true, "pipes": true,
	"pkgutil": true, "platform": true, "plistlib": true, "poplib": true, "posix": true,
	"posixpath": true, "pprint": true, "profile": true, "pstats": true, "pty": true,
	"pwd": true, "py_compile": true, "pyclbr": true, "pydoc": true, "queue": true,
	"quopri": true, "random": true, "re": true, "readline": true, "reprlib": true,
	"resource": true, "rlcompleter": true, "runpy": true, "sched": true, "secrets": true,
	"select": true, "selectors": true, "shelve": true, "shlex": true, "shutil": true,
	"signal": true, "site": true, "smtpd": true, "smtplib": true, "sndhdr": true,
	"socket": true, "socketserver": true, "spwd": true, "sqlite3": true, "sre_compile": true,
	"sre_constants": true, "sre_parse": true, "ssl": true, "stat": true, "statistics": true,
	"string": true, "stringprep": true, "struct": true, "subprocess": true, "sunau": true,
	"symtable": true, "sys": true, "sysconfig": true, "syslog": true, "tabnanny": true,
	"tarfile": true, "telnetlib": true, "tempfile": true, "termios": true, "test": true,
	"textwrap": true, "threading": true, "time": true, "timeit": true, "tkinter": true,
	"token": true, "tokenize": true, "tomllib": true, "trace": true, "traceback": true,
	"tracemalloc": true, "tty": true, "turtle": true, "turtledemo": true, "types": true,
	"typing": true, "unicodedata": true, "unittest": true, "urllib": true, "uu": true,
	"uuid": true, "venv": true, "warnings": true, "wave": true, "weakref": true,
	"webbrowser": true, "winreg": true, "winsound": true, "wsgiref": true,
	"xdrlib": true, "xml": true, "xmlrpc": true, "zipapp": true, "zipfile": true,
	"zipimport": true, "zlib": true, "zoneinfo": true,
	"backports": true, "pkg_resources": true, "setuptools": true, "pip": true,
	"_io": true, "_signal": true, "_abc": true, "_codecs": true, "_collections": true,
	"_functools": true, "_operator": true, "_sre": true, "_stat": true, "_string": true,
	"_weakref": true,
}

func isLocalPythonModule(name, workDir string) bool {
	if workDir == "" || name == "" {
		return false
	}
	for _, suffix := range []string{".py", ".so", ".pyd", ".pyc"} {
		if _, err := os.Stat(filepath.Join(workDir, name+suffix)); err == nil {
			return true
		}
	}
	if info, err := os.Stat(filepath.Join(workDir, name)); err == nil && info.IsDir() {
		return true
	}
	matches, _ := filepath.Glob(filepath.Join(workDir, name+".*.so"))
	if len(matches) > 0 {
		return true
	}
	matches, _ = filepath.Glob(filepath.Join(workDir, name+".*.pyd"))
	if len(matches) > 0 {
		return true
	}
	return false
}

func isPythonStdlib(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, "_") {
		return true
	}
	return pythonStdlibModules[name]
}

func findNearestAncestorWithFile(startDir, targetFile string) string {
	current := strings.TrimSpace(startDir)
	if current == "" {
		return ""
	}

	for {
		candidate := filepath.Join(current, targetFile)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return current
		}

		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}
