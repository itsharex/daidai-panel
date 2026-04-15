package service

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"daidai-panel/config"
	"daidai-panel/database"
	"daidai-panel/model"
)

type managedRuntimePaths struct {
	NodeBin          string
	NodeModules      string
	VenvBin          string
	VenvSitePackages string
	SanitizedPath    string

	searchDirs []string
}

const pythonEnvBootstrap = `import builtins, importlib, json, os, runpy, subprocess, sys
env_file, script_path, extra_path_raw = sys.argv[1:4]
script_args = sys.argv[4:]
with open(env_file, "r", encoding="utf-8") as fh:
    payload = json.load(fh)
for key, value in payload.items():
    if value is None:
        continue
    os.environ[str(key)] = str(value)
for entry in reversed([item for item in extra_path_raw.split(os.pathsep) if item]):
    if entry not in sys.path:
        sys.path.insert(0, entry)
_dd_auto_install_enabled = str(os.environ.get("DD_AUTO_INSTALL_DEPS", "")).strip().lower() in {"1", "true", "yes", "on"}
try:
    _dd_aliases = json.loads(os.environ.get("DD_PY_AUTO_INSTALL_ALIASES", "{}") or "{}")
except Exception:
    _dd_aliases = {}
if not isinstance(_dd_aliases, dict):
    _dd_aliases = {}
if _dd_auto_install_enabled:
    import ast as _dd_ast, tokenize as _dd_tok, io as _dd_io
    _dd_stdlib_names = set(sys.stdlib_module_names) if hasattr(sys, "stdlib_module_names") else set(sys.builtin_module_names)
    _dd_stdlib_names |= set(sys.builtin_module_names)
    _dd_stdlib_names |= {"sendNotify", "notify", "CryptoJS", "ql", "qlApi", "jdCookie"}

    def _dd_scan_imports(path):
        try:
            with open(path, "r", encoding="utf-8") as fh:
                tree = _dd_ast.parse(fh.read(), filename=path)
        except Exception:
            return []
        names = []
        for node in _dd_ast.walk(tree):
            if isinstance(node, _dd_ast.Import):
                for alias in node.names:
                    names.append(alias.name.split(".")[0])
            elif isinstance(node, _dd_ast.ImportFrom):
                if node.module and node.level == 0:
                    names.append(node.module.split(".")[0])
        return names

    def _dd_install_package(request_name, package_name):
        display_name = request_name if not package_name or package_name == request_name else f"{request_name} -> {package_name}"
        print(f"[检测到缺失依赖: {display_name}，正在自动安装...]", flush=True)
        proc = subprocess.run(
            [sys.executable, "-m", "pip", "install", package_name],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
        )
        output = (proc.stdout or "").strip()
        if output:
            print(output, flush=True)
        if proc.returncode != 0:
            print(f"[安装失败: {display_name}]", flush=True)
            return False
        print(f"[安装成功: {display_name}]", flush=True)
        return True

    def _dd_is_local_module(name, script_dir):
        for suffix in [".so", ".pyd", ".py", ".pyc"]:
            if os.path.isfile(os.path.join(script_dir, name + suffix)):
                return True
        if os.path.isdir(os.path.join(script_dir, name)):
            return True
        import glob as _dd_glob
        if _dd_glob.glob(os.path.join(script_dir, name + ".*.so")):
            return True
        if _dd_glob.glob(os.path.join(script_dir, name + ".*.pyd")):
            return True
        return False

    _dd_script_dir = os.path.dirname(os.path.abspath(script_path))
    _dd_imported_names = _dd_scan_imports(script_path)
    _dd_missing = []
    for _dd_name in dict.fromkeys(_dd_imported_names):
        if _dd_name.startswith("_") or _dd_name in _dd_stdlib_names:
            continue
        if _dd_is_local_module(_dd_name, _dd_script_dir):
            continue
        try:
            importlib.import_module(_dd_name)
        except ImportError:
            _dd_missing.append(_dd_name)

    for _dd_name in _dd_missing:
        package_name = str(_dd_aliases.get(_dd_name.lower(), _dd_name)).strip()
        _dd_install_package(_dd_name, package_name)

    del _dd_ast, _dd_tok, _dd_io, _dd_imported_names, _dd_missing

sys.argv = [script_path] + script_args
runpy.run_path(script_path, run_name="__main__")
`

