package main

/*
This application will help remove a corrupted MSI installation of Zscaler from a Windows machine.
This application is based off of the support details of "How to manually uninstall the Zscaler Client Connector"
It requires that the agent have Tamper Protection disabled or that the machine be booted in safe mode.
*/

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	author  = "Nicholas Albright (@nma-io)"
	version = "2024.0.1"
)

func stopProcesses(namePrefix string) error {
	handle, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	for {
		if err := windows.Process32Next(handle, &entry); err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				break
			}
			return err
		}

		processName := windows.UTF16ToString(entry.ExeFile[:])
		if strings.HasPrefix(processName, namePrefix) {
			procHandle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, entry.ProcessID)
			if err != nil {
				return err
			}
			defer windows.CloseHandle(procHandle)
			if err := windows.TerminateProcess(procHandle, 0); err != nil {
				return err
			}
		}
	}
	return nil
}

func deleteService(name string) error {
	managerHandle, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_ALL_ACCESS)
	if err != nil {
		return err
	}
	defer windows.CloseServiceHandle(managerHandle)

	serviceHandle, err := windows.OpenService(managerHandle, syscall.StringToUTF16Ptr(name), windows.SERVICE_ALL_ACCESS)
	if err != nil {
		return err
	}
	defer windows.CloseServiceHandle(serviceHandle)

	if err := windows.DeleteService(serviceHandle); err != nil {
		return err
	}
	return nil
}

func removeRegistryKey(baseKey registry.Key, path string) error {
	key, err := registry.OpenKey(baseKey, path, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	subKeys, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return err
	}

	for _, subKey := range subKeys {
		if err := removeRegistryKey(baseKey, filepath.Join(path, subKey)); err != nil {
			return err
		}
	}

	return registry.DeleteKey(baseKey, path)
}

func removeDirectory(path string) error {
	return os.RemoveAll(path)
}

func main() {
	// Stop processes matching "ZSA*"
	if err := stopProcesses("ZSA"); err != nil {
		log.Fatalf("Failed to stop processes: %v", err)
	}

	// Delete Windows services
	services := []string{"ZSAService", "ZSATrayManager", "ZSATunnel", "ZSAUpdater", "ZSAUpm"}
	for _, service := range services {
		if err := deleteService(service); err != nil {
			log.Printf("Failed to delete service %s: %v", service, err)
		}
	}

	// Remove registry keys
	registryKeys := []string{
		`SOFTWARE\Classes\Installer\Products\F3BAA9CF5789C0A4BBFBC36E47F0DCE4`,
		`SOFTWARE\Classes\zsa`,
		`SOFTWARE\Zscaler Inc.`,
	}
	for _, key := range registryKeys {
		if err := removeRegistryKey(registry.LOCAL_MACHINE, key); err != nil {
			log.Printf("Failed to remove registry key %s: %v", key, err)
		}
	}

	// Remove directories
	directories := []string{
		`C:\Program Files\Zscaler`,
		`C:\ProgramData\Zscaler`,
	}
	for _, dir := range directories {
		if err := removeDirectory(dir); err != nil {
			log.Printf("Failed to remove directory %s: %v", dir, err)
		}
	}

	log.Println("Completed all tasks successfully.")
}
