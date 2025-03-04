package regolith

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type PythonFilter struct {
	Filter

	Script   string `json:"script,omitempty"`
	VenvSlot int    `json:"venvSlot,omitempty"`
}

func PythonFilterFromObject(obj map[string]interface{}) *PythonFilter {
	filter := &PythonFilter{Filter: *FilterFromObject(obj)}

	script, ok := obj["script"].(string)
	if !ok {
		Logger.Fatalf(
			"python filter %q is missing \"script\" field",
			filter.GetFriendlyName())
	}
	filter.Script = script
	filter.VenvSlot, _ = obj["venvSlot"].(int) // default venvSlot is 0
	return filter
}

func (f *PythonFilter) Run(absoluteLocation string) error {
	// Disabled filters are skipped
	if f.Disabled {
		Logger.Infof("Filter '%s' is disabled, skipping.", f.GetFriendlyName())
		return nil
	}
	Logger.Infof("Running filter %s", f.GetFriendlyName())
	start := time.Now()
	defer Logger.Debugf("Executed in %s", time.Since(start))

	// Run filter
	// command is a list of strings that can possibly run python (it's python3
	// on some OSs)
	command := []string{"python", "python3"}
	scriptPath := filepath.Join(absoluteLocation, f.Script)
	if needsVenv(filepath.Dir(scriptPath)) {
		venvPath, err := f.resolveVenvPath()
		if err != nil {
			return wrapError("Failed to resolve venv path", err)
		}
		Logger.Debug("Running Python filter using venv: ", venvPath)
		command = []string{
			filepath.Join(venvPath, venvScriptsPath, "python"+exeSuffix)}
	}
	var args []string
	if len(f.Settings) == 0 {
		args = append([]string{"-u", scriptPath}, f.Arguments...)
	} else {
		jsonSettings, _ := json.Marshal(f.Settings)
		args = append(
			[]string{"-u", scriptPath, string(jsonSettings)},
			f.Arguments...)
	}
	var err error
	for _, c := range command {
		err = RunSubProcess(
			c, args, absoluteLocation, GetAbsoluteWorkingDirectory())
		if err == nil {
			return nil
		}
	}
	if err != nil {
		return wrapError("Failed to run Python script", err)
	}
	return nil
}

func (f *PythonFilter) InstallDependencies(parent *RemoteFilter) error {
	installLocation := ""
	// Install dependencies
	if parent != nil {
		installLocation = parent.GetDownloadPath()
	}
	Logger.Infof("Downloading dependencies for %s...", f.GetFriendlyName())
	scriptPath, err := filepath.Abs(filepath.Join(installLocation, f.Script))
	if err != nil {
		return wrapError(fmt.Sprintf(
			"Unable to resolve path of %s script",
			f.GetFriendlyName()), err)
	}

	// Install the filter dependencies
	filterPath := filepath.Dir(scriptPath)
	if needsVenv(filterPath) {
		venvPath, err := f.resolveVenvPath()
		if err != nil {
			return wrapError("Failed to resolve venv path", err)
		}
		Logger.Info("Creating venv...")
		// it's sometimes python3 on some OSs
		for _, c := range []string{"python", "python3"} {
			err = RunSubProcess(
				c, []string{"-m", "venv", venvPath}, filterPath, "")
			if err == nil {
				break
			}
		}
		Logger.Info("Installing pip dependencies...")
		err = RunSubProcess(
			filepath.Join(venvPath, venvScriptsPath, "pip"+exeSuffix),
			[]string{"install", "-r", "requirements.txt"}, filterPath, filterPath)
		if err != nil {
			return fmt.Errorf(
				"couldn't run pip to install dependencies of %s",
				f.GetFriendlyName(),
			)
		}
	}
	Logger.Infof("Dependencies for %s installed successfully", f.GetFriendlyName())
	return nil
}

func (f *PythonFilter) Check() error {
	python := ""
	var err error
	for _, c := range []string{"python", "python3"} {
		_, err = exec.LookPath(c)
		if err == nil {
			python = c
			break
		}
	}
	if err != nil {
		return errors.New("python not found. Download and install it from https://www.python.org/downloads/")
	}
	cmd, err := exec.Command(python, "--version").Output()
	if err != nil {
		return wrapError("Python version check failed", err)
	}
	a := strings.TrimPrefix(strings.Trim(string(cmd), " \n\t"), "Python ")
	Logger.Debugf("Found Python version %s", a)
	return nil
}

func (f *PythonFilter) CopyArguments(parent *RemoteFilter) {
	f.Arguments = parent.Arguments
	f.Settings = parent.Settings
	f.VenvSlot = parent.VenvSlot
}

func (f *PythonFilter) GetFriendlyName() string {
	if f.Name != "" {
		return f.Name
	}
	return "Unnamed Python filter"
}

func (f *PythonFilter) resolveVenvPath() (string, error) {
	resolvedPath, err := filepath.Abs(
		filepath.Join(".regolith/cache/venvs", strconv.Itoa(f.VenvSlot)))
	if err != nil {
		return "", wrapError(fmt.Sprintf("VenvSlot %v: Unable to create venv", f.VenvSlot), err)
	}
	return resolvedPath, nil
}

func needsVenv(filterPath string) bool {
	Logger.Info(filepath.Join(filterPath, "requirements.txt"))
	stats, err := os.Stat(filepath.Join(filterPath, "requirements.txt"))
	if err == nil {
		return !stats.IsDir()
	}
	return false
}