func BuildManagedRuntimeEnvMap(workDir, scriptsDir string, defaultChannelID *uint, ttl time.Duration) (map[string]string, error) {
	var envVarRecords []model.EnvVar
	// 按稳定顺序读取：置顶 > 组内位置 > 创建时间 > id；避免无 ORDER BY 导致同名变量的相对顺序抖动
	database.DB.Where("enabled = ?", true).
		Order("sort_order DESC, position ASC, created_at ASC, id ASC").
		Find(&envVarRecords)

	// 先按 name 分组保持顺序，再用 joinTaskEnvValues 做带转义合并，
	// 解决值内含 '&' 时脚本按 '&' 切分会错位的问题（与 splitTaskEnvValues 对称）。
	grouped := make(map[string][]string)
	order := make([]string, 0, len(envVarRecords))
	for _, ev := range envVarRecords {
		if _, ok := grouped[ev.Name]; !ok {
			order = append(order, ev.Name)
		}
		grouped[ev.Name] = append(grouped[ev.Name], ev.Value)
	}

	envMap := make(map[string]string, len(grouped))
	for _, name := range order {
		envMap[name] = joinTaskEnvValues(grouped[name])
	}

	runtimePaths := currentManagedRuntimePaths()
	if runtimePaths.NodeModules != "" {
		envMap["NODE_PATH"] = runtimePaths.NodeModules
	}
	if runtimePaths.SanitizedPath != "" {
		envMap["PATH"] = joinPathSegments(runtimePaths.VenvBin, runtimePaths.SanitizedPath, runtimePaths.NodeBin)
	}
	if pythonPath := buildManagedPythonPath(envMap["PYTHONPATH"], workDir, scriptsDir, runtimePaths.VenvSitePackages); pythonPath != "" {
		envMap["PYTHONPATH"] = pythonPath
	}
	if model.GetRegisteredConfigBool("auto_install_deps") {
		envMap["DD_AUTO_INSTALL_DEPS"] = "1"
	} else {
		envMap["DD_AUTO_INSTALL_DEPS"] = "0"
	}
	envMap["DD_PY_AUTO_INSTALL_ALIASES"] = EncodePythonAutoInstallAliases()

	AppendScriptHelperPaths(envMap, scriptsDir)
	var helperErr error
	if helperEnv, err := BuildNotifyHelperEnv(scriptsDir, workDir, config.C.Server.Port, defaultChannelID, ttl); err == nil {
		for key, value := range helperEnv {
			envMap[key] = value
		}
	} else {
		helperErr = err
	}

	return envMap, helperErr
}

func buildManagedPythonPath(existingPythonPath, workDir, scriptsDir, venvSitePackages string) string {
	return joinPathSegments(workDir, scriptsDir, existingPythonPath, venvSitePackages)
}

func CreateManagedCommand(interpreter, scriptPath string, scriptArgs []string, workDir string, envVars map[string]string) (*exec.Cmd, func(), error) {
	runtimePaths := currentManagedRuntimePaths()

	switch interpreter {
	case "python", "python3":
		return createManagedPythonCommand(scriptPath, scriptArgs, workDir, envVars, runtimePaths)
	case "node":
		return createManagedNodeCommand(scriptPath, scriptArgs, workDir, envVars, runtimePaths)
	case "ts-node":
		return createManagedTSNodeCommand(scriptPath, scriptArgs, workDir, envVars, runtimePaths)
	default:
		return createStandardManagedCommand(interpreter, scriptPath, scriptArgs, workDir, envVars, runtimePaths)
	}
}

func currentManagedRuntimePaths() managedRuntimePaths {
	dataDir := ""
	if config.C != nil {
		dataDir = config.C.Data.Dir
	}
	depsDir := filepath.Join(dataDir, "deps")
	venvDir := filepath.Join(depsDir, "python", "venv")
	venvBin := resolveManagedVenvBin(venvDir)
	nodeBin := filepath.Join(depsDir, "nodejs", "node_modules", ".bin")
	sanitizedPath := sanitizeManagedPath(os.Getenv("PATH"), nodeBin, venvBin)

	return managedRuntimePaths{
		NodeBin:          nodeBin,
		NodeModules:      filepath.Join(depsDir, "nodejs", "node_modules"),
		VenvBin:          venvBin,
		VenvSitePackages: findVenvSitePackages(venvDir),
		SanitizedPath:    sanitizedPath,
		searchDirs:       splitPathDirs(sanitizedPath),
	}
}

