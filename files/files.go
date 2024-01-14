package files

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func Remove(rootDir string, toKeep int) ([]os.DirEntry, error) {
	var xrfiles []os.DirEntry
	if len(rootDir) != 0 {
		if files, err := os.ReadDir(rootDir); err != nil {
			return nil, fmt.Errorf("failed to read log directory [ %s ] contents. Reason: %s", rootDir, err)
		} else if len(files) > toKeep {
			sortFilesByModTime(files)
			xrfiles = files[:len(files)-toKeep]
			if err = remove(rootDir, xrfiles); err != nil {
				return nil, err
			}
		}
	}
	return xrfiles, nil
}

func DirExists(dirName string) bool {
	if _, err := os.ReadDir(dirName); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func sortFilesByModTime(files []os.DirEntry) {
	sort.Slice(files, func(i, j int) bool {
		if files[i] == nil || files[j] == nil {
			return false
		}

		infoi, err := files[i].Info()
		infoj, err := files[j].Info()

		if err == nil {
			return false
		}

		compare := infoi.ModTime().Compare(infoj.ModTime())

		if compare > 0 {
			return true
		}

		return false
	})
}

func remove(rootDir string, filesToRemove []os.DirEntry) error {
	for _, file := range filesToRemove {
		fullPath := filepath.Join(rootDir, file.Name())

		if file.IsDir() {
			// Recursively delete the directory and its contents.
			if err := os.RemoveAll(fullPath); err != nil {
				return fmt.Errorf("failed to delete the directory [ %s ]. Reason: %s", fullPath, err)
			}
		} else {
			if err := os.Remove(fullPath); err != nil {
				return fmt.Errorf("failed to delete the file [ %s ]. Reason: %s", fullPath, err)
			}
		}
	}
	return nil
}
