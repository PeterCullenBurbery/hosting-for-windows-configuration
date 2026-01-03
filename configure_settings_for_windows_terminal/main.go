// To open Windows Terminal settings manually, run:
// code "$env:LOCALAPPDATA\Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json"

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GUID constants
const (
	ps7_guid   = "{574e775e-4f2a-5b96-ac1e-a2962a402336}"
	winps_guid = "{61c54bbd-c2c6-5271-96e7-009a87ff44bf}"
	cmd_guid   = "{0caa0dad-35be-5f56-a8ff-afceeeaa6101}"
	azure_guid = "{b453ae62-4e3d-5e58-b989-0a998ec441b8}"
)

func main() {
	path, err := get_settings_path()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	// Read original
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read settings.json: %v\n", err)
		os.Exit(1)
	}
	// Backup
	if err := make_backup(path, data); err != nil {
		fmt.Fprintf(os.Stderr, "warning: backup failed: %v\n", err)
		// continue anyway
	}
	// Unmarshal
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse JSON: %v\n", err)
		os.Exit(1)
	}
	// Transform
	if err := apply_transform(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "transformation error: %v\n", err)
		os.Exit(1)
	}
	// Marshal with indentation
	out, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal JSON: %v\n", err)
		os.Exit(1)
	}
	// Write back
	tmp_path := path + ".tmp"
	if err := os.WriteFile(tmp_path, out, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write temp file: %v\n", err)
		os.Exit(1)
	}
	// Replace original
	if err := os.Rename(tmp_path, path); err != nil {
		fmt.Fprintf(os.Stderr, "failed to overwrite settings.json: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("settings.json updated successfully (backup created).")
}

// get_settings_path builds the path to settings.json using LOCALAPPDATA.
func get_settings_path() (string, error) {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return "", errors.New("LOCALAPPDATA not set")
	}
	// Windows Terminal package path
	rel := filepath.Join("Packages", "Microsoft.WindowsTerminal_8wekyb3d8bbwe", "LocalState", "settings.json")
	full := filepath.Join(local, rel)
	return full, nil
}

// backup writes the original data to settings.json.bak.TIMESTAMP
func make_backup(original_path string, data []byte) error {
	dir := filepath.Dir(original_path)
	base := filepath.Base(original_path)
	timestamp := time.Now().Format("20060102_150405")
	backup_name := fmt.Sprintf("%s.bak.%s", base, timestamp)
	backup_path := filepath.Join(dir, backup_name)
	return os.WriteFile(backup_path, data, 0o644)
}

// transform applies the JSON modifications in-place on cfg.
func apply_transform(cfg map[string]interface{}) error {
	// 1. Set defaultProfile
	cfg["defaultProfile"] = ps7_guid

	// 2. Locate profiles object
	profiles_raw, ok := cfg["profiles"]
	if !ok {
		return errors.New(`missing "profiles" object`)
	}
	profiles, ok := profiles_raw.(map[string]interface{})
	if !ok {
		return errors.New(`"profiles" is not an object`)
	}

	// 3. Set profiles.defaults
	profiles["defaults"] = map[string]interface{}{
		"elevate":     true,
		"historySize": 1000000000,
	}

	// 4. Rebuild profiles.list
	list_raw, ok := profiles["list"]
	if !ok {
		return errors.New(`missing "profiles.list"`)
	}
	list_slice, ok := list_raw.([]interface{})
	if !ok {
		return errors.New(`"profiles.list" is not an array`)
	}

	// Index existing entries by GUID
	existing_entries := make(map[string]map[string]interface{})
	for _, item := range list_slice {
		if m, ok := item.(map[string]interface{}); ok {
			if g, ok := m["guid"].(string); ok {
				existing_entries[g] = m
			}
		}
	}

	// Build new list in order: PS7, Windows PowerShell, Command Prompt, Azure Cloud Shell
	var new_list []interface{}

	// helper to prepare each entry
	prepare_entry := func(guid string) map[string]interface{} {
		if entry, exists := existing_entries[guid]; exists {
			return entry
		}
		return make(map[string]interface{})
	}

	// 4a. PowerShell 7
	ps7_entry := prepare_entry(ps7_guid)
	ps7_entry["guid"] = ps7_guid
	ps7_entry["name"] = "PowerShell 7"
	ps7_entry["hidden"] = false
	ps7_entry["source"] = "Windows.Terminal.PowershellCore"
	delete(ps7_entry, "commandline")
	new_list = append(new_list, ps7_entry)

	// 4b. Windows PowerShell -> rename to PowerShell 5
	winps_entry := prepare_entry(winps_guid)
	winps_entry["guid"] = winps_guid
	winps_entry["name"] = "PowerShell 5"
	winps_entry["hidden"] = false
	// ensure commandline exists; if absent, set default
	if _, ok := winps_entry["commandline"]; !ok {
		winps_entry["commandline"] = "%SystemRoot%\\System32\\WindowsPowerShell\\v1.0\\powershell.exe"
	}
	delete(winps_entry, "source")
	new_list = append(new_list, winps_entry)

	// 4c. Command Prompt
	cmd_entry := prepare_entry(cmd_guid)
	cmd_entry["guid"] = cmd_guid
	cmd_entry["name"] = "Command Prompt"
	cmd_entry["hidden"] = false
	if _, ok := cmd_entry["commandline"]; !ok {
		cmd_entry["commandline"] = "%SystemRoot%\\System32\\cmd.exe"
	}
	delete(cmd_entry, "source")
	new_list = append(new_list, cmd_entry)

	// 4d. Azure Cloud Shell
	azure_entry := prepare_entry(azure_guid)
	azure_entry["guid"] = azure_guid
	azure_entry["name"] = "Azure Cloud Shell"
	azure_entry["hidden"] = false
	azure_entry["source"] = "Windows.Terminal.Azure"
	delete(azure_entry, "commandline")
	new_list = append(new_list, azure_entry)

	profiles["list"] = new_list
	return nil
}