func resolveManagedVenvBin(venvDir string) string {
	venvDir = strings.TrimSpace(venvDir)
	if venvDir == "" {
		return ""
	}

	candidates := []string{
		filepath.Join(venvDir, "Scripts"),
		filepath.Join(venvDir, "bin"),
	}
	if runtime.GOOS != "windows" {
		candidates[0], candidates[1] = candidates[1], candidates[0]
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}

	if runtime.GOOS == "windows" {
		return filepath.Join(venvDir, "Scripts")
	}
	return filepath.Join(venvDir, "bin")
}

func ResolveManagedPipBinary() string {
	runtimePaths := currentManagedRuntimePaths()
	for _, name := range []string{"pip3", "pip"} {
		if binary := findExecutableInDir(runtimePaths.VenvBin, name); binary != "" {
			return binary
		}
	}
	return ""
}

func createManagedPythonCommand(scriptPath string, scriptArgs []string, workDir string, envVars map[string]string, runtimePaths managedRuntimePaths) (*exec.Cmd, func(), error) {
	pythonBin, err := resolveManagedBinary("python3", []string{runtimePaths.VenvBin}, runtimePaths.searchDirs)
	if err != nil {
		pythonBin, err = resolveManagedBinary("python", []string{runtimePaths.VenvBin}, runtimePaths.searchDirs)
		if err != nil {
			return nil, nil, err
		}
	}

	tempDir, envFile, cleanup, err := writeManagedRuntimeEnvFile(envVars)
	if err != nil {
		return nil, nil, err
	}
	_ = tempDir

	args := []string{"-u", "-c", pythonEnvBootstrap, envFile, scriptPath, strings.TrimSpace(envVars["PYTHONPATH"])}
	args = append(args, scriptArgs...)

	cmd := exec.Command(pythonBin, args...)
	cmd.Dir = workDir
	cmd.Env = buildBootstrapProcessEnv(envVars)
	setPgid(cmd)
	return cmd, cleanup, nil
}

func createManagedNodeCommand(scriptPath string, scriptArgs []string, workDir string, envVars map[string]string, runtimePaths managedRuntimePaths) (*exec.Cmd, func(), error) {
	nodeBin, err := resolveManagedBinary("node", nil, runtimePaths.searchDirs)
	if err != nil {
		return nil, nil, err
	}

	_, envFile, cleanup, err := writeManagedRuntimeEnvFile(envVars)
	if err != nil {
		return nil, nil, err
	}
	nodeModulesCleanup := ensureManagedNodeModulesAccess(workDir, runtimePaths.NodeModules)

	preloadFile, preloadErr := writeNodePreloadScript(filepath.Dir(envFile), envFile, envVars)
	if preloadErr != nil {
		cleanup()
		nodeModulesCleanup()
		return nil, nil, preloadErr
	}

	args := []string{"--require", preloadFile, scriptPath}
	args = append(args, scriptArgs...)

	cmd := exec.Command(nodeBin, args...)
	cmd.Dir = workDir
	cmd.Env = buildBootstrapProcessEnv(envVars)
	setPgid(cmd)
	return cmd, combineCleanup(cleanup, nodeModulesCleanup), nil
}

func createManagedTSNodeCommand(scriptPath string, scriptArgs []string, workDir string, envVars map[string]string, runtimePaths managedRuntimePaths) (*exec.Cmd, func(), error) {
	_, envFile, cleanup, err := writeManagedRuntimeEnvFile(envVars)
	if err != nil {
		return nil, nil, err
	}
	nodeModulesCleanup := ensureManagedNodeModulesAccess(workDir, runtimePaths.NodeModules)

	preloadFile, preloadErr := writeNodePreloadScript(filepath.Dir(envFile), envFile, envVars)
	if preloadErr != nil {
		cleanup()
		nodeModulesCleanup()
		return nil, nil, preloadErr
	}

	tsNodeBin, tsErr := resolveManagedBinary("ts-node", []string{runtimePaths.NodeBin}, runtimePaths.searchDirs)
	if tsErr == nil {
		args := []string{"--require", preloadFile, scriptPath}
		args = append(args, scriptArgs...)
		cmd := exec.Command(tsNodeBin, args...)
		cmd.Dir = workDir
		cmd.Env = buildBootstrapProcessEnv(envVars)
		setPgid(cmd)
		return cmd, combineCleanup(cleanup, nodeModulesCleanup), nil
	}

	npxBin, err := resolveManagedBinary("npx", nil, runtimePaths.searchDirs)
	if err != nil {
		cleanup()
		nodeModulesCleanup()
		return nil, nil, err
	}

	args := []string{"ts-node", "--require", preloadFile, scriptPath}
	args = append(args, scriptArgs...)

	cmd := exec.Command(npxBin, args...)
	cmd.Dir = workDir
	cmd.Env = buildBootstrapProcessEnv(envVars)
	setPgid(cmd)
	return cmd, combineCleanup(cleanup, nodeModulesCleanup), nil
}

func createStandardManagedCommand(interpreter, scriptPath string, scriptArgs []string, workDir string, envVars map[string]string, runtimePaths managedRuntimePaths) (*exec.Cmd, func(), error) {
	binary, err := resolveManagedBinary(interpreter, standardBinaryPreferredDirs(interpreter, runtimePaths), runtimePaths.searchDirs)
	if err != nil {
		return nil, nil, err
	}

	var args []string
	switch interpreter {
	case "go":
		args = append([]string{"run", scriptPath}, scriptArgs...)
	case "bash":
		args = append([]string{scriptPath}, scriptArgs...)
	default:
		args = append([]string{scriptPath}, scriptArgs...)
	}

	cmd := exec.Command(binary, args...)
	cmd.Dir = workDir
	cmd.Env = buildEnv(envVars)
	setPgid(cmd)
	return cmd, func() {}, nil
}

func standardBinaryPreferredDirs(interpreter string, runtimePaths managedRuntimePaths) []string {
	switch interpreter {
	case "bash":
		return nil
	case "go":
		return nil
	default:
		return nil
	}
}

func buildBootstrapProcessEnv(envVars map[string]string) []string {
	safeKeys := []string{"PATH", "HOME", "USER", "LANG", "LC_ALL", "TZ"}
	if runtime.GOOS == "windows" {
		safeKeys = append(safeKeys, "SYSTEMROOT", "PATHEXT", "TEMP", "TMP", "APPDATA", "LOCALAPPDATA", "USERPROFILE")
	}

	env := make([]string, 0, len(safeKeys))
	for _, key := range safeKeys {
		value := os.Getenv(key)
		if key == "PATH" && strings.TrimSpace(envVars["PATH"]) != "" {
			value = envVars["PATH"]
		}
		if value == "" {
			continue
		}
		env = append(env, key+"="+value)
	}

	return AppendProxyEnv(env)
}

func writeManagedRuntimeEnvFile(envVars map[string]string) (string, string, func(), error) {
	tempDir, err := os.MkdirTemp("", "daidai-runtime-*")
	if err != nil {
		return "", "", nil, err
	}

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	payload := make(map[string]string, len(envVars))
	for key, value := range envVars {
		if strings.ContainsRune(value, 0) {
			continue
		}
		payload[key] = value
	}

	data, err := json.Marshal(payload)
	if err != nil {
		cleanup()
		return "", "", nil, err
	}

	envFile := filepath.Join(tempDir, "env.json")
	if err := os.WriteFile(envFile, data, 0o600); err != nil {
		cleanup()
		return "", "", nil, err
	}

	return tempDir, envFile, cleanup, nil
}

func writeNodePreloadScript(tempDir, envFile string, envVars map[string]string) (string, error) {
	helperPath := filepath.ToSlash(strings.TrimSpace(envVars["DAIDAI_SEND_NOTIFY_JS"]))
	nodePathList := strings.Split(strings.TrimSpace(envVars["NODE_PATH"]), string(os.PathListSeparator))
	filteredNodePaths := make([]string, 0, len(nodePathList))
	for _, item := range nodePathList {
		item = strings.TrimSpace(item)
		if item != "" {
			filteredNodePaths = append(filteredNodePaths, filepath.ToSlash(item))
		}
	}

	helperJSON, err := json.Marshal(helperPath)
	if err != nil {
		return "", err
	}
	nodePathsJSON, err := json.Marshal(filteredNodePaths)
	if err != nil {
		return "", err
	}

	script := fmt.Sprintf(`const fs = require('fs');
const path = require('path');
const Module = require('module');
const envPayload = JSON.parse(fs.readFileSync(%q, 'utf8'));
for (const [key, value] of Object.entries(envPayload)) {
  if (value === undefined || value === null) {
    continue;
  }
  process.env[key] = String(value);
}
const extraNodePaths = %s;
const mergedNodePaths = [];
for (const value of [...extraNodePaths, ...(process.env.NODE_PATH ? process.env.NODE_PATH.split(path.delimiter) : [])]) {
  if (!value) {
    continue;
  }
  if (!mergedNodePaths.includes(value)) {
    mergedNodePaths.push(value);
  }
}
if (mergedNodePaths.length > 0) {
  process.env.NODE_PATH = mergedNodePaths.join(path.delimiter);
  Module._initPaths();
}
const _origResolve = Module._resolveFilename;
function _resolveExportsEntry(exp) {
  if (typeof exp === 'string') return exp;
  if (exp && typeof exp === 'object') {
    return exp.require || exp.default || exp.node || exp.import || '';
  }
  return '';
}
Module._resolveFilename = function(request, parent, isMain, options) {
  try {
    return _origResolve.call(this, request, parent, isMain, options);
  } catch (err) {
    if (err.code === 'ERR_PACKAGE_PATH_NOT_EXPORTED') {
      const parts = request.split('/');
      const pkgName = parts[0].startsWith('@') ? parts.slice(0, 2).join('/') : parts[0];
      const subPath = parts.slice(pkgName.startsWith('@') ? 2 : 1).join('/');
      for (const np of (process.env.NODE_PATH || '').split(path.delimiter)) {
        if (!np) continue;
        try {
          const pkgDir = path.join(np, pkgName);
          const pkgJson = JSON.parse(fs.readFileSync(path.join(pkgDir, 'package.json'), 'utf8'));
          let target = '';
          if (subPath) {
            const exportKey = './' + subPath;
            if (pkgJson.exports && pkgJson.exports[exportKey]) {
              target = _resolveExportsEntry(pkgJson.exports[exportKey]);
            }
            if (!target) target = subPath;
          } else {
            if (pkgJson.exports && pkgJson.exports['.']) {
              target = _resolveExportsEntry(pkgJson.exports['.']);
            }
            if (!target) target = pkgJson.main || '';
            if (!target) target = 'index.js';
          }
          const candidates = [
            path.join(pkgDir, target),
            path.join(pkgDir, target + '.js'),
            path.join(pkgDir, target, 'index.js')
          ];
          for (const c of candidates) {
            if (fs.existsSync(c)) return c;
          }
        } catch (_) {}
      }
    }
    throw err;
  }
};
const helperPath = %s;
if (helperPath) {
  require(helperPath);
}
`, filepath.ToSlash(envFile), string(nodePathsJSON), string(helperJSON))

	preloadFile := filepath.Join(tempDir, "node-preload.js")
	if err := os.WriteFile(preloadFile, []byte(script), 0o600); err != nil {
		return "", err
	}

	return preloadFile, nil
}

func resolveManagedBinary(name string, preferredDirs []string, fallbackDirs []string) (string, error) {
	if strings.ContainsRune(name, os.PathSeparator) || strings.Contains(name, "/") {
		if isExecutableFile(name) {
			return name, nil
		}
		return "", fmt.Errorf("找不到可执行文件: %s", name)
	}

	searchDirs := make([]string, 0, len(preferredDirs)+len(fallbackDirs))
	seen := make(map[string]struct{}, len(preferredDirs)+len(fallbackDirs))
	for _, dir := range append(preferredDirs, fallbackDirs...) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		if _, exists := seen[dir]; exists {
			continue
		}
		seen[dir] = struct{}{}
		searchDirs = append(searchDirs, dir)
	}

	for _, dir := range searchDirs {
		if binary := findExecutableInDir(dir, name); binary != "" {
			return binary, nil
		}
	}

	return "", fmt.Errorf("找不到可执行文件: %s", name)
}

func findExecutableInDir(dir, name string) string {
	if dir == "" {
		return ""
	}

	candidates := []string{name}
	if runtime.GOOS == "windows" && filepath.Ext(name) == "" {
		pathext := os.Getenv("PATHEXT")
		if pathext == "" {
			pathext = ".COM;.EXE;.BAT;.CMD"
		}
		for _, ext := range strings.Split(pathext, ";") {
			ext = strings.TrimSpace(ext)
			if ext == "" {
				continue
			}
			candidates = append(candidates, name+strings.ToLower(ext))
			candidates = append(candidates, name+strings.ToUpper(ext))
		}
	}

	for _, candidate := range candidates {
		fullPath := filepath.Join(dir, candidate)
		if isExecutableFile(fullPath) {
			return fullPath
		}
	}

	return ""
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

func findVenvSitePackages(venvDir string) string {
	venvDir = strings.TrimSpace(venvDir)
	if venvDir == "" {
		return ""
	}

	windowsSitePackages := filepath.Join(venvDir, "Lib", "site-packages")
	if info, err := os.Stat(windowsSitePackages); err == nil && info.IsDir() {
		return windowsSitePackages
	}

	venvLib := filepath.Join(venvDir, "lib")
	entries, err := os.ReadDir(venvLib)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "python") {
			return filepath.Join(venvLib, entry.Name(), "site-packages")
		}
	}
	return ""
}

func ensureManagedNodeModulesAccess(workDir, nodeModules string) func() {
	workDir = strings.TrimSpace(workDir)
	nodeModules = strings.TrimSpace(nodeModules)
	if workDir == "" || nodeModules == "" {
		return func() {}
	}

	if info, err := os.Stat(nodeModules); err != nil || !info.IsDir() {
		return func() {}
	}

	linkPath := filepath.Join(workDir, "node_modules")
	if _, err := os.Lstat(linkPath); err == nil || !os.IsNotExist(err) {
		return func() {}
	}

	if err := createManagedDirectoryLink(nodeModules, linkPath); err != nil {
		return func() {}
	}

	return func() {
		_ = os.Remove(linkPath)
	}
}

func createManagedDirectoryLink(target, link string) error {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/C", "mklink", "/J", link, target)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("create node_modules junction: %w: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	}

	return os.Symlink(target, link)
}

func combineCleanup(cleanups ...func()) func() {
	return func() {
		for _, cleanup := range cleanups {
			if cleanup != nil {
				cleanup()
			}
		}
	}
}

func sanitizeManagedPath(currentPath, nodeBin, venvBin string) string {
	cleanNodeBin := filepath.Clean(strings.TrimSpace(nodeBin))
	cleanVenvBin := filepath.Clean(strings.TrimSpace(venvBin))

	segments := make([]string, 0)
	seen := make(map[string]struct{})
	for _, item := range splitPathDirs(currentPath) {
		cleanItem := filepath.Clean(strings.TrimSpace(item))
		if cleanItem == "" || cleanItem == "." {
			continue
		}
		if cleanItem == cleanNodeBin || cleanItem == cleanVenvBin {
			continue
		}
		if _, exists := seen[cleanItem]; exists {
			continue
		}
		seen[cleanItem] = struct{}{}
		segments = append(segments, cleanItem)
	}

	return strings.Join(segments, string(os.PathListSeparator))
}

func splitPathDirs(raw string) []string {
	parts := strings.Split(raw, string(os.PathListSeparator))
	result := make([]string, 0, len(parts))
	for _, item := range parts {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func joinPathSegments(parts ...string) string {
	joined := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		for _, item := range splitPathDirs(part) {
			cleanItem := filepath.Clean(strings.TrimSpace(item))
			if cleanItem == "" || cleanItem == "." {
				continue
			}
			if _, exists := seen[cleanItem]; exists {
				continue
			}
			seen[cleanItem] = struct{}{}
			joined = append(joined, cleanItem)
		}
	}
	return strings.Join(joined, string(os.PathListSeparator))
}
